package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// The syntax-closure lowerings (interface heritage, inlined array callbacks,
// closures, parseInt, localeCompare, finally, shadowing, ...) are checked
// against observations of the original JavaScript fixture: only the unit
// driver is handwritten; every expected string comes from oracle.json.
func TestEmitSyntaxFeatures(t *testing.T) {
	dir := "testdata/syntaxfeatures"
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "probe.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "export class Probe")
	end := strings.LastIndex(text, "}") + 1
	registry, err := overrides.New(overrides.Entry{ID: "fixture-checked-cast", Key: overrides.Key{File: "probe.ts", Symbol: "Probe", Kind: "KindClassDeclaration"}, SHA256: overrides.Fingerprint(text[start:end]), Rationale: "fixture: the assertion sits under an instanceof of the same pure call", Patterns: &overrides.Patterns{CheckedCasts: map[string]bool{
		`Probe.base(sib) as Sibling2`: true,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"probe.ts"}, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "hir.txt"), []byte(hir.Dump(prog)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Raw, Needle string
		N           float64
		Flag        bool
		Expected    string
	}
	raw, err = os.ReadFile(filepath.Join(dir, "oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 5 {
		t.Fatal("missing oracle cases")
	}
	class := names.Get("probe.ts.Probe")
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("CLASS ltcl_syntax DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.")
	line("PRIVATE SECTION.")
	line("METHODS observations FOR TESTING.")
	line("ENDCLASS.")
	line("CLASS ltcl_syntax IMPLEMENTATION.")
	line("METHOD observations.")
	for _, name := range []string{"raw", "needle", "expected", "actual"} {
		line("DATA %s TYPE string.", name)
	}
	line("DATA n TYPE f.")
	line("DATA flag TYPE abap_bool.")
	line("DATA ch TYPE c LENGTH 1.")
	line("DATA probe TYPE REF TO %s.", class)
	for i, c := range cases {
		for _, v := range []struct{ name, value string }{{"raw", c.Raw}, {"needle", c.Needle}, {"expected", c.Expected}} {
			line("CLEAR %s.", v.name)
			for _, s := range abapStringBuild(v.name, "ch", v.value) {
				line("%s", s)
			}
		}
		line("n = '%s'.", fmt.Sprintf("%.0f", c.N))
		flag := "abap_false"
		if c.Flag {
			flag = "abap_true"
		}
		line("flag = %s.", flag)
		line("CREATE OBJECT probe.")
		line("CALL METHOD probe->%s EXPORTING %s = raw %s = needle %s = n %s = flag RECEIVING result = actual.", names.Get("member.run"), names.Get("param.raw"), names.Get("param.needle"), names.Get("param.n"), names.Get("param.flag"))
		line("cl_abap_unit_assert=>assert_equals( act = actual exp = expected msg = `syntax case %d` ).", i+1)
	}
	line("ENDMETHOD.")
	line("ENDCLASS.")
	files[class+".clas.testclasses.abap"] = b.String()
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("%d emitted files, %d original observations", len(files), len(cases))
}
