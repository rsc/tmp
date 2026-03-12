package main

import (
	"io"
	"os"
	"strconv"
)

func main() {
	n := 0
	if len(os.Args) > 1 {
		n, _ = strconv.Atoi(os.Args[1])
	}
	data, _ := io.ReadAll(os.Stdin)
	runes := []rune(string(data))
	for i, r := range runes {
		if r != '\t' && r != '\n' {
			runes[i] = r&0xffff | rune(n)<<16
		}
	}
	os.Stdout.Write([]byte(string(runes)))
}
