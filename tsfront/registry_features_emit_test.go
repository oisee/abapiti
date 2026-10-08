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

// Production methods come from tsgo -> HIR -> ABAP. Only the unit driver is
// handwritten; expected strings are observations from the original JS fixture.
func TestEmitRegistryFeatures(t *testing.T) {
	dir := "testdata/registryfeatures"
	source, err := os.ReadFile(filepath.Join(dir, "probe.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "public static dead()")
	end := strings.Index(text[start:], "}") + start + 1
	coverage := &Reachability{Schema: 1, Workloads: []string{"feature-unit"}, Spans: []CoverageSpan{{File: "probe.ts", Start: start, End: end, Kind: "MethodDeclaration", Line: strings.Count(text[:start], "\n") + 1, SHA256: overrides.Fingerprint(text[start:end])}}}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"probe.ts"}, nil, coverage)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
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
	raw, err := os.ReadFile(filepath.Join(dir, "oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 6 {
		t.Fatal("missing oracle cases")
	}
	class := names.Get("probe.ts.Probe")
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("CLASS ltcl_features DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.")
	line("PRIVATE SECTION.")
	line("METHODS observations FOR TESTING.")
	line("METHODS trap FOR TESTING.")
	line("ENDCLASS.")
	line("CLASS ltcl_features IMPLEMENTATION.")
	line("METHOD observations.")
	for _, name := range []string{"raw", "needle", "expected", "actual"} {
		line("DATA %s TYPE string.", name)
	}
	line("DATA n TYPE f.")
	line("DATA flag TYPE abap_bool.")
	line("DATA ch TYPE c LENGTH 1.")
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
		line("CALL METHOD %s=>%s EXPORTING %s = raw %s = needle %s = n %s = flag RECEIVING result = actual.", class, names.Get("member.run"), names.Get("param.raw"), names.Get("param.needle"), names.Get("param.n"), names.Get("param.flag"))
		line("cl_abap_unit_assert=>assert_equals( act = actual exp = expected msg = `feature case %d` ).", i+1)
	}
	line("ENDMETHOD.")
	line("METHOD trap.")
	line("DATA actual TYPE f.")
	line("DATA caught TYPE abap_bool.")
	line("TRY.")
	line("actual = %s=>%s( ).", class, names.Get("member.dead"))
	line("CATCH %s INTO DATA(failure).", names.Get("exception.unexecuted"))
	line("caught = abap_true.")
	line("cl_abap_unit_assert=>assert_equals( act = failure->source_location exp = `probe.ts:%d` ).", coverage.Spans[0].Line)
	line("ENDTRY.")
	line("cl_abap_unit_assert=>assert_equals( act = caught exp = abap_true ).")
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
	t.Logf("%d emitted files, %d original observations, located trap", len(files), len(cases))
}
