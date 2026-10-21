package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/internal/abapfixture"
)

// testdata/extern-abap is also the OSGo smoke program
// (tools/extern-abap-osg-smoke.sh); here only emission is checked.
func TestEmitABAPInitializeExterns(t *testing.T) {
	project, err := filepath.Abs("testdata/extern-abap")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	emit := newEmitABAPCmd()
	emit.SetArgs([]string{"--tsconfig", filepath.Join(project, "tsconfig.json"), "--externs", filepath.Join(project, "externs.json"),
		"--initialize", filepath.Join(project, "initialization.json"), "-o", out, "main.ts", "state.ts", "host.ts"})
	if err = emit.Execute(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(out, "src")
	host, err := os.ReadFile("../../extern/hostio/abap/zcl_abapiti_hostio.clas.abap")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(src, "zcl_abapiti_hostio.clas.abap"), host, 0644); err != nil {
		t.Fatal(err)
	}
	abapfixture.CollectDir(t, "cmd/abapiti", src)
	main, err := os.ReadFile(filepath.Join(src, "zcl_abapiti_main.clas.abap"))
	if err != nil {
		t.Fatal(err)
	}
	// host.ts has no module state; state.ts initializes before main.ts.
	want := "CLASS zcl_abapiti_main DEFINITION PUBLIC FINAL CREATE PUBLIC.\n  PUBLIC SECTION.\n    CLASS-METHODS run.\nENDCLASS.\nCLASS zcl_abapiti_main IMPLEMENTATION.\n  METHOD run.\n" +
		"    z_state_ts_mod_eb5af01b30085e=>z_builtin_init_9016f21c683b49( ).\n    z_main_ts_modu_50921dc30544a5=>z_builtin_init_1003c5810dfa2d( ).\n  ENDMETHOD.\nENDCLASS.\n"
	if string(main) != want {
		t.Fatalf("main class:\n%s", main)
	}
	names, err := os.ReadFile(filepath.Join(out, "evidence", "names.json"))
	if err != nil || !strings.Contains(string(names), `"ZCL_ABAPITI_MAIN"`) {
		t.Fatalf("names.json lacks the entry class: %v", err)
	}
	traps, err := os.ReadFile(filepath.Join(out, "evidence", "trap-sites.json"))
	if err != nil || strings.TrimSpace(string(traps)) != "null" {
		t.Fatalf("unexpected traps: %v %s", err, traps)
	}
	for _, call := range []string{"zcl_abapiti_hostio=>env( name = ", "zcl_abapiti_hostio=>list_entries( dir = ", "zcl_abapiti_hostio=>write_stdout( text = "} {
		found := false
		for _, file := range []string{"z_state_ts_mod_eb5af01b30085e.clas.abap", "z_main_ts_modu_50921dc30544a5.clas.abap"} {
			b, _ := os.ReadFile(filepath.Join(src, file))
			found = found || strings.Contains(string(b), call)
		}
		if !found {
			t.Errorf("missing host call %q", call)
		}
	}
	bad := filepath.Join(t.TempDir(), "order.json")
	if err = os.WriteFile(bad, []byte(`["other.ts"]`), 0600); err != nil {
		t.Fatal(err)
	}
	emit = newEmitABAPCmd()
	emit.SetArgs([]string{"--tsconfig", filepath.Join(project, "tsconfig.json"), "--externs", filepath.Join(project, "externs.json"),
		"--initialize", bad, "-o", t.TempDir(), "main.ts", "state.ts", "host.ts"})
	emit.SilenceUsage, emit.SilenceErrors = true, true
	if err = emit.Execute(); err == nil || !strings.Contains(err.Error(), "outside selected closure") {
		t.Fatalf("order outside the closure: %v", err)
	}
}

// A recursive discriminated union family has no ABAP ABI: its declaration is
// ownerless, so the strict TS-HA policy aborts with an unattributable refusal
// while --keep-going records it, traps the owning bodies and still emits.
func TestEmitABAPKeepGoingRecordsUnionDeclaration(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	dir := t.TempDir()
	probe := `type Type = {readonly k:'num'} | {readonly k:'list';readonly item:Type};
type Meta = {readonly arith?:boolean};
type Expr = Meta & ({readonly e:'num';readonly value:number;readonly type:Type} | {readonly e:'wrap';readonly x:Expr;readonly type:Type} | {readonly e:'flag';readonly value:boolean;readonly type?:Type} | {readonly e:'none'} | {readonly e:'call';readonly args:readonly {readonly value:Expr}[];readonly type:Type});
type TypedExpr = Expr & {readonly type:Type};
type ValueExpr = TypedExpr & {readonly arith?:boolean};
function wrap(x:ValueExpr):Expr {const args=[{value:x}];return {e:'call',args,type:{k:'num'}};}
class Probe {static run():string {const x:ValueExpr={e:'num',value:7,type:{k:'num'}};const y=wrap(x);return y.e==='call' && y.args[0].value.e==='num' ? ''+y.args[0].value.value : 'bad';}}`
	for name, text := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2022"},"files":["probe.ts"]}`,
		"probe.ts":      probe,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"--tsconfig", filepath.Join(dir, "tsconfig.json")}
	// Since car U1 union families emit (union.declarations is supported), so
	// the strict policy no longer aborts on the ownerless declaration; the
	// owning bodies with payload reads still trap whole under both policies.
	strict := newEmitABAPCmd()
	strict.SilenceErrors, strict.SilenceUsage = true, true
	strict.SetArgs(append(append([]string{}, args...), "-o", filepath.Join(dir, "strict"), "probe.ts"))
	if err := strict.Execute(); err != nil {
		t.Fatalf("a family program must emit under the strict policy since car U1, got %v", err)
	}
	strictSites, err := os.ReadFile(filepath.Join(dir, "strict", "evidence", "trap-sites.json"))
	if err != nil || !strings.Contains(string(strictSites), `"Kind": "body"`) || !strings.Contains(string(strictSites), `"Method": "run"`) {
		t.Fatalf("strict policy must still trap the owning bodies: %v\n%s", err, strictSites)
	}
	keep := newEmitABAPCmd()
	keep.SilenceErrors, keep.SilenceUsage = true, true
	keep.SetArgs(append(append([]string{"--keep-going"}, args...), "-o", filepath.Join(dir, "keep"), "probe.ts"))
	if err := keep.Execute(); err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join(dir, "keep", "evidence", "unattributable-refusals.json"))
	if err != nil || strings.Contains(string(rows), "union.declarations") {
		t.Fatalf("car U1 supports union.declarations, so keep-going must not record it: %v\n%s", err, rows)
	}
	sites, err := os.ReadFile(filepath.Join(dir, "keep", "evidence", "trap-sites.json"))
	if err != nil || !strings.Contains(string(sites), `"Kind": "body"`) || !strings.Contains(string(sites), `"Method": "run"`) {
		t.Fatalf("owning bodies must keep the whole-body trap policy: %v\n%s", err, sites)
	}
	stub := ""
	entries, err := os.ReadDir(filepath.Join(dir, "keep", "src"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		text, err := os.ReadFile(filepath.Join(dir, "keep", "src", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(text), "lowering trap:") {
			stub = string(text)
		}
	}
	if !strings.Contains(stub, "RAISE EXCEPTION") {
		t.Fatalf("the unemittable body must be a named stub that raises at runtime, found none among %d files", len(entries))
	}
}
