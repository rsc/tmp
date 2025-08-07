// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gadget is an interactive client for [Google's Gemini API].
//
// Usage:
//
//	gadget [-l] [-k keyfile] [prompt...]
//
// Gadget concatenates its arguments, sends the result as a prompt
// to the Gemini Pro model, and prints the response.
//
// With no arguments, gemini reads standard input until EOF
// and uses that as the prompt.
//
// The -l flag runs gemini in an interactive line-based mode:
// it reads a single line of input and prints the Gemini response,
// and repeats. The -l flag cannot be used with arguments.
//
// The -k flag specifies the name of a file containing the Gemini API key
// (default $HOME/.geminikey).
//
// [Google's Gemini API]: https://ai.google.dev/gemini-api/docs
package main

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"google.golang.org/genai"
	"rsc.io/tmp/gadget/internal/envfile"
	"rsc.io/tmp/gadget/internal/schema"
)

var (
	home, _ = os.UserHomeDir()
	model   = flag.String("m", "gemini-2.5-flash", "use gemini `model`")

	flagEnv = flag.String("env", filepath.Join(home, ".env"), "read env settings from `file`")

	flagCode        = flag.Bool("code", false, "enable code execution tool (on Gemini servers)")
	flagComputer    = flag.Bool("computer", false, "enable computer use tool")
	flagMaps        = flag.Bool("maps", false, "enable Google Maps tool") // not supported in Gemini API
	flagGoogle      = flag.Bool("google", false, "enable Google Search tool")
	flagGoogleRAG   = flag.Bool("googlerag", false, "enable Google Search Retrieval tool") // not supported (in Gemini API?)
	flagURLs        = flag.Bool("urls", false, "enable URL context retrieval tool")
	flagSys         = flag.String("sys", "", "use `text` as system instruction")
	flagSysFile     = flag.String("sysfile", "", "read system instruction from `file`")
	flagThink       = flag.Bool("think", false, "show thoughts")
	flagThinkBudget = flag.Int("thinkbudget", -1, "set thinking budget to `N` tokens (< 0 is unlimited)")
	flagMaxOutput   = flag.Int("maxoutput", -1, "set output limit to `N` tokens (≤ 0 is unlimited)")
	flagSeed        = flag.Int("seed", -1, "use random seed `N`")
	flagRot13       = flag.Bool("rot13", false, "enable local rot13 tool")
	flagAttach      = flag.String("a", "", "attach `file` to request")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gadget [options] [prompt]\n")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("gadget: ")
	flag.Usage = usage
	flag.Parse()

	var env map[string]string
	var err error
	if *flagEnv != "" {
		env, err = envfile.Load(*flagEnv)
		if err != nil {
			log.Fatal(err)
		}
	}
	key := cmp.Or(os.Getenv("GEMINI_API_KEY"), env["GEMINI_API_KEY"])
	if key == "" {
		log.Fatalf("missing $GEMINI_API_KEY; set in environment or $HOME/.env")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  key,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	lf := logFile()

	config := &genai.GenerateContentConfig{
		CandidateCount: 1,

		// // Optional. List of strings that tells the model to stop generating text if one
		// // of the strings is encountered in the response.
		// StopSequences []string `json:"stopSequences,omitempty"`

		// // Optional. Output response mimetype of the generated candidate text.
		// // Supported mimetype:
		// //   - `text/plain`: (default) Text output.
		// //   - `application/json`: JSON response in the candidates.
		// // The model needs to be prompted to output the appropriate response type,
		// // otherwise the behavior is undefined.
		// // This is a preview feature.
		// ResponseMIMEType string `json:"responseMimeType,omitempty"`

		// // Optional. The `Schema` object allows the definition of input and output data types.
		// // These types can be objects, but also primitives and arrays.
		// // Represents a select subset of an [OpenAPI 3.0 schema
		// // object](https://spec.openapis.org/oas/v3.0.3#schema).
		// // If set, a compatible response_mime_type must also be set.
		// // Compatible mimetypes: `application/json`: Schema for JSON response.
		// ResponseSchema *Schema `json:"responseSchema,omitempty"`

		// // Optional. Safety settings in the request to block unsafe content in the
		// // response.
		// SafetySettings []*SafetySetting `json:"safetySettings,omitempty"`

		// // Optional. Code that enables the system to interact with external systems to
		// // perform an action outside of the knowledge and scope of the model.
		// Tools []*Tool `json:"tools,omitempty"`

		// // Optional. Associates model output to a specific function call.
		// ToolConfig *ToolConfig `json:"toolConfig,omitempty"`
	}
	if *flagSeed >= 0 {
		config.Seed = ptr(int32(*flagSeed))
	}
	if *flagSysFile != "" {
		data, err := os.ReadFile(*flagSysFile)
		if err != nil {
			log.Fatal(err)
		}
		config.SystemInstruction = genai.Text(string(data))[0]
	}
	if *flagSys != "" {
		if config.SystemInstruction == nil {
			config.SystemInstruction = genai.Text(*flagSys)[0]
		} else {
			config.SystemInstruction.Parts = append(config.SystemInstruction.Parts, genai.Text(*flagSys)[0].Parts...)
		}
	}
	if *flagMaxOutput > 0 {
		config.MaxOutputTokens = int32(*flagMaxOutput)
	}
	if *flagThink || *flagThinkBudget >= 0 {
		config.ThinkingConfig = &genai.ThinkingConfig{
			IncludeThoughts: *flagThink,
		}
		if *flagThinkBudget >= 0 {
			config.ThinkingConfig.ThinkingBudget = ptr(int32(*flagThinkBudget))
		}
	}
	if *flagCode {
		config.Tools = append(config.Tools, &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}})
	}
	if *flagComputer {
		// This seems to do nothing.
		config.Tools = append(config.Tools, &genai.Tool{ComputerUse: &genai.ToolComputerUse{Environment: genai.EnvironmentBrowser}})
	}
	if *flagMaps {
		config.Tools = append(config.Tools, &genai.Tool{GoogleMaps: &genai.GoogleMaps{}})
	}
	if *flagGoogle {
		config.Tools = append(config.Tools, &genai.Tool{GoogleSearch: &genai.GoogleSearch{}})
	}
	if *flagGoogleRAG {
		config.Tools = append(config.Tools, &genai.Tool{GoogleSearchRetrieval: &genai.GoogleSearchRetrieval{DynamicRetrievalConfig: &genai.DynamicRetrievalConfig{Mode: genai.DynamicRetrievalConfigModeDynamic}}})
	}
	if *flagURLs {
		config.Tools = append(config.Tools, &genai.Tool{URLContext: &genai.URLContext{}})
	}
	if *flagRot13 {
		config.Tools = append(config.Tools, rot13Tool)
	}

	logJSON(lf, "config", config)

	first := true
	var script []*genai.Content
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		fmt.Fprintf(os.Stderr, "\n")
		content := &genai.Content{Role: "user"}
		if first {
			first = false
			if *flagAttach != "" {
				data, err := os.ReadFile(*flagAttach)
				if err != nil {
					log.Fatal(err)
				}
				typ := http.DetectContentType(data)
				typ = "text/plain"
				content.Parts = append(content.Parts, &genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: typ}})
			}
		}
		content.Parts = append(content.Parts, &genai.Part{Text: line})
		logJSON(lf, "script", content)
		script = append(script, content)
	Resend:
		start := time.Now()
		debugPrint(script)
		r, err := client.Models.GenerateContent(ctx, *model, script, config)
		if err != nil {
			log.Fatal(err)
		}
		logJSON(lf, "response", r)
		dt := time.Since(start)
		usage := r.UsageMetadata
		fmt.Fprintf(os.Stderr, "# %din+%dthink+%dtool+%dout=%d tokens (%d cached), %.1fs\n", usage.PromptTokenCount, usage.ThoughtsTokenCount, usage.ToolUsePromptTokenCount, usage.CandidatesTokenCount, usage.TotalTokenCount, usage.CachedContentTokenCount, dt.Seconds())
		debugPrint(r)
		if len(r.Candidates) == 0 || r.Candidates[0].Content == nil || len(r.Candidates[0].Content.Parts) == 0 {
			log.Print("no candidate responses\n")
			continue
		}
		cand := r.Candidates[0]
		logJSON(lf, "script", cand.Content)
		script = append(script, cand.Content)
		responded := false
		for _, p := range cand.Content.Parts {
			if code := p.ExecutableCode; code != nil {
				fmt.Printf("# %s\n%s\n", strings.ToLower(string(code.Language)), code.Code)
			}
			if code := p.CodeExecutionResult; code != nil {
				fmt.Printf("%s\n%s\n", code.Outcome, code.Output)
			}
			if fn := p.FunctionCall; fn != nil {
				var args rot13Args
				if err := schema.Unmarshal(fn.Args, &args, "args"); err != nil {
					panic(err)
				}
				reply, err := rot13(ctx, &args)
				if err != nil {
					panic(err)
				}
				js, err := schema.Marshal(reply, "reply")
				if err != nil {
					panic(err)
				}
				resp := &genai.Content{
					Role: "user",
					Parts: []*genai.Part{
						{
							FunctionResponse: &genai.FunctionResponse{
								ID:       fn.ID,
								Name:     fn.Name,
								Response: map[string]any{"output": js},
							},
						},
					},
				}
				debugPrint(resp)
				logJSON(lf, "script", resp)
				script = append(script, resp)
				responded = true
			}
			if p.Text != "" {
				if p.Thought {
					fmt.Printf("<THINK>\n%s</THINK>\n", p.Text)
					continue
				}
				fmt.Printf("%s\n", p.Text)
			}
		}
		fmt.Fprintf(os.Stderr, "\n")
		if responded {
			goto Resend
		}
	}
}

func ptr[T any](x T) *T { return &x }

func debugPrint(x any) {
	response, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "%s\n", response)
}

type rot13Args struct {
	Text string `tool:"text to be translated"`
}

type rot13Reply struct {
	Grkg string `tool:"rot13 of input text"`
}

func rot13(ctx context.Context, in *rot13Args) (*rot13Reply, error) {
	out := []byte(in.Text)
	for i, b := range out {
		if 'A' <= b && b <= 'M' || 'a' <= b && b <= 'm' {
			out[i] = b + 13
		} else if 'N' <= b && b <= 'Z' || 'n' <= b && b <= 'z' {
			out[i] = b - 13
		}
	}
	return &rot13Reply{Grkg: string(out)}, nil
}

func mustType[T any]() *genai.Schema {
	t, err := schema.Type(reflect.TypeFor[T]())
	if err != nil {
		panic(err)
	}
	return t
}

var rot13Tool = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{{
		Name:        "rot13",
		Description: "applies rot13 encoding to text",
		Parameters:  mustType[*rot13Args](),
		Response:    mustType[*rot13Reply](),
	}},
}

func logFile() *os.File {
	dir := filepath.Join(home, ".gadget/log")
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Fatal(err)
		}
	}
	file := time.Now().UTC().Format("2006-01-02-150405")
	id := big.NewInt(int64(rand.Int64())).Text(36)
	for len(id) < 10 {
		id = "0" + id
	}
	file += "-" + id[:7]

	f, err := os.Create(filepath.Join(dir, file))
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func logJSON(f *os.File, verb string, arg any) {
	line := []byte(verb)
	if arg != nil {
		js, err := json.Marshal(arg)
		if err != nil {
			log.Fatal(err)
		}
		line = append(line, ' ')
		line = append(line, js...)
	}
	line = append(line, '\n')
	if _, err := f.Write(line); err != nil {
		log.Fatal(err)
	}
}
