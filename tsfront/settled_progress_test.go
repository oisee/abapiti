package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestClosedProgressOptionalDrain(t *testing.T) {
	p, ds := asyncProbe(t, `
interface IProgress { tick(text:string):Promise<void>; }
class Trace { static events=""; static calls=0; }
class Progress implements IProgress {
 async tick(text:string):Promise<void> { await Promise.resolve(); Trace.events=Trace.events+text; }
}
class Rejected implements IProgress { tick(text:string):Promise<void> { return Promise.reject("rejected"); } }
class ProbeInput { constructor(public progress?:IProgress) {} }
function argument():string { Trace.calls=Trace.calls+1; return "tick"; }
async function parseAsync(input?:ProbeInput):Promise<void> { await input?.progress?.tick(argument()); Trace.events=Trace.events+"parsed"; }
export class Probe { static async run():Promise<string> {
 await parseAsync(); await parseAsync(new ProbeInput());
 await parseAsync(new ProbeInput(new Progress()));
 try { await parseAsync(new ProbeInput(new Rejected())); } catch(e) { return Trace.events+":"+Trace.calls+":"+(e as string); }
 return "lost rejection";
} }
`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()).String())")
	if strings.TrimSpace(out) != "parsedparsedtickparsed:2:rejected" {
		t.Fatal(out)
	}
	files, _, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, text := range files {
		if strings.Contains(text, "->rejection") && strings.Contains(text, " IS BOUND.") {
			found = true
		}
	}
	if !found {
		t.Fatal("missing guarded ABAP rejection drain")
	}
}

// Rebuild the same core with test-file providers included: no persisted fact
// may allow a newly added implementation to evade the proof.
func TestClosedProgressTestFileImplementers(t *testing.T) {
	for name, fixture := range map[string]struct{ source, refused string }{
		"empty":                   {},
		"async":                   {`class TestProgress implements IProgress { async tick(text:string):Promise<void> { await Promise.resolve(); } }`, ""},
		"sync":                    {`class TestProgress implements IProgress { tick(text:string):Promise<void> { return Promise.resolve(); } }`, ""},
		"inherited":               {`class TestProgress { async tick(text:string):Promise<void> {} } class Child extends TestProgress {}`, ""},
		"static_provider":         {`class TestProgress {static async tick(text:string):Promise<void> {}} parseAsync(TestProgress);`, ""},
		"static_pending":          {`class TestProgress {static async tick(text:string):Promise<void> {await pending();}} declare function pending():Promise<void>; parseAsync(TestProgress);`, "TestProgress"},
		"structural":              {`class TestProgress { async tick(text:string):Promise<void> {} }`, ""},
		"object":                  {`const testProgress:IProgress={tick:async (text:string):Promise<void> => { await Promise.resolve(); }};`, ""},
		"object_alias":            {`const tick=async (text:string):Promise<void> => {await Promise.resolve();}; const testProgress:IProgress={tick};`, ""},
		"object_method":           {`const testProgress:IProgress={async tick(text:string):Promise<void> {}};`, ""},
		"invalid_declared":        {`class TestProgress implements IProgress {tick(text:string):void {}}`, "TestProgress"},
		"missing_tick":            {`class TestProgress implements IProgress {}`, "TestProgress"},
		"pending":                 {`class TestProgress implements IProgress { tick(text:string):Promise<void> { return new Promise<void>(()=>{}); } }`, "TestProgress"},
		"await_pending":           {`declare function pending():Promise<void>; class TestProgress implements IProgress { async tick(text:string):Promise<void> { await pending(); } }`, "TestProgress"},
		"override":                {`class Good { async tick(text:string):Promise<void> {} } class TestProgress extends Good { async tick(text:string):Promise<void> { await pending(); } } declare function pending():Promise<void>;`, "TestProgress"},
		"object_pending":          {`const testProgress:IProgress={tick:(text:string):Promise<void> => new Promise<void>(()=>{})};`, "object literal"},
		"erase_to_any":            {`class TestProgress {async tick(text:string):Promise<void> {}} const p=new TestProgress(); const erased=p as any;`, "casts an unproven"},
		"cast":                    {`declare const external:any; const testProgress=external as IProgress;`, "casts an unproven"},
		"any_argument":            {`declare const external:any; parseAsync(external);`, "passes an unproven"},
		"external":                {`declare const external:IProgress; parseAsync(external);`, "external IProgress"},
		"nested_wrapper_cast":     {`interface Holder {payload:{provider:IProgress};} declare const external:any; const holder=external as Holder;`, "casts an unproven"},
		"nested_wrapper_argument": {`interface Holder {payload:{provider:IProgress};} function wrapped(holder:Holder):void {} declare const external:any; wrapped(external);`, "passes an unproven"},
		"any_wrapper":             {`interface Input {progress?:IProgress;} function wrapped(input?:Input):void {} declare const external:any; wrapped(external);`, "passes an unproven"},
		"any_field":               {`interface Input {progress?:IProgress;} function wrapped(input?:Input):void {} class TestInput {progress:any;} wrapped(new TestInput());`, "passes an unproven"},
		"external_wrapper":        {`interface Input {progress?:IProgress;} declare function external():Input; const input=external();`, "external provider"},
		"any_return":              {`declare const external:any; function make():IProgress { return external; }`, "returns an unproven"},
		"mutated_index":           {`class TestProgress { async tick(text:string):Promise<void> {} } const p=new TestProgress(); p["tick"]=(text:string)=>new Promise<void>(()=>{});`, "mutates tick"},
		"mutated":                 {`class TestProgress { async tick(text:string):Promise<void> {} } const p=new TestProgress(); p.tick=(text:string)=>new Promise<void>(()=>{});`, "mutates tick"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			sources := map[string]string{
				"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2020"},"files":["input.ts","test.ts"]}`,
				"input.ts":      `export interface IProgress {tick(text:string):Promise<void>;} export async function parseAsync(progress?:IProgress):Promise<void> {await progress?.tick("parse");}`,
				"test.ts":       `import {IProgress,parseAsync} from "./input"; ` + fixture.source,
			}
			for file, text := range sources {
				if err := os.WriteFile(filepath.Join(dir, file), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			loaded, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			_, ds, err := loaded.Lower([]string{"input.ts", "test.ts"})
			if err != nil {
				t.Fatal(err)
			}
			refusal := ""
			for _, d := range ds {
				if d.Category == "unsupported-async" && strings.Contains(d.Loc, "input.ts:") {
					refusal += d.Message
				}
			}
			if fixture.refused == "" && refusal != "" {
				t.Fatal(ds)
			}
			if fixture.refused != "" && !strings.Contains(refusal, fixture.refused) {
				t.Fatalf("want implementer %q in refusal, got %v", fixture.refused, ds)
			}
		})
	}
}

func TestOptionalPromiseWithoutProgressProofRefused(t *testing.T) {
	_, ds := asyncProbe(t, `class Other {static async tick():Promise<void> {}} export async function run():Promise<void> {await Other?.tick();}`)
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "closed-world") {
			return
		}
	}
	t.Fatal(ds)
}

func TestClosedProgressUnawaitedCallRefused(t *testing.T) {
	_, ds := asyncProbe(t, `interface IProgress {tick(text:string):Promise<void>;} class TestProgress {async tick(text:string):Promise<void> {await Promise.resolve();}} export function run(progress?:IProgress):void {progress?.tick("parse");}`)
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "microtask interleaving") {
			return
		}
	}
	t.Fatal(ds)
}

func TestClosedProgressOutsideLoweredFilesRefused(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2020"},"files":["input.ts","test.ts"]}`,
		"input.ts":      `interface IProgress {tick(text:string):Promise<void>;} async function parseAsync(progress?:IProgress):Promise<void> {await progress?.tick("parse");}`,
		"test.ts":       `class TestProgress implements IProgress {async tick(text:string):Promise<void> {}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, ds := lowerProbe(t, dir)
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "TestProgress") && strings.Contains(d.Message, "outside the translated closure") {
			return
		}
	}
	t.Fatal(ds)
}
