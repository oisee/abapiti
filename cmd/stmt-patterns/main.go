// stmt-patterns measures emitted ABAP; it never rewrites compiler output.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront"
)

type statement struct {
	Text        string
	Line, Depth int
}
type method struct {
	Class, ID, Name, ABAP, Source, Profile string
	Statements                             []statement
	TS, Ops, HIR, HIROps                   int
	Mix                                    map[string]int
	Patterns                               map[string][2]int
}
type occurrence struct {
	Method, Pattern, Example string
	Line, Depth, Saved       int
}
type gram struct {
	Key, Example, Method string
	N, Count, Loops      int
}

var tempRE = regexp.MustCompile(`(?i)\bt[0-9]+\b`)
var wordRE = regexp.MustCompile(`[a-zA-Z_][a-zA-Z_0-9]*|<[^>]+>`)
var declRE = regexp.MustCompile(`(?i)^DATA\(\s*([a-zA-Z_][a-zA-Z_0-9]*)\s*\)\s*=\s*(.*)\.$`)
var assignRE = regexp.MustCompile(`(?i)^([a-zA-Z_][a-zA-Z_0-9]*)\s*(\?=|=)\s*(.*)\.$`)
var nameRE = regexp.MustCompile(`(?i)\bz_[a-z0-9_]+\b`)
var numberRE = regexp.MustCompile(`\b[0-9]+\b`)
var patterns = []string{"temp_assign", "temp_argument", "cast_temp", "value_init_overwritten", "literal_constructor", "literal_range_build", "receiver_copy", "bool_if", "return_copy", "table_read_copy", "clear_before_write", "literal_scalar", "range_bound_assignment"}

func main() {
	input := flag.String("input", "out", "emitted directory containing classes/ and names.json")
	source := flag.String("source", ".local/stmt-patterns/source/packages/core", "pinned TypeScript core directory")
	output := flag.String("output", "docs/history/2026-10-10-stmt-patterns", "CSV destination")
	hirFlag := flag.Bool("hir", false, "also lower the pinned closure and count HIR nodes (uses -work)")
	work := flag.String("work", ".local/stmt-patterns/hir", "workspace for optional HIR lowering")
	flag.Parse()
	if *peepholeFlag {
		if err := exportPeepholes(*input, *output); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(*input, *source, *output, *work, *hirFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(input, source, output, work string, withHIR bool) error {
	if err := tsfront.EmbeddedRegistryClosure().Verify(source); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(input, "names.json"))
	if err != nil {
		return err
	}
	names := map[string]abap.SourceEntry{}
	if err = json.Unmarshal(raw, &names); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(input, "classes", "*.clas.abap"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	var methods []*method
	for _, file := range files {
		class := strings.ToUpper(strings.TrimSuffix(filepath.Base(file), ".clas.abap"))
		entry := names[class]
		if entry.Kind != "class" || !selected(entry.ID) {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		for _, m := range parseMethods(string(data)) {
			m.Class, m.ID = class, entry.ID
			member := names[strings.ToUpper(m.ABAP)]
			m.Name = strings.TrimPrefix(member.ID, "member.")
			if m.Name == "" {
				m.Name = m.ABAP
			}
			if m.ABAP == "constructor" {
				m.Name = "constructor"
				m.Source = entry.Source
			}
			for _, d := range member.Declared {
				if strings.HasPrefix(d, entry.ID+"."+m.Name+"@") {
					m.Source = strings.SplitN(d, "@", 2)[1]
					break
				}
			}
			if strings.HasSuffix(entry.ID, ".Sequence") || isCombi(entry.ID) {
				if m.Name != "run" && m.Name != "run_one" {
					continue
				}
			}
			if strings.HasSuffix(entry.ID, ".Lexer") && m.Name != "process" && m.Name != "add" {
				continue
			}
			if m.Source == "" {
				continue
			}
			m.Profile = profile(entry.ID, m.Name)
			for _, st := range m.Statements {
				if strings.Contains(st.Text, "z_exception_un_5f2d02892fe8b8") {
					m.Profile = "trapped (not executed)"
					break
				}
			}
			methods = append(methods, m)
		}
	}
	sort.Slice(methods, func(i, j int) bool { return identity(methods[i]) < identity(methods[j]) })
	if len(methods) == 0 {
		return fmt.Errorf("no selected methods in %s", input)
	}
	if err = countTS(methods, source); err != nil {
		return err
	}
	if withHIR {
		if err = countHIR(methods, source, work, input, output); err != nil {
			return err
		}
	}
	var occurrences []occurrence
	grams := map[string]*gram{}
	for _, m := range methods {
		m.Mix = map[string]int{}
		m.Patterns = map[string][2]int{}
		for _, s := range m.Statements {
			m.Mix[mix(s.Text)]++
		}
		occurrences = append(occurrences, detect(m)...)
		for _, n := range []int{2, 3} {
			for i := 0; i+n <= len(m.Statements); i++ {
				parts := []string{}
				ex := []string{}
				loop := true
				for _, s := range m.Statements[i : i+n] {
					parts = append(parts, normalize(s.Text))
					ex = append(ex, s.Text)
					if s.Depth == 0 {
						loop = false
					}
				}
				key := strings.Join(parts, " | ")
				g := grams[key]
				if g == nil {
					g = &gram{Key: key, N: n, Example: strings.Join(ex, " "), Method: identity(m) + ":" + strconv.Itoa(m.Statements[i].Line)}
					grams[key] = g
				}
				g.Count++
				if loop {
					g.Loops++
				}
			}
		}
	}
	if err = os.MkdirAll(output, 0755); err != nil {
		return err
	}
	var rows [][]string
	for _, m := range methods {
		rows = append(rows, []string{identity(m), m.Class, m.ABAP, m.Source, m.Profile, tsOrigin(m), itoa(len(m.Statements)), itoa(len(m.Statements) - m.Mix["declaration"] - m.Mix["control_marker"]), itoa(m.TS), itoa(m.Ops), ratio(len(m.Statements), m.TS), ratio(len(m.Statements), m.Ops), itoa(m.HIR), itoa(m.HIROps)})
	}
	if err = writeCSV(output, "methods.csv", []string{"method", "abap_class", "abap_method", "source", "profile", "ts_denominator_origin", "abap_statements", "executable_site_proxy", "ts_statements", "ts_operations", "abap_per_ts_statement", "abap_per_ts_operation", "hir_statements", "hir_operations"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, m := range methods {
		keys := sortedKeys(m.Mix)
		for _, k := range keys {
			rows = append(rows, []string{identity(m), k, itoa(m.Mix[k])})
		}
	}
	if err = writeCSV(output, "mix.csv", []string{"method", "kind", "count"}, rows); err != nil {
		return err
	}
	rows = nil
	totals := map[string][2]int{}
	for _, m := range methods {
		for _, p := range patterns {
			v := m.Patterns[p]
			t := totals[p]
			t[0] += v[0]
			t[1] += v[1]
			totals[p] = t
			rows = append(rows, []string{identity(m), p, itoa(v[0] + v[1]), itoa(v[0]), itoa(v[1]), itoa(v[0]), itoa(v[1])})
		}
	}
	for _, p := range patterns {
		v := totals[p]
		rows = append(rows, []string{"TOTAL", p, itoa(v[0] + v[1]), itoa(v[0]), itoa(v[1]), itoa(v[0]), itoa(v[1])})
	}
	if err = writeCSV(output, "patterns.csv", []string{"method", "pattern", "sites", "outside_loops", "inside_loops", "straight_line_saved_sites", "saved_sites_per_loop_visit"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, o := range occurrences {
		rows = append(rows, []string{o.Method, o.Pattern, itoa(o.Line), itoa(o.Depth), itoa(o.Saved), o.Example})
	}
	if err = writeCSV(output, "occurrences.csv", []string{"method", "pattern", "line", "loop_depth", "potential_saved", "example"}, rows); err != nil {
		return err
	}
	gs := []*gram{}
	for _, g := range grams {
		gs = append(gs, g)
	}
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].Count != gs[j].Count {
			return gs[i].Count > gs[j].Count
		}
		return gs[i].Key < gs[j].Key
	})
	rows = nil
	for _, g := range gs {
		rows = append(rows, []string{itoa(g.N), itoa(g.Count), itoa(g.Loops), g.Key, g.Method, g.Example})
	}
	if err = writeCSV(output, "ngrams.csv", []string{"n", "count", "inside_loops", "normalized", "example_location", "example"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, m := range methods {
		for _, p := range patterns {
			v := m.Patterns[p]
			rows = append(rows, []string{identity(m), m.Profile, p, itoa(v[0]), itoa(v[1]), fmt.Sprintf("%d + %d*I", v[0], v[1]), "site visits; branch-dependent; I includes nesting; overlaps not additive"})
		}
	}
	if err = writeCSV(output, "savings.csv", []string{"method", "profile", "pattern", "outside_saved_sites", "loop_saved_sites", "per_call_equal_visits_envelope", "assumptions"}, rows); err != nil {
		return err
	}
	if err = writeManifest(input, output, files, withHIR); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "| Method | ABAP | Exec sites* | TS stmts | TS ops | ABAP/TS stmt | ABAP/TS op | HIR stmts | Profile |\n|---|---:|---:|---:|---:|---:|---:|---:|---|")
	for _, m := range methods {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %s | %s | %d | %s |\n", shortID(m), len(m.Statements), len(m.Statements)-m.Mix["declaration"]-m.Mix["control_marker"], m.TS, m.Ops, ratio(len(m.Statements), m.TS), ratio(len(m.Statements), m.Ops), m.HIR, m.Profile)
	}
	fmt.Fprintln(&b, "\n| Pattern | Total | Outside loops | Inside loops |\n|---|---:|---:|---:|")
	for _, p := range patterns {
		v := totals[p]
		fmt.Fprintf(&b, "| %s | %d | %d | %d |\n", p, v[0]+v[1], v[0], v[1])
	}
	fmt.Fprintln(&b, "\n| Rank | N | Count | In loops | Normalized | Example location | Example |\n|---:|---:|---:|---:|---|---|---|")
	for i, g := range gs {
		if i == 15 {
			break
		}
		fmt.Fprintf(&b, "| %d | %d | %d | %d | <code>%s</code> | %s | <code>%s</code> |\n", i+1, g.N, g.Count, g.Loops, md(g.Key), g.Method, md(g.Example))
	}
	if err = os.WriteFile(filepath.Join(output, "tables.md"), []byte(b.String()), 0644); err != nil {
		return err
	}
	fmt.Print(b.String())
	return nil
}
func identity(m *method) string { return m.ID + "." + m.Name }
func shortID(m *method) string  { p := strings.Split(m.ID, "."); return p[len(p)-1] + "." + m.Name }
func isCombi(id string) bool    { return strings.HasPrefix(id, "src/abap/2_statements/combi.ts.") }
func selected(id string) bool {
	if isCombi(id) {
		for _, c := range []string{"Sequence", "AlternativePriority", "Alternative", "Expression", "Token", "Word", "Regex", "Star", "Optional", "Plus", "Permutation"} {
			if strings.HasSuffix(id, "."+c) {
				return true
			}
		}
		return false
	}
	for _, wanted := range []string{"src/abap/1_lexer/lexer.ts.Lexer", "src/abap/1_lexer/lexer_stream.ts.LexerStream", "src/abap/2_statements/result.ts.Result", "src/abap/2_statements/statement_parser.ts.StatementParser", "src/abap/4_file_information/abap_file_information.ts.ABAPFileInformation"} {
		if id == wanted {
			return true
		}
	}
	return false
}
func profile(id, name string) string {
	c := id[strings.LastIndex(id, ".")+1:]
	switch c {
	case "Sequence", "AlternativePriority", "Expression", "Token", "Word", "Regex":
		if name == "run" || name == "run_one" {
			return "A4H hot LOOP (qualitative)"
		}
	case "Lexer":
		return "A4H hot (qualitative)"
	case "ABAPFileInformation":
		return "A4H lookup family (qualitative)"
	}
	return "supporting hot-path family"
}
func itoa(v int) string { return strconv.Itoa(v) }
func ratio(n, d int) string {
	if d == 0 {
		return "NA"
	}
	return fmt.Sprintf("%.2f", float64(n)/float64(d))
}
func md(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "`", "&#96;")
	return strings.ReplaceAll(s, "|", "&#124;")
}
func sortedKeys[V any](m map[string]V) []string {
	k := []string{}
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}
func writeCSV(dir, name string, header []string, rows [][]string) error {
	f, e := os.Create(filepath.Join(dir, name))
	if e != nil {
		return e
	}
	w := csv.NewWriter(f)
	if e = w.Write(header); e == nil {
		e = w.WriteAll(rows)
	}
	w.Flush()
	if e == nil {
		e = w.Error()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}

func tsOrigin(m *method) string {
	if strings.HasSuffix(m.Name, "_one") {
		return "original " + strings.TrimSuffix(m.Name, "_one") + " body (synthetic method)"
	}
	return "original method body"
}
