package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The differential corpus: ABAP snippets and real files (cases.json) plus the
// token dumps the original TypeScript lexer produced for them (tokens.json,
// written by tools/lexer-oracle.mjs against the abaplint build output).

// LexerCase is one corpus entry with its oracle dump.
type LexerCase struct {
	Name   string `json:"name"`
	Abap   string `json:"abap"`
	Dump   string `json:"dump"`
	Tokens int    `json:"tokens"`
}

// LoadLexerCorpus reads cases.json and tokens.json from dir and joins them in
// order; both files must describe the same cases in the same order.
func LoadLexerCorpus(dir string) ([]LexerCase, error) {
	var cases []LexerCase
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		return nil, err
	}
	raw, err = os.ReadFile(filepath.Join(dir, "tokens.json"))
	if err != nil {
		return nil, err
	}
	var oracle []LexerCase
	if err := json.Unmarshal(raw, &oracle); err != nil {
		return nil, err
	}
	if len(oracle) != len(cases) {
		return nil, fmt.Errorf("corpus has %d cases, oracle %d", len(cases), len(oracle))
	}
	for i := range cases {
		if oracle[i].Name != cases[i].Name {
			return nil, fmt.Errorf("case %d: %q vs oracle %q", i, cases[i].Name, oracle[i].Name)
		}
		cases[i].Dump = oracle[i].Dump
		cases[i].Tokens = oracle[i].Tokens
	}
	return cases, nil
}

// DriverParams carries the mangled ABAP names the generated unit test needs:
// the lowered driver class, its dump and firstDiff methods, and those
// methods' parameter names.
type DriverParams struct {
	IntegerNumbers                                    bool
	Class, Dump, Diff, Raw, A, B, TokenCount, Virtual string
}

// LexerTestClass generates the ABAP Unit test class that runs the lowered
// lexer over every corpus case and compares against the embedded oracle
// dumps: one test method, one block per case (osgo limits the method count).
// The comparison itself is lowered TypeScript (LexerDump.firstDiff), so
// trailing blanks and code units count exactly.
func LexerTestClass(cases []LexerCase, p DriverParams) string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("CLASS ltcl_lexer_diff DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.")
	line("PRIVATE SECTION.")
	line("DATA executed TYPE i.")
	line("METHODS teardown.")
	line("METHODS tokens FOR TESTING.")
	line("ENDCLASS.")
	line("CLASS ltcl_lexer_diff IMPLEMENTATION.")
	line("METHOD teardown.")
	line("cl_abap_unit_assert=>assert_equals( act = executed exp = %d msg = `corpus completion` ).", len(cases))
	line("ENDMETHOD.")
	line("METHOD tokens.")
	for _, v := range []string{"raw", "expected", "actual", "msg"} {
		line("DATA %s TYPE string.", v)
	}
	if p.IntegerNumbers {
		line("DATA diff TYPE int8.")
	} else {
		line("DATA diff TYPE f.")
	}
	line("DATA idx TYPE i.")
	line("DATA ch TYPE c LENGTH 1.")
	line("DATA virtual TYPE abap_bool.")
	for i, c := range cases {
		line("CLEAR raw.")
		for _, s := range abapStringBuild("raw", "ch", c.Abap) {
			line("%s", s)
		}
		line("CLEAR expected.")
		for _, s := range abapStringBuild("expected", "ch", c.Dump) {
			line("%s", s)
		}
		line("CALL METHOD %s=>%s EXPORTING %s = raw RECEIVING result = actual.", p.Class, p.Dump, p.Raw)
		line("CALL METHOD %s=>%s EXPORTING %s = actual %s = expected RECEIVING result = diff.", p.Class, p.Diff, p.A, p.B)
		line("idx = %d.", i+1)
		line("msg = |case %s { idx }|.", abapTemplate(c.Name))
		line("cl_abap_unit_assert=>assert_equals( act = diff exp = -1 msg = msg ).")
		line("cl_abap_unit_assert=>assert_equals( act = %s=>%s exp = %d msg = msg ).", p.Class, p.TokenCount, c.Tokens)
		line("executed = executed + 1.")
	}
	line("CALL METHOD %s=>%s RECEIVING result = virtual.", p.Class, p.Virtual)
	line("cl_abap_unit_assert=>assert_equals( act = virtual exp = abap_true msg = `virtual positions` ).")
	line("ENDMETHOD.")
	line("ENDCLASS.")
	return b.String()
}

// abapTemplate escapes a string for use inside a |...| string template.
func abapTemplate(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "|", "\\|", "{", "\\{", "}", "\\}")
	return r.Replace(s)
}

// abapStringBuild emits statements that build value in variable varname,
// chunk by chunk, like the HIR emitter's string literals: printable runs in
// backtick literals, control characters through uccpi. ch must be a c LENGTH 1
// variable. Every line stays far below the 255-byte limit.
func abapStringBuild(varname, ch, value string) []string {
	var out []string
	r := []rune(value)
	for len(r) > 0 {
		if r[0] < 32 || r[0] == 127 || r[0] == 0xfeff {
			out = append(out, fmt.Sprintf("%s = cl_abap_conv_in_ce=>uccpi( %d ).", ch, r[0]))
			out = append(out, fmt.Sprintf("CONCATENATE %s %s INTO %s RESPECTING BLANKS.", varname, ch, varname))
			r = r[1:]
			continue
		}
		k, size := 0, 0
		for k < len(r) && r[k] >= 32 && r[k] != 127 && r[k] != 0xfeff && size+len(string(r[k]))*2 <= 120 {
			size += len(string(r[k])) * 2
			k++
		}
		if k == 0 {
			k = 1
		}
		chunk := strings.ReplaceAll(string(r[:k]), "`", "``")
		out = append(out, fmt.Sprintf("CONCATENATE %s `%s` INTO %s RESPECTING BLANKS.", varname, chunk, varname))
		r = r[k:]
	}
	return out
}
