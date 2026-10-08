package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func loadStructuresCorpus(dir string) ([]StructureCase, error) {
	var cases, oracle []StructureCase
	for name, target := range map[string]*[]StructureCase{"cases.json": &cases, "dumps.json": &oracle} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return nil, err
		}
	}
	if len(cases) != len(oracle) {
		return nil, fmt.Errorf("structures corpus/oracle lengths differ")
	}
	for i := range cases {
		if cases[i].Name != oracle[i].Name || oracle[i].SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(oracle[i].Dump))) {
			return nil, fmt.Errorf("structure oracle %d name/checksum mismatch", i)
		}
		oracle[i].Abap = cases[i].Abap
		oracle[i].Filename = cases[i].Filename
	}
	return oracle, nil
}

func structuresTestClass(cases []StructureCase, names *hir.Names) string {
	var b strings.Builder
	line := func(s string, args ...any) { fmt.Fprintf(&b, s+"\n", args...) }
	class := names.Get("harness/structures_dump.ts.StructuresDump")
	line("CLASS ltcl_structures_diff DEFINITION FOR TESTING DURATION LONG RISK LEVEL HARMLESS.")
	line("PRIVATE SECTION.")
	line("METHODS structures FOR TESTING.")
	for i := range cases {
		line("METHODS data_%03d EXPORTING raw TYPE string expected TYPE string.", i)
	}
	line("ENDCLASS.")
	line("CLASS ltcl_structures_diff IMPLEMENTATION.")
	line("METHOD structures.")
	line("DATA raw TYPE string.")
	line("DATA expected TYPE string.")
	line("DATA actual TYPE string.")
	line("DATA checksum TYPE string.")
	line("DATA completed TYPE i.")
	line("DATA equal_cases TYPE i.")
	line("DATA msg TYPE string.")
	line("DATA first_mismatch TYPE string.")
	line("DATA err TYPE REF TO cx_root.")
	for i, c := range cases {
		line("CALL METHOD data_%03d IMPORTING raw = raw expected = expected.", i)
		line("TRY.")
		line("CALL METHOD %s=>%s EXPORTING %s = raw %s = `%s` RECEIVING result = actual.", class, names.Get("member.dump"), names.Get("param.raw"), names.Get("param.filename"), c.Filename)
		line("CATCH cx_root INTO err.")
		line("actual = `EXCEPTION|` && err->get_text( ).")
		line("ENDTRY.")
		line("cl_abap_message_digest=>calculate_hash_for_char( EXPORTING if_algorithm = `SHA256` if_data = actual IMPORTING ef_hashstring = checksum ).")
		line("checksum = to_lower( checksum ).")
		line("completed = completed + 1.")
		line("IF actual = expected AND %s=>%s = %d AND checksum = `%s`.", class, names.Get("member.lastStructures"), c.Structures, c.SHA256)
		line("equal_cases = equal_cases + 1.")
		line("ELSEIF first_mismatch IS INITIAL.")
		line("first_mismatch = `%s`.", strings.ReplaceAll(c.Name, "`", "``"))
		line("IF strlen( actual ) > 180.")
		line("first_mismatch = first_mismatch && ` actual=` && actual(180).")
		line("ELSE.")
		line("first_mismatch = first_mismatch && ` actual=` && actual.")
		line("ENDIF.")
		line("ENDIF.")
	}
	line("msg = |structures { equal_cases }/{ completed } first { first_mismatch }|.")
	line("cl_abap_unit_assert=>assert_equals( act = completed exp = %d msg = `completion` ).", len(cases))
	line("cl_abap_unit_assert=>assert_equals( act = equal_cases exp = %d msg = msg ).", len(cases))
	line("ENDMETHOD.")
	for i, c := range cases {
		line("METHOD data_%03d.", i)
		line("DATA ch TYPE c LENGTH 1.")
		line("CLEAR raw.")
		line("CLEAR expected.")
		for _, s := range abapStringBuild("raw", "ch", c.Abap) {
			line("%s", s)
		}
		for _, s := range abapStringBuild("expected", "ch", c.Dump) {
			line("%s", s)
		}
		line("ENDMETHOD.")
	}
	line("ENDCLASS.")
	return b.String()
}

func TestEmitStructuresClosure(t *testing.T) {
	if os.Getenv("STRUCTURES_EMIT") == "" {
		t.Skip("set STRUCTURES_EMIT=1 to emit structures and differential")
	}
	prog, diags := lowerStmtsClosure(t)
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	if os.Getenv("STRUCTURES_MUTATE") != "" {
		skipStructureSequenceEnd(t, prog)
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	cases, err := loadStructuresCorpus(filepath.Join("testdata", "structurescorpus"))
	if err != nil {
		t.Fatal(err)
	}
	files[names.Get("harness/structures_dump.ts.StructuresDump")+".clas.testclasses.abap"] = structuresTestClass(cases, names)
	if benchmark := os.Getenv("STRUCTURES_BENCH"); benchmark != "" {
		var input, oracle []StructureCase
		for name, target := range map[string]*[]StructureCase{"benchmark.json": &input, "benchmark-dumps.json": &oracle} {
			raw, err := os.ReadFile(filepath.Join(benchmark, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, target); err != nil {
				t.Fatal(err)
			}
		}
		if len(input) != 1 || len(oracle) != 1 || input[0].Name != oracle[0].Name || oracle[0].SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(oracle[0].Dump))) {
			t.Fatal("invalid benchmark oracle")
		}
		c := oracle[0]
		c.Abap = input[0].Abap
		c.Filename = input[0].Filename
		files["zcl_phase3_benchmark.clas.abap"] = StructuresBenchmarkClass(c, c.LexerSHA256, c.StatementsSHA256, names)
		files["zcl_phase3_benchmark.clas.testclasses.abap"] = StructuresBenchmarkTest()
		files["zphase3_structures_bench.prog.abap"] = StructuresBenchmarkReport()
	}
	lines, bytes := 0, 0
	for name, source := range files {
		lines += strings.Count(source, "\n")
		bytes += len(source)
		for _, s := range strings.Split(source, "\n") {
			if len(s) > 255 {
				t.Fatalf("%s oversized line %d", name, len(s))
			}
		}
		if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
			if err := os.MkdirAll(out, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("emitted %d ABAP files, %d lines, %d bytes; %d differential cases", len(files), lines, bytes, len(cases))
}

// The acceptance mutant skips the closing element of the IF Sequence.
// It is emitted only to an explicitly requested separate output directory.
func skipStructureSequenceEnd(t *testing.T, prog *hir.Program) {
	t.Helper()
	for _, c := range prog.Classes {
		if c.Name != "src/abap/3_structures/structures/_combi.ts.Sequence" {
			continue
		}
		for _, m := range c.Methods {
			if m.Name != "run" {
				continue
			}
			var walk func(*hir.Stmt) bool
			walk = func(s *hir.Stmt) bool {
				if s == nil {
					return false
				}
				if s.Kind == hir.ForEach {
					list := &hir.Expr{Kind: hir.FieldGet, Name: "list", Type: hir.T(hir.Array, s.Type), X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}}
					first := &hir.Expr{Kind: hir.IndexGet, Type: s.Type, X: list, Y: hir.L(hir.T(hir.I32), 0)}
					keywords := &hir.Expr{Kind: hir.VirtualCall, Name: "first", Type: hir.T(hir.Array, hir.T(hir.String)), X: first}
					isIf := &hir.Expr{Kind: hir.RuntimeOp, Op: "array.includes", Type: hir.T(hir.Bool), X: keywords, Args: []*hir.Expr{hir.L(hir.T(hir.String), "IF")}}
					end := &hir.Expr{Kind: hir.IndexGet, Type: s.Type, X: list, Y: hir.L(hir.T(hir.I32), 2)}
					equal := &hir.Expr{Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: hir.V(s.Name, s.Type), Y: end}
					skip := &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Binary, Op: "&&", Type: hir.T(hir.Bool), X: isIf, Y: equal}, Body: hir.B(&hir.Stmt{Kind: hir.Continue})}
					s.Body = hir.B(skip, s.Body)
					return true
				}
				for _, child := range s.List {
					if walk(child) {
						return true
					}
				}
				return walk(s.Body) || walk(s.Else)
			}
			if walk(m.Body) {
				return
			}
		}
	}
	t.Fatal("structure Sequence.run loop not found")
}
