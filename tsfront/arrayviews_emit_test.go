package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// The structures override maps one splice(1) to array.splice1_view. These
// probes use the same op on ordinary arrays and check, against the original
// JS observations, what a view must preserve: mutation of the truncated input,
// loops over views, element writes, views of views and the one-element case.
func TestEmitArrayViews(t *testing.T) {
	dir := "testdata/arrayviews"
	raw, err := os.ReadFile(filepath.Join(dir, "probe.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	strs := hir.T(hir.Array, hir.T(hir.String))
	var entries []overrides.Entry
	method := regexp.MustCompile(`(?m)^  public static (\w+)\(\)`)
	locs := method.FindAllStringSubmatchIndex(text, -1)
	for i, m := range locs {
		name := text[m[2]:m[3]]
		start := m[0] + 2
		end := strings.LastIndex(text, "}")
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		body := text[start:end]
		body = body[:strings.LastIndex(body, "}")+1]
		exprs := map[string]func() *hir.Expr{}
		for _, v := range []string{"s", "t"} {
			v := v
			if strings.Contains(body, v+".splice(1)") {
				exprs[v+".splice(1)"] = func() *hir.Expr {
					return &hir.Expr{Kind: hir.RuntimeOp, Op: "array.splice1_view", Type: strs, X: hir.V(v, strs), Args: []*hir.Expr{hir.L(hir.T(hir.I32), 1)}}
				}
			}
		}
		entries = append(entries, overrides.Entry{ID: "views-" + name, Key: overrides.Key{File: "probe.ts", Symbol: "Probe." + name, Kind: "KindMethodDeclaration"}, SHA256: overrides.Fingerprint(body), Rationale: "test: splice(1) through views", Expressions: exprs})
	}
	registry, err := overrides.New(entries...)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithOverrides([]string{"probe.ts"}, registry)
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
	class := names.Get("probe.ts.Probe")
	if !strings.Contains(files[class+".clas.abap"], "splice1_view") {
		t.Fatal("override did not apply")
	}
	var oracle map[string]string
	data, err := os.ReadFile(filepath.Join(dir, "oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &oracle); err != nil || len(oracle) != len(entries) {
		t.Fatal("oracle does not cover every probe")
	}
	keys := make([]string, 0, len(oracle))
	for k := range oracle {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("CLASS ltcl_views DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.")
	line("PRIVATE SECTION.")
	line("METHODS observations FOR TESTING.")
	line("ENDCLASS.")
	line("CLASS ltcl_views IMPLEMENTATION.")
	line("METHOD observations.")
	line("DATA actual TYPE string.")
	for _, k := range keys {
		line("actual = %s=>%s( ).", class, names.Get("member."+k))
		line("cl_abap_unit_assert=>assert_equals( act = actual exp = `%s` msg = `%s` ).", oracle[k], k)
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
}
