package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/internal/gracecheck"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func TestEmitRegistryJSON(t *testing.T) {
	prog := lowerRegistryJSON(t)
	if os.Getenv("ABAPITI_GRACECHECK") == "1" {
		gracecheck.Check(t, prog)
	}
	config := os.Getenv("REGISTRY_JSON_CONFIG")
	if config == "" {
		t.Skip("set REGISTRY_JSON_CONFIG and REGISTRY_JSON_RESOLVED to original JSON inputs")
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	class, dynamic := names.Get("json.ts.JSONProbe"), names.Get("runtime.dynamic")
	var inputs []string
	for _, path := range []string{config, os.Getenv("REGISTRY_JSON_RESOLVED")} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, string(raw))
	}
	inputs = append(inputs, `{"global":{"files":"/src/**/*.abap"},"syntax":{"version":"v702"},"rules":{"unknown_rule":false},"list":[0,"second"],"n":null}`, `{"a":"\"\\\/\b\f\n\r\t\u0041\u00e9\u03b1\ud83d\ude00","a": "last", "n":-1.25e+2,"empty":{},"array":[],"bool":true}`, `{"escaped":"\"\\\/\b\f\n\r\t\u0041\u00e9\u03b1\ud83d\ude00", "nested":[null,false,true,0,1.25,-125,{}]}`)
	configRaw, err := os.ReadFile("testdata/registryfeatures/config-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var configCases []struct{ Input, Expected string }
	if err := json.Unmarshal(configRaw, &configCases); err != nil {
		t.Fatal(err)
	}
	if len(configCases) != 2 {
		t.Fatal("missing original config observations")
	}
	for _, test := range configCases {
		inputs = append(inputs, test.Input)
	}
	for i, input := range inputs {
		var value any
		if err := json.Unmarshal([]byte(input), &value); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		line := func(f string, args ...any) { fmt.Fprintf(&b, f+"\n", args...) }
		driver := fmt.Sprintf("z_json_case_%03d", i)
		files[driver+".clas.abap"] = "CLASS " + driver + " DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + driver + " IMPLEMENTATION.\nENDCLASS.\n"
		line("CLASS ltcl_json DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.")
		line("PRIVATE SECTION.\nMETHODS graph FOR TESTING.\nENDCLASS.\nCLASS ltcl_json IMPLEMENTATION.\nMETHOD graph.")
		line("DATA text TYPE string.\nDATA ch TYPE c LENGTH 1.")
		for _, s := range abapStringBuild("text", "ch", input) {
			line("%s", s)
		}
		line("DATA input TYPE string.\ninput = text.")
		line("DATA root TYPE REF TO %s.", dynamic)
		line("root = %s=>%s( text ).", class, names.Get("member.parse"))
		serial := 0
		var check func(any, string)
		check = func(value any, parent string) {
			line("cl_abap_unit_assert=>assert_bound( %s ).", parent)
			switch v := value.(type) {
			case nil:
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 8 ).", parent)
			case bool:
				flag := "abap_false"
				if v {
					flag = "abap_true"
				}
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 7 ).", parent)
				line("cl_abap_unit_assert=>assert_equals( act = %s->bval exp = %s ).", parent, flag)
			case float64:
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 4 ).", parent)
				line("cl_abap_unit_assert=>assert_equals( act = %s->nval exp = CONV f( `%g` ) ).", parent, v)
			case string:
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 1 ).", parent)
				line("CLEAR text.")
				for _, s := range abapStringBuild("text", "ch", v) {
					line("%s", s)
				}
				line("cl_abap_unit_assert=>assert_equals( act = %s->sval exp = text ).", parent)
			case map[string]any:
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 5 ).", parent)
				line("cl_abap_unit_assert=>assert_equals( act = lines( %s->entries ) exp = %d ).", parent, len(v))
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					serial++
					local := fmt.Sprintf("node%d", serial)
					line("DATA %s TYPE REF TO %s.", local, dynamic)
					line("%s = %s->get( `%s` ).", local, parent, strings.ReplaceAll(k, "`", "``"))
					check(v[k], local)
				}
			case []any:
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 6 ).", parent)
				line("cl_abap_unit_assert=>assert_equals( act = lines( %s->items ) exp = %d ).", parent, len(v))
				for j, c := range v {
					serial++
					local := fmt.Sprintf("node%d", serial)
					line("DATA %s TYPE REF TO %s.", local, dynamic)
					line("READ TABLE %s->items INDEX %d INTO %s.", parent, j+1, local)
					check(c, local)
				}
			default:
				t.Fatalf("unexpected JSON leaf %T", value)
			}
		}
		check(value, "root")
		if i <= 1 || i >= 5 {
			line("DATA typed TYPE REF TO %s.", names.Get("json.ts.ConfigGraph"))
			line("typed = %s=>%s( input ).", class, names.Get("member.config"))
			line("cl_abap_unit_assert=>assert_bound( typed ).")
			line("root = typed->%s.", names.Get("builtin.materializedSource"))
			check(value, "root")
			if i >= 5 {
				line("DATA actual TYPE string.")
				line("actual = %s=>%s( input ).", class, names.Get("member.defaults"))
				line("CLEAR text.")
				for _, s := range abapStringBuild("text", "ch", configCases[i-5].Expected) {
					line("%s", s)
				}
				line("cl_abap_unit_assert=>assert_equals( act = actual exp = text ).")
			}
		}
		line("ENDMETHOD.\nENDCLASS.")
		files[driver+".clas.testclasses.abap"] = b.String()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CLASS ltcl_contract DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS rejects FOR TESTING.\nENDCLASS.\nCLASS ltcl_contract IMPLEMENTATION.\nMETHOD rejects.\nDATA caught TYPE abap_bool.\nDATA value TYPE REF TO %s.\n", dynamic)
	fmt.Fprintf(&b, "DATA failure TYPE REF TO %s.\nDATA message TYPE string.\n", names.Get("exception.RegistryJSONSubsetError"))
	for _, input := range []string{``, `{"x":1,}`, `[1,]`, `{x:1}`, `{'x':1}`, `/*comment*/{}`, `{"x":NaN}`, `{"x":Infinity}`, `{"x":0x10}`, `{"x":01}`, `{"x":+1}`, `{"x":.5}`, `{"x":1.}`, `{"x":"\x41"}`, `{"x":true} extra`, `{"x":"unterminated}`, `[1 2]`} {
		fmt.Fprintf(&b, "CLEAR caught.\nTRY.\nvalue = %s=>%s( `%s` ).\nCATCH %s INTO failure.\ncaught = abap_true.\nmessage = failure->get_text( ).\ncl_abap_unit_assert=>assert_true( xsdbool( strlen( message ) > 0 ) ).\nENDTRY.\ncl_abap_unit_assert=>assert_true( caught ).\n", class, names.Get("member.parse"), input, names.Get("exception.RegistryJSONSubsetError"))
	}
	fmt.Fprintf(&b, "DATA(probe) = NEW %s( ).\ncl_abap_unit_assert=>assert_true( xsdbool( probe IS INSTANCE OF %s ) ).\nENDMETHOD.\nENDCLASS.\n", class, class)
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
	t.Logf("%d full JSON graphs, 17 rejection cases, %d emitted files", len(inputs), len(files))
}

func lowerRegistryJSON(t *testing.T) *hir.Program {
	t.Helper()
	dir := t.TempDir()
	source, err := os.ReadFile("testdata/registryfeatures/json.ts")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "json.ts"), source, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true},"files":["json.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := overrides.New(overrides.Entry{ID: "json-fixture", Key: overrides.Key{File: "json.ts", Symbol: "JSONProbe.parse", Kind: "KindMethodDeclaration"}, SHA256: "4d627fb0829356bf92903ad6dd03bba444f77d7c57560f3c162f7a1bd23a7db9", Rationale: "strict JSON external adapter differential", Method: func() *hir.Method {
		return &hir.Method{Name: "parse", Static: true, Result: hir.T(hir.Dynamic), Params: []hir.Param{{Name: "text", Type: hir.T(hir.String)}}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "json.parseSubset", Type: hir.T(hir.Dynamic), X: hir.V("text", hir.T(hir.String))}})}
	}}, overrides.Entry{ID: "typed-json-fixture", Key: overrides.Key{File: "json.ts", Symbol: "JSONProbe.config", Kind: "KindMethodDeclaration"}, SHA256: "64d64c5d5ae6d288faa6c7732fd89638a55f9cd3725a91f59dcc25aa2055fdad", Rationale: "same typed adapter projection as Config constructor", Method: func() *hir.Method {
		return &hir.Method{Name: "config", Static: true, Result: hir.Ref("json.ts.ConfigGraph"), Params: []hir.Param{{Name: "text", Type: hir.T(hir.String)}}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: overrides.StrictJSONProjection(hir.V("text", hir.T(hir.String)), hir.Ref("json.ts.ConfigGraph"))})}
	}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"json.ts"}, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs, hir.Dump(prog))
	}
	return prog
}
