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
	"github.com/oisee/abapiti/internal/gracecheck"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func TestEmitRegistrySorts(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile("testdata/registryfeatures/sorts.ts")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "sorts.ts"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true},"files":["sorts.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "export class SortProbe")
	end := strings.LastIndex(text, "}") + 1
	item, file := hir.Ref("sorts.ts.SortItem"), hir.Ref("sorts.ts.SortFile")
	registry, err := overrides.New(overrides.Entry{ID: "fixture-stable-sorts", Key: overrides.Key{File: "sorts.ts", Symbol: "SortProbe", Kind: "KindClassDeclaration"}, SHA256: overrides.Fingerprint(text[start:end]), Rationale: "pure field/metadata/filename comparators, same production adapter builders", Expressions: map[string]func() *hir.Expr{
		"rules.sort((a, b) => a.getMetadata().key.localeCompare(b.getMetadata().key))": func() *hir.Expr {
			return overrides.StableProjectedSort(hir.V("rules", hir.T(hir.Array, item)), overrides.StringKeyComparator("string.compareRegistryKey", func(value *hir.Expr) *hir.Expr {
				return &hir.Expr{Kind: hir.FieldGet, Name: "key", Type: hir.T(hir.String), X: &hir.Expr{Kind: hir.VirtualCall, Name: "getMetadata", Type: hir.Ref("sorts.ts.SortMetadata"), X: value}}
			}))
		},
		"objects.sort((a, b) => a.key.localeCompare(b.key))": func() *hir.Expr {
			return overrides.StableProjectedSort(hir.V("objects", hir.T(hir.Array, item)), overrides.StringKeyComparator("string.compareObjectName", func(value *hir.Expr) *hir.Expr {
				return &hir.Expr{Kind: hir.FieldGet, Name: "key", Type: hir.T(hir.String), X: value}
			}))
		},
		"files.slice().sort((a, b) => {\n      const aValue = sequence.findIndex(s => a.getFilename().endsWith(s));\n      const bValue = sequence.findIndex(s => b.getFilename().endsWith(s));\n      return aValue - bValue;\n    })": func() *hir.Expr {
			return overrides.StableProjectedSort(&hir.Expr{Kind: hir.RuntimeOp, Op: "array.slice0", Type: hir.T(hir.Array, file), X: hir.V("files", hir.T(hir.Array, file))}, overrides.FileSequenceComparator)
		},
		"invalid.sort((a, b) => a.localeCompare(b))": func() *hir.Expr {
			return overrides.StableProjectedSort(hir.V("invalid", hir.T(hir.Array, hir.T(hir.String))), overrides.StringKeyComparator("string.compareRegistryKey", func(value *hir.Expr) *hir.Expr { return value }))
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"sorts.ts"}, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	if errors := hir.Verify(prog); len(errors) > 0 {
		t.Fatal(errors, hir.Dump(prog))
	}
	gracecheck.Check(t, prog)
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/registryfeatures/sorts-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle map[string]json.RawMessage
	if err = json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	class := names.Get("sorts.ts.SortProbe")
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("CLASS ltcl_sorts DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.")
	line("PRIVATE SECTION.")
	methods := []string{"ruleKeys", "objectNames", "fileSequence"}
	for _, method := range methods {
		line("METHODS %s FOR TESTING.", method)
	}
	line("METHODS ordering_domain FOR TESTING.")
	line("ENDCLASS.\nCLASS ltcl_sorts IMPLEMENTATION.")
	for _, method := range methods {
		var expected string
		if err = json.Unmarshal(oracle[method], &expected); err != nil {
			t.Fatal(err)
		}
		line("METHOD %s.", method)
		line("DATA expected TYPE string.\nDATA ch TYPE c LENGTH 1.\nDATA actual TYPE string.")
		for _, statement := range abapStringBuild("expected", "ch", expected) {
			line("%s", statement)
		}
		line("actual = %s=>%s( ).", class, names.Get("member."+method))
		line("cl_abap_unit_assert=>assert_equals( act = actual exp = expected ).")
		line("DATA(probe) = NEW %s( ).", class)
		line("cl_abap_unit_assert=>assert_true( xsdbool( probe IS INSTANCE OF %s ) ).\nENDMETHOD.", class)
	}
	var divergence struct {
		Outcome string
		Value   []string
	}
	if err = json.Unmarshal(oracle["ordering_domain"], &divergence); err != nil {
		t.Fatal(err)
	}
	if divergence.Outcome != "return" || len(divergence.Value) != 2 || divergence.Value[0] != "a" || divergence.Value[1] != "é" {
		t.Fatal("stale original ordering-domain divergence", divergence)
	}
	line("METHOD ordering_domain.\nDATA caught TYPE abap_bool.")
	line("TRY.\n%s=>%s( ).", class, names.Get("member.rejected"))
	line("CATCH %s INTO DATA(failure).", names.Get("exception.RegistryOrderingSubsetError"))
	line("caught = abap_true.\nDATA(message) = failure->get_text( ).")
	line("cl_abap_unit_assert=>assert_true( xsdbool( strlen( message ) > 0 ) ).\nENDTRY.")
	line("cl_abap_unit_assert=>assert_true( act = caught ).\nENDMETHOD.\nENDCLASS.")
	files[class+".clas.testclasses.abap"] = b.String()
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err = os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err = os.WriteFile(filepath.Join(out, name), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("%d emitted files", len(files))
}
