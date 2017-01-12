package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

var (
	numRuns  = flag.Int("n", 2, "number of calls to make")
	latency  = flag.Duration("latency", 4*time.Millisecond, "artificial latency to introduce (symmetric)")
	msgSize  = flag.Int("size", 1<<20, "message size")
	addr     = flag.String("addr", "localhost:8080", "listen address")
	useGRPC  = flag.Bool("grpc", true, "use GRPC (fall back is plain HTTP)")
	useHTTP2 = flag.Bool("http2", true, "use HTTP2")
	verbose  = flag.Bool("v", false, "verbose output")
)

const (
	certFile = "cert.pem"
	keyFile  = "key.pem"
)

func main() {
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
			if *useHTTP2 {
				if err := http2.ConfigureTransport(t); err != nil {
					log.Fatal(err)
				}
			}
		}

		ctx := context.Background()

		for i := 0; i < *numRuns; i++ {
			randomBytes := make([]byte, *msgSize)
			n, err := rand.Read(randomBytes)
			if err != nil {
				log.Fatal(err)
			}
			if n != *msgSize {
				log.Fatal("didn't read enough bytes")
			}
			msg := string(randomBytes)

			t1 := time.Now()
			var proto string
			if *useGRPC {
				_, err = client.SayHello(ctx, &helloworld.HelloRequest{Name: msg})
				proto = "GRPC"
			} else {
				var resp *http.Response
				resp, err = http.Post("https://"+*addr, "text/plain", bytes.NewReader(randomBytes))
				proto = "HTTP"
				if resp != nil {
					proto = resp.Proto
					resp.Body.Close()
				}
			}
			if *verbose {
				fmt.Println()
			}
			fmt.Printf("%v\t%v\t%v\n", time.Now().Sub(t1), *latency, proto)
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
		if *useHTTP2 {
			config.NextProtos = []string{"h2"}
		}
		config.Certificates = make([]tls.Certificate, 1)
		config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatal(err)
		}
		srv := &http.Server{Addr: *addr, TLSConfig: &config, Handler: http.HandlerFunc(validate)}
		tlsListener := tls.NewListener(l, &config)
		log.Fatal(srv.Serve(tlsListener))
	}
}

func validate(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		log.Fatalf("validate: %v", err)
	}
	if len(b) != *msgSize {
		log.Fatalf("validate: got %d bytes, want %d", len(b), *msgSize)
	}
}

type greeter struct {
}

func (s greeter) SayHello(ctx context.Context, req *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	if len(req.Name) != *msgSize {
		log.Fatalf("greeter: got %d bytes, want %d", len(req.Name), *msgSize)
	}
	return &helloworld.HelloReply{}, nil
}
