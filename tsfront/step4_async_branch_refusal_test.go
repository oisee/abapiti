package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// observeParse is synchronous in step 4. Unsupported scheduling in the other
// arm must remain a located refusal without poisoning the synchronous call.
func TestStep4AsyncBranchRefusalNodeOracle(t *testing.T) {
	const source = `class Registry{value=7;parse():Registry{return this;}async parseAsync():Promise<Registry>{await Promise.resolve();return this;}}
 function observe(reg:Registry,args:unknown[],async:boolean):Registry|Promise<Registry>{
 const result=async?(reg as {parseAsync(...args:unknown[]):Promise<Registry>}).parseAsync(...args):(reg as {parse(...args:unknown[]):Registry}).parse(...args);
 if((result as Promise<Registry>)?.then)return (result as Promise<Registry>).then(value=>value);
 return result;
 }
 class Probe{static run():string{return String((observe(new Registry(),[],false) as Registry).value);}static blocked():void{observe(new Registry(),[],true);}}`
	want := nodePlatformOracle(t, source)
	dir := t.TempDir()
	for name, body := range map[string]string{"probe.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(ds) {
		t.Fatalf("lower: %v %v", err, ds)
	}
	if es := hir.Verify(prog); len(es) != 0 {
		t.Fatal(es)
	}
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	active := fmt.Sprintf(`_=fmt.Sprint;func(){defer func(){checkAsyncRefusal(recover())}();%s()}()`, goEntry("probe.ts.Probe", "blocked"))
	got = runGoHIR(t, prog, active, func(files map[string]string) {
		files["refusalcheck.go"] = `package main
import("fmt";"strings")
func checkAsyncRefusal(v any){if v==nil || !strings.Contains(fmt.Sprint(v),"probe.ts:"){panic("missing located async refusal")};fmt.Println("trapped")}
`
	})
	if got != "trapped\n" {
		t.Fatalf("async branch: %q", got)
	}
}
