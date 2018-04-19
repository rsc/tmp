// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ssh-namespace-agent tunnels the 9P name space over ssh-agent protocol.
//
// To use, add to your profile on both the local and remote systems:
//
//	eval $(ssh-namespace-agent)
//
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	plan9client "9fans.net/go/plan9/client"
)

var verbose = flag.Bool("v", false, "enable verbose debugging")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: eval $(ssh-namespace-agent)\n")
	os.Exit(2)
}

func main() {
	log.SetPrefix("ssh-namespace-agent: ")
	log.SetFlags(0)
	if len(os.Args) == 2 && os.Args[1] == "--daemon--" {
		daemon()
		return
	}

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 0 {
		usage()
	}

	r1, w1, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	r2, w2, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "--daemon--")
	cmd.Stdout = w1
	cmd.Stderr = w2
	err = cmd.Start()
	if err != nil {
		log.Fatalf("reexec: %v", err)
	}
	w1.Close()
	w2.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	done := make(chan bool, 2)
	go func() {
		io.Copy(&stdout, r1)
		done <- true
	}()
	go func() {
		io.Copy(&stderr, r2)
		done <- true
	}()
	<-done
	<-done

	out := stdout.Bytes()
	ok := false
	if bytes.HasSuffix(out, []byte("\nOK\n")) || bytes.Equal(out, []byte("OK\n")) {
		out = out[:len(out)-len("OK\n")]
		ok = true
	}
	if len(out)+stderr.Len() == 0 {
		log.Print("no output")
	}
	os.Stdout.Write(out)
	os.Stderr.Write(stderr.Bytes())
	if !ok {
		os.Exit(1)
	}
}

func readMsg(c net.Conn) ([]byte, error) {
	buf := make([]byte, 4)
	n, err := io.ReadFull(c, buf)
	if err != nil {
		return buf[:n], err
	}
	nn := int(binary.BigEndian.Uint32(buf))
	bbuf := make([]byte, nn)
	copy(bbuf, buf)
	_, err = io.ReadFull(c, bbuf)
	if err != nil {
		return nil, err
	}
	return bbuf, nil
}

func writeMsg(c net.Conn, body []byte) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(body)))
	_, err := c.Write(buf)
	if err != nil {
		return err
	}
	_, err = c.Write(body)
	if err != nil {
		return err
	}
	return nil
}

const (
	SSH_AGENT_FAILURE           = 5
	SSH_AGENT_SUCCESS           = 6
	SSH_AGENTC_EXTENSION        = 27
	SSH_AGENT_EXTENSION_FAILURE = 28
	extName                     = "sshns@9fans.net"
)

var (
	extHeader = []byte("\x1b\x0fsshns@9fans.net")
)

func runExt(c net.Conn, req []byte) ([]byte, error, bool) {
	msg := make([]byte, 4+len(extHeader))
	binary.BigEndian.PutUint32(msg, uint32(len(extHeader)+len(req)))
	copy(msg[4:], extHeader)
	if _, err := c.Write(msg); err != nil {
		return nil, err, false
	}
	if _, err := c.Write(req); err != nil {
		return nil, err, false
	}
	m, err := readMsg(c)
	if err != nil {
		return nil, err, true
	}
	if !bytes.HasPrefix(m, extHeader) {
		return nil, fmt.Errorf("unexpected response"), true
	}
	m = m[len(extHeader):]
	if bytes.HasPrefix(m, []byte("ok\n")) {
		return m[3:], nil, true
	}
	if bytes.HasPrefix(m, []byte("err\n")) {
		return nil, errors.New(string(m[4:])), true
	}
	return nil, fmt.Errorf("unexpected response"), true
}

func writeExtReply(c net.Conn, data []byte) error {
	return writeMsg(c, append(extHeader, data...))
}

func parseExtmsg(m []byte) (string, []byte) {
	line := m
	if i := bytes.IndexByte(line, '\n'); i >= 0 {
		line, m = line[:i], m[i+1:]
	} else {
		line, m = m, nil
	}
	cmd := string(line)
	return cmd, m
}

func daemon() {
	if os.Getenv("SSH_CONNECTION") != "" {
		server()
		return
	}
	client()
}

// runs on ssh server side
func server() {
	// Maybe these should be quiet failures?
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		log.Fatal("$SSH_AUTH_SOCK not set")
	}

	_, err := listRemote(sock)
	if err != nil {
		log.Fatal(err)
	}

	dir := filepath.Dir(sock)
	plan9 := filepath.Join(dir, "plan9")
	_, err = os.Stat(plan9)
	if err == nil {
		// Daemon already running.
		fmt.Printf("export NAMESPACE=%s\n", plan9)
		fmt.Printf("OK\n")
		return
	}
	err = os.Mkdir(plan9, 0700)
	if err != nil {
		log.Fatal(err)
	}

	if err := createSockets(sock, plan9); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("export NAMESPACE=%s\n", plan9)
	fmt.Printf("OK\n")
	closeStdout()

	for {
		time.Sleep(1 * time.Minute)
		createSockets(sock, plan9)
	}
}

var connCache struct {
	sync.Mutex
	c []net.Conn
}

// TODO: Cache connections.
func dialAndRunExt(sock string, msg []byte) ([]byte, error) {
	connCache.Lock()
	var c net.Conn
	if len(connCache.c) > 0 {
		c = connCache.c[len(connCache.c)-1]
		connCache.c = connCache.c[:len(connCache.c)-1]
	}
	connCache.Unlock()
	if c == nil {
		var err error
		log.Printf("redial %s", sock)
		c, err = net.Dial("unix", sock)
		if err != nil {
			return nil, err
		}
	}
	m, err, ok := runExt(c, msg)
	if !ok {
		c.Close()
	} else {
		connCache.Lock()
		connCache.c = append(connCache.c, c)
		connCache.Unlock()
	}
	return m, err
}

func listRemote(sock string) ([]string, error) {
	data, err := dialAndRunExt(sock, []byte("list"))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	return strings.Split(string(data), "\x00"), nil
}

func closeStdout() {
	fd, err := syscall.Open("/dev/null", syscall.O_RDWR, 0)
	if err != nil {
		log.Fatal(err)
	}
	syscall.Dup2(fd, 0)
	if fd > 2 {
		syscall.Close(fd)
	}
	fd, err = syscall.Open(os.Getenv("HOME")+"/.sshns.log", syscall.O_WRONLY|syscall.O_APPEND|syscall.O_CREAT, 0600)
	if err != nil {
		log.Fatal(err)
	}
	syscall.Dup2(fd, 1)
	syscall.Dup2(fd, 2)
	if fd > 2 {
		syscall.Close(fd)
	}
	log.SetFlags(log.LstdFlags)
}

func reverseDial(sock, name string) (rc *remoteConn, err error) {
	id, err := dialAndRunExt(sock, []byte("dial "+name))
	if err != nil {
		log.Printf("dial %s: %v", name, err)
		return nil, err
	}
	log.Printf("dial %s -> %s\n", name, id)
	r := &remoteConn{sock: sock, id: string(id)}
	go r.lease()
	return r, nil
}

type remoteConn struct {
	id   string
	sock string
	dead uint32
}

const expireDelta = 10 * time.Minute

func (r *remoteConn) lease() {
	for atomic.LoadUint32(&r.dead) == 0 {
		dialAndRunExt(r.sock, []byte("refresh "+r.id))
		time.Sleep(expireDelta / 2)
	}
}

func (r *remoteConn) Read(data []byte) (int, error) {
	log.Printf("read %s %d\n", r.id, len(data))
	d, err := dialAndRunExt(r.sock, []byte(fmt.Sprintf("read %d %s", len(data), r.id)))
	if err != nil {
		log.Printf("read %s %d: %v", r.id, len(data), err)
		return 0, err
	}
	log.Printf("read %s %d: %d", r.id, len(data), len(d))
	return copy(data, d), nil
}

func (r *remoteConn) Write(data []byte) (int, error) {
	log.Printf("write %s %d\n", r.id, len(data))
	var w int
	for len(data) > 0 {
		n := len(data)
		if n > 10000 {
			n = 10000
		}
		log.Printf("write1 %s %d\n", r.id, n)
		_, err := dialAndRunExt(r.sock, append([]byte("write "+r.id+"\n"), data[:n]...))
		if err != nil {
			return w, err
		}
		w += n
		data = data[n:]
	}
	return w, nil
}

func (r *remoteConn) Close() error {
	log.Printf("close %s\n", r.id)
	atomic.StoreUint32(&r.dead, 1)
	_, err := dialAndRunExt(r.sock, []byte("close "+r.id))
	return err
}

var created = map[string]bool{}

func createSockets(sock, plan9 string) error {
	names, err := listRemote(sock)
	if err != nil {
		log.Fatal(err) // probably client is gone
	}
	for _, name := range names {
		if !created[name] {
			created[name] = true
			go proxySocket(sock, plan9, name)
		}
	}
	return nil
}

func proxySocket(sock, plan9, name string) {
	l, err := net.Listen("unix", filepath.Join(plan9, name))
	if err != nil {
		log.Printf("post %s: %v", name, err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			time.Sleep(1 * time.Minute)
			continue
		}
		c1, err := reverseDial(sock, name)
		if err != nil {
			c.Close()
			log.Printf("reverseDial %s: %v", name, err)
			continue
		}
		go proxy(c, c1)
	}
}

func proxy(c, c1 io.ReadWriteCloser) {
	done := make(chan bool, 2)
	go func() {
		io.Copy(c, c1)
		c.Close()
		done <- true
	}()
	go func() {
		io.Copy(c1, c)
		c1.Close()
		done <- true
	}()
	<-done
	<-done
}

// runs on ssh client side
func client() {
	// Maybe these should be quiet failures?
	oldSock := os.Getenv("SSH_AUTH_SOCK")
	if oldSock == "" {
		if *verbose {
			log.Fatal("$SSH_AUTH_SOCK not set")
		}
		return
	}
	if strings.HasSuffix(oldSock, "/sshns.socket") {
		if *verbose {
			log.Fatal("$SSH_AUTH_SOCK is already an ssh-namespace-agent")
		}
		return
	}

	ns := plan9client.Namespace()
	if ns == "" {
		log.Fatal("no plan9 namespace")
	}
	if err := os.MkdirAll(ns, 0700); err != nil {
		log.Fatal(err)
	}

	// NOTE(rsc): Tried to use ssh-namespace-agent.socket,
	// but combined with my Mac's current default $(namespace)
	// of /tmp/ns.rsc._private_tmp_com.apple.launchd.7VN9hyV2B7_org.macosforge.xquartz:0/
	// that name just barely exceeds the 104-byte limit.
	// Probably the default namespace needs to be shortened,
	// but to avoid requiring that, we use a shorter name.
	newSock := filepath.Join(ns, "sshns.socket")
	l, err := net.Listen("unix", newSock)
	if err != nil {
		// Maybe already running?
		c, err := net.Dial("unix", newSock)
		if err == nil {
			c.Close()
			fmt.Printf("export SSH_AUTH_SOCK=%s\n", newSock)
			fmt.Printf("OK\n")
			return
		}
		os.Remove(newSock)
		l, err = net.Listen("unix", newSock)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("export SSH_AUTH_SOCK=%s\n", newSock)
	fmt.Printf("OK\n")
	closeStdout()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go serve(c, oldSock, ns)
	}
}

func serve(c net.Conn, oldSock, ns string) {
	log.Printf("serving on client\n")
	var c1 net.Conn
	defer c.Close()
	for {
		m, err := readMsg(c)
		if err != nil {
			log.Printf("serving socket: readMsg: %v", err)
			return
		}
		log.Printf("serve %d %d", len(m), m[0])
		if !bytes.HasPrefix(m, extHeader) {
			// pass message to underlying agent
			if c1 == nil {
				c1, err = net.Dial("unix", oldSock)
				if err != nil {
					log.Printf("proxying message: dial: %v", err)
					return
				}
				defer c1.Close()
			}
			if err := writeMsg(c1, m); err != nil {
				log.Printf("proxying message: write: %v", err)
				return
			}
			m, err = readMsg(c1)
			if err != nil {
				log.Printf("proxying message: read: %v", err)
				return
			}
			if err := writeMsg(c, m); err != nil {
				log.Printf("proxying message: write back: %v", err)
				return
			}
			continue
		}
		cmd, m := parseExtmsg(m[len(extHeader):])
		f := strings.Fields(cmd)
		if len(f) > 0 {
			switch f[0] {
			case "list":
				handleList(c, ns)
				continue
			case "dial":
				if len(f) == 2 {
					handleDial(c, ns, f[1])
					continue
				}
			case "close":
				if len(f) == 2 {
					handleClose(c, f[1])
					continue
				}
			case "write":
				if len(f) == 2 {
					handleWrite(c, f[1], m)
					continue
				}
			case "read":
				if len(f) == 3 {
					n, err := strconv.Atoi(f[1])
					if err == nil {
						handleRead(c, n, f[2])
						continue
					}
				}
			case "refresh":
				if len(f) == 2 {
					handleRefresh(c, f[1])
					continue
				}
			}
		}
		writeExtReply(c, []byte(fmt.Sprintf("err\nunknown command %q", cmd)))
	}
}

func handleList(c net.Conn, ns string) {
	names, _ := filepath.Glob(filepath.Join(ns, "*"))
	var out []string
	for _, name := range names {
		name = filepath.Base(name)
		if !strings.HasSuffix(name, ".socket") {
			out = append(out, name)
		}
	}
	reply := []byte("ok\n" + strings.Join(out, "\x00"))
	writeExtReply(c, reply)
}

type conn struct {
	c      net.Conn
	expire time.Time
}

var conns struct {
	sync.Mutex
	m map[string]*conn
	n int
}

func init() {
	go func() {
		for {
			time.Sleep(expireDelta)
			conns.Lock()
			var dead []*conn
			for k, cc := range conns.m {
				if time.Now().After(cc.expire) {
					dead = append(dead, cc)
					delete(conns.m, k)
				}
			}
			conns.Unlock()
			for _, cc := range dead {
				cc.c.Close()
			}
		}
	}()
}

func handleDial(c net.Conn, ns string, name string) {
	c1, err := net.Dial("unix", filepath.Join(ns, name))
	if err != nil {
		writeExtReply(c, []byte("err\n"+err.Error()))
		return
	}
	conns.Lock()
	conns.n++
	id := fmt.Sprint(conns.n)
	if conns.m == nil {
		conns.m = map[string]*conn{}
	}
	conns.m[id] = &conn{c: c1, expire: time.Now().Add(expireDelta)}
	conns.Unlock()
	writeExtReply(c, []byte("ok\n"+id))
}

func handleClose(c net.Conn, id string) {
	conns.Lock()
	cc := conns.m[id]
	if cc != nil {
		delete(conns.m, id)
	}
	conns.Unlock()

	if cc == nil {
		writeExtReply(c, []byte("err\nunknown conn"))
		return
	}

	cc.c.Close()
	writeExtReply(c, []byte("ok\n"))
}

func handleRead(c net.Conn, n int, id string) {
	conns.Lock()
	cc := conns.m[id]
	if cc != nil {
		cc.expire = time.Now().Add(expireDelta)
	}
	conns.Unlock()

	if cc == nil {
		writeExtReply(c, []byte("err\nunknown conn"))
		return
	}

	log.Printf("handleRead %s %d", id, n)
	buf := make([]byte, 3+n)
	n, err := cc.c.Read(buf[3:])
	if n > 0 {
		err = nil
	}
	if err != nil {
		writeExtReply(c, []byte("err\n"+err.Error()))
		return
	}
	copy(buf[0:], "ok\n")
	writeExtReply(c, buf[:3+n])
}

func handleWrite(c net.Conn, id string, data []byte) {
	conns.Lock()
	cc := conns.m[id]
	if cc != nil {
		cc.expire = time.Now().Add(expireDelta)
	}
	conns.Unlock()

	if cc == nil {
		writeExtReply(c, []byte("err\nunknown conn"))
		return
	}

	log.Printf("handleWrite %s %d", id, len(data))
	_, err := cc.c.Write(data)
	if err != nil {
		writeExtReply(c, []byte("err\n"+err.Error()))
		return
	}
	writeExtReply(c, []byte("ok\n"))
}

func handleRefresh(c net.Conn, id string) {
	conns.Lock()
	cc := conns.m[id]
	if cc != nil {
		cc.expire = time.Now().Add(expireDelta)
	}
	conns.Unlock()
	if cc == nil {
		writeExtReply(c, []byte("err\nunknown conn"))
		return
	}
	writeExtReply(c, []byte("ok\n"))
}
