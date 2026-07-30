// Tschat is a chat program for testing Tailscale connectivity between machines.
//
// Usage:
//
//	tschat [-v] name
//
// Tschat joins the tailnet as an embedded Tailscale node named name, connects to
// every other tailnet node running tschat, and broadcasts each line typed on
// standard input to all of them. Lines received from other nodes are printed
// with a time stamp and the name of the sending node.
//
// Tschat runs entirely in user space: it does not use tailscaled, a TUN device,
// or any system configuration, so it needs no special privileges and can run
// alongside an ordinary Tailscale installation.
//
// Two special input lines are recognized:
//
//	/who	list connected peers, and whether each connection is direct or relayed
//	/quit	exit
//
// If the environment variable TS_AUTHKEY is set, tschat uses it to authenticate
// the node. Otherwise, on the first run for a given name, tschat prints a login
// URL to visit. Node identity is saved in a per-name subdirectory of the user
// config directory, so later runs reuse the same node.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
)

// port is the TCP port tschat listens on within the tailnet.
const port = 4747

var verbose = flag.Bool("v", false, "print verbose logging, including the Tailscale node logs")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: tschat [-v] name\n")
	os.Exit(2)
}

// A message is one chat line, sent as a JSON object on its own line.
type message struct {
	From string    `json:"from"`
	Time time.Time `json:"time"`
	Text string    `json:"text"`
}

// A hello is the first JSON object sent by the node that dialed the connection,
// telling the accepting node who is calling.
type hello struct {
	Name string `json:"name"`
}

// A peer is a live connection to another tschat node.
type peer struct {
	name string
	conn net.Conn

	mu  sync.Mutex // guards writes to enc
	enc *json.Encoder
}

// send writes m to the peer.
func (p *peer) send(m *message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.enc.Encode(m)
}

// A chat is the state of the running program.
type chat struct {
	self string
	srv  *tsnet.Server

	mu      sync.Mutex
	peers   map[string]*peer      // connected peers, by name
	addrs   map[string]netip.Addr // last known tailnet address, by name
	dialing map[string]bool       // peers with a running dial loop

	outMu sync.Mutex // serializes writes to standard output
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("tschat: ")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
	}
	name := flag.Arg(0)

	cfg, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Join(cfg, "tschat", name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Fatal(err)
	}

	srv := &tsnet.Server{
		Hostname: name,
		Dir:      dir,
		AuthKey:  os.Getenv("TS_AUTHKEY"),
		UserLogf: onceLogf(log.Printf),    // tsnet repeats the login URL every few seconds
		Logf:     func(string, ...any) {}, // the node logs are very chatty; see -v
	}
	if *verbose {
		srv.Logf = log.Printf
		srv.UserLogf = log.Printf
	}

	// Close the node on interrupt so it stops cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		srv.Close()
		os.Exit(0)
	}()
	defer srv.Close()

	st, err := srv.Up(context.Background())
	if err != nil {
		log.Fatalf("joining tailnet: %v", err)
	}
	if st.Self == nil {
		log.Fatal("joining tailnet: no self status")
	}

	c := &chat{
		self:    peerName(st.Self),
		srv:     srv,
		peers:   make(map[string]*peer),
		addrs:   make(map[string]netip.Addr),
		dialing: make(map[string]bool),
	}
	c.printf("* connected to tailnet as %s %v", c.self, st.Self.TailscaleIPs)

	ln, err := srv.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	go c.listen(ln)
	go c.discover()

	c.input()
}

// onceLogf returns a log function that prints each distinct message only once.
func onceLogf(logf func(string, ...any)) func(string, ...any) {
	var mu sync.Mutex
	seen := make(map[string]bool)
	return func(format string, args ...any) {
		s := fmt.Sprintf(format, args...)
		mu.Lock()
		dup := seen[s]
		seen[s] = true
		mu.Unlock()
		if !dup {
			logf("%s", s)
		}
	}
}

// peerName returns the short tailnet name of the node described by st.
func peerName(st *ipnstate.PeerStatus) string {
	if name, _, ok := strings.Cut(st.DNSName, "."); ok && name != "" {
		return name
	}
	return st.HostName
}

// printf prints a line to standard output.
func (c *chat) printf(format string, args ...any) {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	fmt.Printf(format+"\n", args...)
}

// discover polls the tailnet for peers, starting a dial loop for each new one.
//
// To avoid two nodes dialing each other simultaneously and ending up with a
// redundant pair of connections, only the node with the lexically smaller name
// dials; the other waits to accept. Each pair of nodes therefore shares exactly
// one connection, used in both directions.
func (c *chat) discover() {
	for {
		lc, err := c.srv.LocalClient()
		if err == nil {
			var st *ipnstate.Status
			if st, err = lc.Status(context.Background()); err == nil {
				for _, ps := range st.Peer {
					name := peerName(ps)
					if name == "" || len(ps.TailscaleIPs) == 0 {
						continue
					}
					c.mu.Lock()
					c.addrs[name] = ps.TailscaleIPs[0]
					start := c.self < name && !c.dialing[name]
					if start {
						c.dialing[name] = true
					}
					c.mu.Unlock()
					if start {
						go c.dial(name)
					}
				}
			}
		}
		if err != nil && *verbose {
			log.Printf("status: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
}

// dial maintains a connection to the named peer, reconnecting as needed.
// Most tailnet nodes are not running tschat, so failures are expected and
// are only reported under -v.
func (c *chat) dial(name string) {
	backoff := 1 * time.Second
	for {
		c.mu.Lock()
		addr, ok := c.addrs[name]
		c.mu.Unlock()
		if ok {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			conn, err := c.srv.Dial(ctx, "tcp", netip.AddrPortFrom(addr, port).String())
			cancel()
			if err == nil {
				backoff = 1 * time.Second
				enc := json.NewEncoder(conn)
				if err := enc.Encode(&hello{Name: c.self}); err != nil {
					conn.Close()
				} else {
					c.serve(name, conn, enc, json.NewDecoder(conn))
				}
			} else if *verbose {
				log.Printf("dial %s: %v", name, err)
			}
		}
		time.Sleep(backoff)
		if backoff *= 2; backoff > 15*time.Second {
			backoff = 15 * time.Second
		}
	}
}

// listen accepts connections from peers with lexically smaller names.
func (c *chat) listen(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("accept: %v", err)
		}
		go func() {
			dec := json.NewDecoder(conn)
			var h hello
			if err := dec.Decode(&h); err != nil || h.Name == "" {
				conn.Close()
				return
			}
			c.serve(h.Name, conn, json.NewEncoder(conn), dec)
		}()
	}
}

// serve runs the receive loop for a connection to the named peer,
// returning when the connection ends.
func (c *chat) serve(name string, conn net.Conn, enc *json.Encoder, dec *json.Decoder) {
	defer conn.Close()
	p := &peer{name: name, conn: conn, enc: enc}

	c.mu.Lock()
	_, dup := c.peers[name]
	if !dup {
		c.peers[name] = p
	}
	c.mu.Unlock()
	if dup {
		return
	}
	defer func() {
		c.mu.Lock()
		if c.peers[name] == p {
			delete(c.peers, name)
		}
		c.mu.Unlock()
		c.printf("* %s disconnected", name)
	}()

	c.printf("* %s connected", name)
	for {
		var m message
		if err := dec.Decode(&m); err != nil {
			if !errors.Is(err, io.EOF) && *verbose {
				log.Printf("%s: %v", name, err)
			}
			return
		}
		c.printf("%s <%s> %s", m.Time.Local().Format("15:04:05"), m.From, m.Text)
	}
}

// input reads lines from standard input and broadcasts them to all peers.
func (c *chat) input() {
	br := bufio.NewReader(os.Stdin)
	for {
		line, err := br.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if err != nil && line == "" {
			return
		}
		switch strings.TrimSpace(line) {
		case "":
			continue
		case "/quit":
			return
		case "/who":
			c.who()
			continue
		}
		m := &message{From: c.self, Time: time.Now(), Text: line}
		c.mu.Lock()
		peers := make([]*peer, 0, len(c.peers))
		for _, p := range c.peers {
			peers = append(peers, p)
		}
		c.mu.Unlock()
		if len(peers) == 0 {
			c.printf("* no peers connected")
			continue
		}
		for _, p := range peers {
			if err := p.send(m); err != nil {
				if *verbose {
					log.Printf("send %s: %v", p.name, err)
				}
				p.conn.Close() // makes serve clean up
			}
		}
	}
}

// who prints the connected peers and how each connection is carried:
// directly to the peer, or relayed through a DERP server.
func (c *chat) who() {
	c.mu.Lock()
	names := make([]string, 0, len(c.peers))
	for name := range c.peers {
		names = append(names, name)
	}
	c.mu.Unlock()
	slices.Sort(names)
	if len(names) == 0 {
		c.printf("* no peers connected")
		return
	}

	// Look up the current route to each peer. This is best effort:
	// if the status is unavailable, still list the peers.
	route := make(map[string]string)
	if lc, err := c.srv.LocalClient(); err == nil {
		if st, err := lc.Status(context.Background()); err == nil {
			for _, ps := range st.Peer {
				switch {
				case ps.CurAddr != "":
					route[peerName(ps)] = "direct " + ps.CurAddr
				case ps.Relay != "":
					route[peerName(ps)] = "relay " + ps.Relay
				}
			}
		}
	}
	for _, name := range names {
		how := route[name]
		if how == "" {
			how = "route unknown"
		}
		c.printf("* %s (%s)", name, how)
	}
}
