package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func asyncProbe(t *testing.T, source string) (*hir.Program, []LowerDiagnostic) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2020"},"files":["input.ts"]}`, "input.ts": source} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return lowerProbe(t, dir)
}

func TestSettledAsyncDrain(t *testing.T) {
	source := `
class Trace { static order = ""; }
async function value(): Promise<number> { Trace.order = Trace.order + "v"; return 7; }
async function failing(): Promise<string> { throw "original"; }
async function nested(): Promise<number> { Trace.order = Trace.order + "a"; const n = await value(); Trace.order = Trace.order + "b"; return n; }
async function tick(text: string): Promise<void> { Trace.order = Trace.order + text; }
async function parseAsync(): Promise<string> {
 Trace.order = "";
 const n = await nested();
 await tick("p");
 try { await failing(); } catch(e) { return Trace.order + ":" + n + ":" + (e as string); }
 return "missed";
}
export class Probe { static async run(): Promise<string> { return await parseAsync(); } }
`
	p, ds := asyncProbe(t, source)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()).String())")
	if strings.TrimSpace(out) != "avbp:7:original" {
		t.Fatal(out)
	}
	files, _, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	all := ""
	for _, s := range files {
		all += s
	}
	for _, want := range []string{"->rejection", "RAISE EXCEPTION", "->source_value", "CATCH", "->value"} {
		if !strings.Contains(all, want) {
			t.Fatalf("ABAP missing %s", want)
		}
	}
}

func TestSettledResolveReject(t *testing.T) {
	p, ds := asyncProbe(t, `export class Probe {
 static async run(): Promise<string> {
 const p = Promise.resolve("value");
 try { await Promise.reject<string>("reason"); } catch(e) { return (await p) + ":" + (e as string); }
 return "missed";
 }
}`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()).String())")
	if strings.TrimSpace(out) != "value:reason" {
		t.Fatal(out)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

func TestPendingAsyncRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"constructor":   `return await new Promise<number>(resolve => { setTimeout(() => resolve(1), 0); });`,
		"parameter":     `return await p;`,
		"race":          `return await Promise.race([Promise.resolve(1)]);`,
		"thenable":      `return await ({then: (resolve: (n: number)=>void) => resolve(1)});`,
		"open_progress": `await progress.tick("parse"); return 1;`,
		"then":          `return await Promise.resolve(1).then(n => n + 1);`,
	} {
		t.Run(name, func(t *testing.T) {
			_, ds := asyncProbe(t, `interface Progress {tick(text:string):Promise<void>;} export class Probe {static async run(p:Promise<number>,progress:Progress):Promise<number>{`+body+`}}`)
			for _, d := range ds {
				if d.Category == "unsupported-async" && strings.Contains(d.Message, "settled") {
					return
				}
			}
			t.Fatalf("missing explicit settlement refusal: %v", ds)
		})
	}
}

func TestSettledRejectionTiming(t *testing.T) {
	p, ds := asyncProbe(t, `class Failure { static async fail():Promise<string> { throw "payload"; } }
 export class Probe { static async run():Promise<string> {
 let timing="";
 try { const p=Failure.fail(); timing="returned"; try { await p; } catch(e) { timing=timing+":"+(e as string); } }
 catch(e) { return "synchronous throw"; }
 try { await Promise.reject(); } catch(e) { if(e === undefined) { return timing+":undefined"; } }
 return "lost";
 } }`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()).String())")
	if strings.TrimSpace(out) != "returned:payload:undefined" {
		t.Fatal(out)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

func TestSettledProgressTick(t *testing.T) {
	p, ds := asyncProbe(t, `class Progress { static events=""; public async tick(text:string):Promise<void> { Progress.events=Progress.events+text; } }
 export class Probe { static async parseAsync():Promise<string> {
 await new Progress().tick("lex");
 Progress.events=Progress.events+"parse";
 await new Progress().tick("defs");
 return Progress.events;
 } }`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "parseAsync")+"()).String())")
	if strings.TrimSpace(out) != "lexparsedefs" {
		t.Fatal(out)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

func TestObservableMicrotasksRefused(t *testing.T) {
	_, ds := asyncProbe(t, `class Trace { static value=""; }
 async function effect():Promise<void> { await Promise.resolve(); Trace.value="after"; }
 export class Probe { static run():string { effect(); Trace.value="caller"; return Trace.value; } }`)
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "microtask interleaving") {
			return
		}
	}
	t.Fatal(ds)
}

func TestSettledVoidExpression(t *testing.T) {
	p, ds := asyncProbe(t, `class Trace { static value=""; } function effect():void { Trace.value="effect"; }
 async function tick():Promise<void> { return effect(); }
 export class Probe { static async run():Promise<string> { await tick(); return Trace.value; } }`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()).String())")
	if strings.TrimSpace(out) != "effect" {
		t.Fatal(out)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

func TestSettledAdoptionIdentity(t *testing.T) {
	p, ds := asyncProbe(t, `const original = Promise.resolve(1);
 async function adopt():Promise<number> { return original; }
 export class Probe { static run():boolean { const adopted=adopt(); return adopted !== original && Promise.resolve(original) === original; } }`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println("+goEntry("input.ts.Probe", "run")+"())")
	if strings.TrimSpace(out) != "true" {
		t.Fatal(out)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
