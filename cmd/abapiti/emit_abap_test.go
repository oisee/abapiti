package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	strict := newEmitABAPCmd()
	strict.SilenceErrors, strict.SilenceUsage = true, true
	strict.SetArgs(append(append([]string{}, args...), "-o", filepath.Join(dir, "strict"), "probe.ts"))
	err := strict.Execute()
	if err == nil || !strings.Contains(err.Error(), "unattributable ABAP test refusal") || !strings.Contains(err.Error(), "union.declarations") {
		t.Fatalf("strict policy must abort on the ownerless union declaration, got %v", err)
	}
	keep := newEmitABAPCmd()
	keep.SilenceErrors, keep.SilenceUsage = true, true
	keep.SetArgs(append(append([]string{"--keep-going"}, args...), "-o", filepath.Join(dir, "keep"), "probe.ts"))
	if err := keep.Execute(); err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join(dir, "keep", "evidence", "unattributable-refusals.json"))
	if err != nil || !strings.Contains(string(rows), `"operation": "union.declarations"`) || !strings.Contains(string(rows), `"node": `) {
		t.Fatalf("unattributable-refusals.json lacks the union declaration: %v\n%s", err, rows)
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
