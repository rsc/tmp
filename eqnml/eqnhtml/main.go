package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"rsc.io/tmp/eqnml"
)

var exitStatus = 0

var wflag = flag.Bool("w", false, "write output back to files")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: eqnmark [-w] [file...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("eqnmark: ")

	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		if *wflag {
			log.Fatal("cannot use -w with standard input")
		}
		read("<stdin>", os.Stdin)
	} else {
		for _, arg := range args {
			f, err := os.Open(arg)
			if err != nil {
				log.Print(err)
				exitStatus = 1
				continue
			}
			read(arg, f)
			f.Close()
		}
	}
	os.Exit(exitStatus)
}

func read(name string, r io.Reader) {
	bdata, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatalf("reading %s: %v", name, err)
	}
	var out bytes.Buffer
	data := string(bdata)
	all := data
	for data != "" {
		i := strings.Index(data, "<eqn>")
		if i < 0 {
			out.WriteString(data)
			break
		}
		out.WriteString(data[:i])
		data = data[i:]
		prefix := all[:len(all)-len(data)]
		block := strings.HasSuffix(prefix, "\n\n") || prefix == "\n" || prefix == ""
		j := strings.Index(data, "</eqn>")
		if j < 0 {
			out.WriteString(data)
			break
		}
		eqntext := data[len("<eqn>"):j]
		rest := data[j+len("</eqn>"):]
		block = block && (len(rest) == 0 || rest[0] == '\n')
		if block {
			rest = strings.TrimPrefix(rest, "\n")
		}
		if strings.HasPrefix(rest, "<math") {
			if i := strings.Index(rest, "</math>"); i >= 0 {
				rest = rest[i+len("</math>"):]
				if block {
					rest = strings.TrimPrefix(rest, "\n")
				}
			}
		}

		math, err := eqnml.ToHTML(eqntext)
		if err != nil {
			math = "<mtext class='err'>" + html.EscapeString(err.Error()) + "</mtext>"
		}
		out.WriteString("<eqn>")
		out.WriteString(eqntext)
		out.WriteString("</eqn>")
		if block {
			out.WriteString("\n")
			out.WriteString("<math display='block'>")
		} else {
			out.WriteString("<math>")
		}
		out.WriteString(math)
		out.WriteString("</math>")
		if block {
			out.WriteString("\n")
		}

		data = rest
	}
	if *wflag {
		if err := os.WriteFile(name, out.Bytes(), 0666); err != nil {
			log.Print(err)
			exitStatus = 1
		}
	} else {
		os.Stdout.Write(out.Bytes())
	}
}
