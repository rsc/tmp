//go:build ignore

package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("."))

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%q %s %q\n", r.Header["Accept-Encoding"], r.URL.Path, r.URL.Query())
		fs.ServeHTTP(w, r)
	})
	log.Fatal(http.ListenAndServe("localhost:8080", h))
}
