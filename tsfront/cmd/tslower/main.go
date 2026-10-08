// Command tslower lowers TypeScript sources of a tsfront project into the
// object HIR, verifies it, emits ABAP, and reports what was lowered: files,
// classes, methods, and diagnostics by category with counts. With -corpus it
// also generates the ABAP Unit differential test against the oracle dumps
// produced by tools/lexer-oracle.mjs.
//
// Typical use (phase 1, abaplint's lexer):
//
//	tslower -tsconfig tsfront/testdata/lexer/tsconfig.json \
//	        -out "$dir" -corpus tsfront/testdata/lexercorpus \
//	        src/position.ts src/virtual_position.ts src/files/_ifile.ts \
//	        src/abap/1_lexer/*.ts src/abap/1_lexer/tokens/*.ts harness/*.ts
//
// File arguments are resolved against the tsconfig directory; pass them in a
// deterministic order (declaration order in the HIR follows it).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront"
)

func main() {
	tsconfig := flag.String("tsconfig", "tsconfig.json", "path to the project's tsconfig.json")
	out := flag.String("out", "", "directory to write the emitted ABAP to (default: no files written)")
	corpus := flag.String("corpus", "", "directory with cases.json/tokens.json; generates the differential unit test")
	assumeInteger := flag.Bool("assume-only-integer-calculations", os.Getenv("ABAPITI_ASSUME_INT") == "1", "explicit integer number contract: unknown number becomes int8")
	exceptionPath := flag.String("integer-exceptions", "", "JSON fingerprinted integer exception list")
	jsonOut := flag.Bool("json", false, "write the report as JSON")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: tslower [-tsconfig path] [-out dir] [-corpus dir] [-json] file...\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	program, err := tsfront.Load(*tsconfig)
	if err != nil {
		fatal(err)
	}
	options := tsfront.LowerOptions{AssumeOnlyIntegerCalculations: *assumeInteger}
	if *exceptionPath != "" {
		if !*assumeInteger {
			fatal(fmt.Errorf("integer exceptions require integer mode"))
		}
		raw, err := os.ReadFile(*exceptionPath)
		if err != nil {
			fatal(err)
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&options.IntegerExceptions); err != nil {
			fatal(err)
		}
	}
	prog, diags, err := program.LowerWithOptions(flag.Args(), options)
	if err != nil {
		fatal(err)
	}
	verifyErrors := hir.Verify(prog)

	report := map[string]any{"files": len(flag.Args())}
	report["classes"] = len(prog.Classes)
	report["interfaces"] = len(prog.Interfaces)
	methods := 0
	fields := 0
	for _, c := range prog.Classes {
		methods += len(c.Methods)
		fields += len(c.Fields)
		if c.Ctor != nil {
			methods++
		}
	}
	report["methods"] = methods
	report["fields"] = fields
	byCategory := map[string]int{}
	var numberSites []string
	examples := map[string][]string{}
	for _, d := range diags {
		if d.Category == "note-number-ranges" {
			numberSites = append(numberSites, d.Message)
		}
		byCategory[d.Category]++
		if len(examples[d.Category]) < 2 {
			examples[d.Category] = append(examples[d.Category], d.String())
		}
	}
	if *assumeInteger {
		sites := []string{}
		for _, d := range diags {
			if d.Category == "assume-integer" || d.Category == "note-integer-exception" {
				sites = append(sites, d.String())
			}
		}
		report["integerExceptionSites"] = sites
		if !*jsonOut {
			for _, site := range sites {
				fmt.Println(site)
			}
			if len(sites) == 0 {
				fmt.Println("integer exceptions: 0 sites")
			}
		}
	}
	report["numberSites"] = numberSites
	report["diagnostics"] = byCategory
	report["examples"] = examples
	blocking := 0
	for c := range byCategory {
		if !strings.HasPrefix(c, "note-") && c != "skipped-computed-name" {
			blocking += byCategory[c]
		}
	}
	report["blocking"] = blocking
	if len(verifyErrors) > 0 {
		var msgs []string
		for _, e := range verifyErrors {
			msgs = append(msgs, e.Error())
		}
		report["verify"] = msgs
	}

	status := 0
	if blocking > 0 || len(verifyErrors) > 0 {
		status = 1
	}
	files := map[string]string{}
	if status == 0 && (*out != "" || *corpus != "") {
		var names *hir.Names
		var err error
		files, names, err = abap.EmitNamed(prog)
		if err != nil {
			report["emit"] = err.Error()
			status = 1
		} else {
			report["abapFiles"] = len(files)
			lines := 0
			maxLen := 0
			for _, src := range files {
				for _, line := range strings.Split(src, "\n") {
					lines++
					if len(line) > maxLen {
						maxLen = len(line)
					}
				}
			}
			report["abapLines"] = lines
			report["abapMaxLine"] = maxLen
			if *corpus != "" {
				cases, err := tsfront.LoadLexerCorpus(*corpus)
				if err != nil {
					fatal(err)
				}
				params := tsfront.DriverParams{
					IntegerNumbers: *assumeInteger,
					Class:          names.Get("harness/lexer_dump.ts.LexerDump"),
					Dump:           names.Get("member.dump"),
					TokenCount:     names.Get("member.tokenCount"),
					Virtual:        names.Get("member.virtualProbe"),
					Diff:           names.Get("member.firstDiff"),
					Raw:            names.Get("param.raw"),
					A:              names.Get("param.a"),
					B:              names.Get("param.b"),
				}
				files[params.Class+".clas.testclasses.abap"] = tsfront.LexerTestClass(cases, params)
				report["corpusCases"] = len(cases)
			}
		}
	}
	if *out != "" && status == 0 {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			fatal(err)
		}
		for name, src := range files {
			if err := os.WriteFile(filepath.Join(*out, name), []byte(src), 0o644); err != nil {
				fatal(err)
			}
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", " ")
		if err := enc.Encode(report); err != nil {
			fatal(err)
		}
	} else {
		var keys []string
		for c := range byCategory {
			keys = append(keys, c)
		}
		sort.Strings(keys)
		fmt.Printf("files %v, classes %v (+%v interfaces), methods %v, fields %v\n",
			report["files"], report["classes"], report["interfaces"], report["methods"], report["fields"])
		for _, c := range keys {
			fmt.Printf("  %-24s %d\n", c, byCategory[c])
			for _, e := range examples[c] {
				fmt.Printf("    %s\n", e)
			}
		}
		if v, ok := report["verify"].([]string); ok {
			for _, m := range v {
				fmt.Printf("verify: %s\n", m)
			}
		}
		if status == 0 {
			fmt.Printf("abap: %v files, %v lines, longest line %v bytes\n", report["abapFiles"], report["abapLines"], report["abapMaxLine"])
			if report["corpusCases"] != nil {
				fmt.Printf("corpus: %v differential cases\n", report["corpusCases"])
			}
		}
	}
	os.Exit(status)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tslower:", err)
	os.Exit(2)
}
