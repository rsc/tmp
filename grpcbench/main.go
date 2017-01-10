package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

func main() {
	var (
		numRuns = flag.Int("n", 2, "number of calls to make")
		latency = flag.Duration("latency", 4*time.Millisecond, "artificial latency to introduce (symmetric)")
		msgSize = flag.Int("size", 1<<20, "message size")
		addr    = flag.String("addr", "localhost:8080", "listen address")
		useGRPC = flag.Bool("grpc", true, "use GRPC (fall back is plain HTTP)")
	)
	const (
		certFile = "cert.pem"
		keyFile  = "key.pem"
	)
	flag.Parse()

	ready := make(chan struct{})
	go func() {
		<-ready

		var client helloworld.GreeterClient
		if *useGRPC {
			opts := []grpc.DialOption{
				grpc.WithBlock(),
				grpc.WithTimeout(3 * time.Second),
				grpc.WithInsecure(),
			}
			conn, err := grpc.Dial(*addr, opts...)
			if err != nil {
				log.Fatalf("grpc.Dial: %v", err)
			}
			client = helloworld.NewGreeterClient(conn)
		} else {
			t := (http.DefaultTransport.(*http.Transport))
			t.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
			if err := http2.ConfigureTransport(t); err != nil {
				log.Fatal(err)
			}
		}

		ctx := context.Background()

		msg := strings.Repeat(" ", *msgSize)
		for i := 0; i < *numRuns; i++ {
			t1 := time.Now()
			var err error
			var proto string
			if *useGRPC {
				_, err = client.SayHello(ctx, &helloworld.HelloRequest{Name: msg})
				proto = "GRPC"
			} else {
				var resp *http.Response
				resp, err = http.Post("https://"+*addr, "text/plain", strings.NewReader(msg))
				proto = "HTTP"
				if resp != nil {
					proto = resp.Proto
				}
			}
			fmt.Printf("\n%v\t%v\t%v\n", time.Now().Sub(t1), *latency, proto)
			if err != nil {
				log.Fatal(err)
			}
		}

		os.Exit(0)
	}()

	var server *grpc.Server
	if *useGRPC {
		server = grpc.NewServer()
		helloworld.RegisterGreeterServer(server, greeter{})
	}
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	rate := Rate{Latency: *latency}
	l = &Listener{l, rate, rate}
	close(ready)
	if *useGRPC {
		log.Fatal(server.Serve(l))
	} else {
		var config tls.Config
		var err error
		config.NextProtos = []string{"h2"}
		config.Certificates = make([]tls.Certificate, 1)
		config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatal(err)
		}
		srv := &http.Server{Addr: *addr, TLSConfig: &config}
		tlsListener := tls.NewListener(l, &config)
		log.Fatal(srv.Serve(tlsListener))
	}
}

type greeter struct {
}

func (s greeter) SayHello(context.Context, *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	return &helloworld.HelloReply{}, nil
}
