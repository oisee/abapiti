package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettledTransitiveDispatch(t *testing.T) {
	p, ds := asyncProbe(t, `interface Worker {work(n:number):Promise<number>}
 class A implements Worker {async work(n:number):Promise<number>{return n+1;}}
 class B implements Worker {async work(n:number):Promise<number>{return await new A().work(n+1);}}
 async function invoke(w:Worker):Promise<number>{return await w.work(2);}
 export class Probe {static async run():Promise<number>{return await invoke(new B());}}`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()))")
	if strings.TrimSpace(out) != "4" {
		t.Fatal(out)
	}
}

func TestSettledTransitivePendingImplementer(t *testing.T) {
	_, ds := asyncProbe(t, `interface Worker {work():Promise<number>}
 class Good implements Worker {async work():Promise<number>{return 1;}}
 class Pending implements Worker {async work():Promise<number>{return await new Promise<number>(()=>{});}}
 export class Probe {static async run(w:Worker):Promise<number>{return await w.work();}}`)
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "Pending.work") {
			return
		}
	}
	t.Fatal(ds)
}

func TestSettledTransitiveRecursion(t *testing.T) {
	p, ds := asyncProbe(t, `async function a(n:number):Promise<number>{if(n===0){return 7;}return await b(n-1);}
 async function b(n:number):Promise<number>{return await a(n);}
 export class Probe {static async run():Promise<number>{return await a(3);}}`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()))")
	if strings.TrimSpace(out) != "7" {
		t.Fatal(out)
	}
	_, ds = asyncProbe(t, `async function a(n:number):Promise<number>{if(n===0){return await new Promise<number>(()=>{});}return await b(n-1);}
 async function b(n:number):Promise<number>{return await a(n);}
 export class Probe {static async run():Promise<number>{return await b(3);}}`)
	if !hasBlocking(ds) {
		t.Fatal("pending SCC certified", ds)
	}
}

func TestSettledTransitiveFluentReceiver(t *testing.T) {
	p, ds := asyncProbe(t, `class Registry {addFile(n:number):Registry{return this;}async parseAsync():Promise<number>{return 9;}}
 class Pending extends Registry {async parseAsync():Promise<number>{return await new Promise<number>(()=>{});}}
 export class Probe {static async run():Promise<number>{const reg=new Registry();const alias=reg;return await alias.addFile(1).parseAsync();}}`)
	// The unused Pending body is independently refused, but must not poison the
	// statically pinned Registry await. Remove it for executable HIR validation.
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Loc, "input.ts:3:") {
			t.Fatal(ds)
		}
	}
	p, ds = asyncProbe(t, `class Registry {addFile(n:number):Registry{return this;}async parseAsync():Promise<number>{return 9;}}
 export class Probe {static async run():Promise<number>{const reg=new Registry();const alias=reg;return await alias.addFile(1).parseAsync();}}`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()))")
	if strings.TrimSpace(out) != "9" {
		t.Fatal(out)
	}
}

func TestSettledTransitiveHiddenPendingSource(t *testing.T) {
	_, ds := asyncProbe(t, `function schedule():void{setTimeout(()=>{},0);} function helper():void{schedule();}
 async function value():Promise<number>{helper();return 1;}
 export class Probe {static async run():Promise<number>{return await value();}}`)
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "pending source") {
			return
		}
	}
	t.Fatal(ds)
}

func TestSettledTransitiveWorldRefusals(t *testing.T) {
	for name, setup := range map[string]string{
		"mutation": `const worker=new Worker(); worker.work=async()=>1;`,
		"erasure":  `const worker:any=new Worker();`,
	} {
		t.Run(name, func(t *testing.T) {
			_, ds := asyncProbe(t, `class Worker {async work():Promise<number>{return 1;}} `+setup+`
  export class Probe {static async run():Promise<number>{return await new Worker().work();}}`)
			for _, d := range ds {
				if d.Category == "unsupported-async" && (strings.Contains(d.Message, "mutates") || strings.Contains(d.Message, "unproven")) {
					return
				}
			}
			t.Fatal(ds)
		})
	}
}

func TestSettledTransitivePendingSCCInvalidatesEarlierMember(t *testing.T) {
	_, ds := asyncProbe(t, `async function a(n:number):Promise<number>{await b(n);return await new Promise<number>(()=>{});}
 async function b(n:number):Promise<number>{if(n>0){return await a(n-1);}return 1;}
 export class Probe {
 static async run():Promise<number>{return await a(1);}
 static async again():Promise<number>{return await b(1);}}`)
	found := false
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Loc, "input.ts:5:") {
			found = true
		}
	}
	// The later query of b must observe invalidation of the whole SCC.
	count := 0
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "already-settled") {
			count++
		}
	}
	if !found || count < 3 {
		t.Fatal(ds)
	}
}

func TestSettledTransitiveLoadedImplementations(t *testing.T) {
	for _, scenario := range []struct {
		name, provider, refusal string
		include                 bool
	}{
		{"settled", `class TestWorker {async work():Promise<number>{return 1;}}`, "", true},
		{"pending", `class TestWorker {async work():Promise<number>{return await new Promise<number>(()=>{});}}`, "TestWorker.work", true},
		{"outside", `class TestWorker {async work():Promise<number>{return 1;}}`, "outside the translated closure", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, source := range map[string]string{
				"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"es2020"},"files":["input.ts","test.ts"]}`,
				"input.ts":      `interface Worker {work():Promise<number>} export async function invoke(w:Worker):Promise<number>{return await w.work();}`,
				"test.ts":       scenario.provider,
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			program, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			files := []string{"input.ts"}
			if scenario.include {
				files = append(files, "test.ts")
			}
			_, ds, err := program.Lower(files)
			if err != nil {
				t.Fatal(err)
			}
			refusal := ""
			for _, d := range ds {
				if d.Category == "unsupported-async" && strings.Contains(d.Loc, "input.ts:") {
					refusal += d.Message
				}
			}
			if scenario.refusal == "" && refusal != "" || scenario.refusal != "" && !strings.Contains(refusal, scenario.refusal) {
				t.Fatal(ds)
			}
		})
	}
}

func TestSettledTransitiveVirtualRecursion(t *testing.T) {
	p, ds := asyncProbe(t, `class Worker {async work(n:number):Promise<number>{if(n===0){return 8;}return await this.work(n-1);}}
 export class Probe {static async run():Promise<number>{return await new Worker().work(2);}}`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()))")
	if strings.TrimSpace(out) != "8" {
		t.Fatal(out)
	}
}

func TestSettledTransitiveMissingImplementerRefused(t *testing.T) {
	_, ds := asyncProbe(t, `interface Worker {work():Promise<number>} class Good implements Worker {async work():Promise<number>{return 1;}}
 class Missing implements Worker {} export async function invoke(w:Worker):Promise<number>{return await w.work();}`)
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "Missing") {
			return
		}
	}
	t.Fatal(ds)
}

func TestSettledTransitiveInheritedPendingInitializer(t *testing.T) {
	_, ds := asyncProbe(t, `class Base {value=new Promise<number>(()=>{});} class Child extends Base {}
 async function value():Promise<number>{const child=new Child();return 1;}
 export class Probe {static async run():Promise<number>{return await value();}}`)
	for _, d := range ds {
		if d.Category == "unsupported-async" && strings.Contains(d.Message, "pending source") {
			return
		}
	}
	t.Fatal(ds)
}
