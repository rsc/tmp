// Ecosum prepares a summary of Go ecosystem pipeline results.
//
// Usage:
//
//	ecosum [-g regexp] [-n max] [-s seed] [-q] report.json
//
// The Go ecosystem pipeline runs analysis programs, such as new vet analyzers,
// on the latest versions of public Go packages. (For security reasons, it is currently
// only available inside Google.) Ecosum takes the raw results from an analysis and
// prepares a nicely formatted version that can be published to inform design discussions.
//
// Ecosum prints a report with statistics and then a random sample of 100 diagnostics.
// The number of diagnostics can be changed with the -n flag. A negative maximum sets no limit.
//
// By default ecosum considers all diagnostic errors in the report. The -g (grep) flag
// only considers diagnostics with messages matching regexp.
//
// The output is formatted as Markdown that can be pasted into a GitHub issue
// but is also mostly human-readable for direct use.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ecosum [-g regexp] [-n max] [-s seed] [-q] report.json\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	grep    = flag.String("g", "", "only consider diagnostics matching `regexp`")
	seed    = flag.Int64("s", 0, "seed random number generator with `seed`")
	samples = flag.Int("n", 100, "print at most `max` sample diagnostics (-1 for unlimited)")
	quiet   = flag.Bool("q", false, "quiet mode: do not print source listings")
)

var posRE = regexp.MustCompile(`^/tmp/modules/([^:]*):([0-9]+)(:[0-9]+)?$`)

func main() {
	log.SetFlags(0)
	log.SetPrefix("ecosum: ")
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		usage()
	}

	var re *regexp.Regexp
	if *grep != "" {
		var err error
		re, err = regexp.Compile(*grep)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *seed != 0 {
		rand.Seed(*seed)
	}

	f, err := os.Open(args[0])
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(f)
	var sum Summary
	sum.Grep = *grep
	byMod := make(map[string][]*Diagnostic)
	var mods []string
	for {
		var r Report
		err := dec.Decode(&r)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("reading %s: %v", args[0], err)
		}
		if r.Error != "" {
			continue
		}
		sum.Modules++
		reported := false
		for _, d := range r.Diagnostic {
			if d.Error != "" {
				continue
			}
			if re == nil || re.MatchString(d.Message) {
				if !reported {
					sum.BadModules++
					reported = true
				}
				m := posRE.FindStringSubmatch(d.Position)
				if m == nil {
					log.Fatalf("missing pos: %+v", d)
				}
				d.URL = "https://go-mod-viewer.appspot.com/" + m[1] + "#L" + m[2]
				d.Position = m[1] + ":" + m[2] + m[3]
				d.File = m[1]
				d.Line, _ = strconv.Atoi(m[2])
				if !*quiet && d.Source != "" {
					d.SourceQuote = "``````\n" + trim(d.Source) + "\n``````\n"
				}
				if byMod[r.ModulePath] == nil {
					mods = append(mods, r.ModulePath)
				}
				byMod[r.ModulePath] = append(byMod[r.ModulePath], d)
				sum.TotalSamples++
			}
		}
	}
	if *samples < 0 {
		*samples = sum.TotalSamples
	}
	for ; *samples > 0 && len(mods) > 0; *samples-- {
		i := rand.Intn(len(mods))
		m := mods[i]
		diags := byMod[m]
		j := rand.Intn(len(diags))
		sum.Samples = append(sum.Samples, diags[j])
		diags[j] = diags[len(diags)-1]
		diags = diags[:len(diags)-1]
		byMod[m] = diags
		if len(diags) == 0 {
			mods[i] = mods[len(mods)-1]
			mods = mods[:len(mods)-1]
		}
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, &sum)
	if err != nil {
		log.Fatalf("internal template error: %v", err)
	}
	os.Stdout.Write(buf.Bytes())
}

// A Report is the report for a single module.
type Report struct {
	CreatedAt     string        `json:"created_at"`
	ModulePath    string        `json:"module_path"`
	Version       string        `json:"version"`
	SortVersion   string        `json:"sort_version"`
	CommitTime    string        `json:"commit_time"`
	BinaryName    string        `json:"binary_name"`
	Error         string        `json:"error"`
	ErrorCategory string        `json:"error_category"`
	BinaryVersion string        `json:"binary_version"`
	BinaryArgs    string        `json:"binary_args"`
	WorkerVersion string        `json:"worker_version"`
	SchemaVersion string        `json:"schema_version"`
	Diagnostic    []*Diagnostic `json:"diagnostic"`
}

type Diagnostic struct {
	URL          string `json:"-"`
	SourceQuote  string `json:"-"`
	PackageID    string `json:"package_id"`
	AnalyzerName string `json:"analyzer_name"`
	Error        string `json:"error"`
	Category     string `json:"category"`
	Position     string `json:"position"`
	Message      string `json:"message"`
	Source       string `json:"source"`
	File         string `json:"-"`
	Line         int    `json:"-"`
}

type Summary struct {
	Grep         string
	Modules      int
	BadModules   int
	TotalSamples int
	Samples      []*Diagnostic
}

var tmpl = template.Must(template.New("").Funcs(
	template.FuncMap{
		"inc":  func(x int) int { return x + 1 },
		"code": func(s string) string { return "```" + s + "```" },
	},
).Parse(`
{{.Modules}} modules analyzed.
{{.TotalSamples}} diagnostics generated{{if .Grep}} matching {{code .Grep}}{{end}} in {{.BadModules}} modules.
{{if .Samples}}
{{- if eq (len .Samples) .TotalSamples}}<details><summary>All diagnostics.</summary>
{{- else}}<details><summary>{{len .Samples}} randomly sampled diagnostics.</summary>
{{- end}}

{{range $i, $d := .Samples}}({{inc $i}}) [{{$d.Position}}]({{$d.URL}}):
{{$d.Message}}
{{$d.SourceQuote}}
{{end}}

</details>

{{end}}
`))

func trim(s string) string {
	lines := strings.SplitAfter(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}
	prefix := lines[0]
	i := 0
	for i < len(prefix) && (prefix[i] == ' ' || prefix[i] == '\t') {
		i++
	}
	prefix = prefix[:i]
	for _, line := range lines {
		for !strings.HasPrefix(line, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, prefix)
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 {
		lines[len(lines)-1] = strings.TrimSuffix(lines[len(lines)-1], "\n")
	}
	return strings.Join(lines, "")
}
