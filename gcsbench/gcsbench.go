// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
)

var (
	bucketName = flag.String("b", "", "bucket name")
	size       = flag.Int("size", 1<<20, "size of writes in bytes")
	parallel   = flag.Int("p", 1, "parallelism")
	count      = flag.Int("n", 20, "number of writes")
	newClient  = flag.Bool("newclient", false, "use new client for each write")
	writeChunk = flag.Int("writechunk", 1<<20, "max size of write")
)

func main() {
	log.SetPrefix("gcsbench: ")
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gcsbench -b bucketName\n")
		os.Exit(2)
	}
	flag.Parse()
	if *bucketName == "" || len(flag.Args()) > 0 {
		flag.Usage()
	}

	http.DefaultTransport = newLogger(http.DefaultTransport)

	start := time.Now()
	var (
		mu    sync.Mutex
		total time.Duration
	)
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("storage.NewClient: %v", err)
	}
	sema := make(chan bool, *parallel)
	for i := 0; i < *count; i++ {
		sema <- true
		name := fmt.Sprintf("gcsbench/tmp.%d", i)
		go func() {
			start := time.Now()
			client := client
			if *newClient {
				var err error
				client, err = storage.NewClient(ctx)
				if err != nil {
					log.Fatalf("storage.NewClient: %v", err)
				}
			}
			obj := client.Bucket(*bucketName).Object(name)
			w := obj.NewWriter(ctx)
			buf := make([]byte, *size)
			for len(buf) > 0 {
				n := len(buf)
				if n > *writeChunk {
					n = *writeChunk
				}
				w.Write(buf[:n])
				buf = buf[n:]
			}
			if err := w.Close(); err != nil {
				log.Fatalf("writing file: %v", err)
			}
			mu.Lock()
			total += time.Since(start)
			mu.Unlock()
			<-sema
		}()
	}
	for i := 0; i < *parallel; i++ {
		sema <- true
	}

	fmt.Printf("avg %.3fs per write\n", (total / time.Duration(*count)).Seconds())
	elapsed := time.Since(start)
	fmt.Printf("total %.3fs %.3f MB/s\n", elapsed.Seconds(), float64(*count)*float64(*size)/1e6/elapsed.Seconds())
}

func newLogger(t http.RoundTripper) http.RoundTripper {
	return &loggingTransport{transport: t}
}

type loggingTransport struct {
	transport http.RoundTripper
	mu        sync.Mutex
	active    []byte
}

func (t *loggingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.mu.Lock()
	index := len(t.active)
	start := time.Now()
	fmt.Printf("HTTP: %s %s+ %s\n", timeFormat(start), t.active, r.URL)
	t.active = append(t.active, '|')
	t.mu.Unlock()

	resp, err := t.transport.RoundTrip(r)

	last := r.URL.Path
	if i := strings.LastIndex(last, "/"); i >= 0 {
		last = last[i:]
	}
	display := last
	if resp != nil {
		display += " " + resp.Status
	}
	if err != nil {
		display += " error: " + err.Error()
	}
	now := time.Now()

	t.mu.Lock()
	t.active[index] = '-'
	fmt.Printf("HTTP: %s %s %s (%.3fs)\n", timeFormat(now), t.active, display, now.Sub(start).Seconds())
	t.active[index] = ' '
	n := len(t.active)
	for n%4 == 0 && n >= 4 && t.active[n-1] == ' ' && t.active[n-2] == ' ' && t.active[n-3] == ' ' && t.active[n-4] == ' ' {
		t.active = t.active[:n-4]
		n -= 4
	}
	t.mu.Unlock()

	return resp, err
}

func timeFormat(t time.Time) string {
	return t.Format("15:04:05.000")
}
