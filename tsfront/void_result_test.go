package tsfront

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestVoidLocalFunctionEffects(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
  values: number[] = [];
  add(value: number): void { this.values.push(value); }
  collect(): number {
   const values = this.values;
   const arrow = (value: number): void => values.push(value);
   const call = (value: number): void => this.add(value);
   const block = function(value: number): void { return values.push(value); };
   const update = (): void => values[0] = values[0] + 1;
   const contextual: (value: number) => void = value => values.push(value);
   function nested(value: number): void { return values.push(value); }
   const relay = (value: number): void => nested(value);
   arrow(2); arrow(3); call(4); block(5); update(); contextual(6); relay(7);
   return values[0] + values[1] + values[2] + values[3] + values[4] + values[5];
  }
  again(): void { const arrow = (): void => this.add(99); arrow(); }
  static run(): number { return new Probe().collect(); }
 }
 `}, []string{"probe.ts"})
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			if !strings.HasPrefix(m.Name, "fn_") {
				continue
			}
			if m.Result.Kind != hir.Void {
				t.Fatalf("%s returns %s", m.Name, m.Result)
			}
			source := files[names.Get(c.Name)+".clas.abap"]
			for _, line := range strings.Split(source, "\n") {
				if strings.Contains(line, "METHODS "+names.Get("member."+m.Name)) && strings.Contains(line, "RETURNING") {
					t.Fatal(line)
				}
			}
		}
	}
	if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "28\n" {
		t.Fatalf("got %q", got)
	}
}

func TestVoidResultLocalAndThunk(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
  values: number[] = [];
  effect(): void { this.values.push(1); }
  collect(): boolean {
   const missing = this.effect();
   const values = this.values;
   const tasks: (() => void)[] = [() => values.push(2), () => { return values.push(3); }];
   for (const task of tasks) { task(); }
   return missing === undefined && values.length === 3 && values[0] === 1 && values[1] === 2 && values[2] === 3;
  }
  static run(): boolean { return new Probe().collect(); }
 }
 `}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "true\n" {
		t.Fatalf("got %q", got)
	}
}

func TestVoidArrowPreludeRunsOnInvocation(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe { static run(): boolean {
  const values = [1, 2];
  const results: number[] = [];
  const append = (): void => results.push(...values.map(value => value + 1));
  const before = results.length;
  append(); append();
  return before === 0 && results.length === 4 && results[0] === 2 && results[3] === 3;
 } }
 `}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "true\n" {
		t.Fatalf("got %q", got)
	}
}

func TestVoidLocalTypeFailureRetainsRoot(t *testing.T) {
	_, diags := lowerProbe(t, sourceProbe(t, `
 export class Probe { run(value: Date): void { const local = value; } }
 `))
	found := false
	for _, d := range diags {
		if d.Category == "unsupported-type" && strings.Contains(d.Message, "Date") {
			found = true
		}
		if strings.Contains(d.Message, "void local") {
			t.Fatalf("failed type mislabeled as void ABI: %s", d)
		}
	}
	if !found {
		t.Fatal("missing unsupported Date root diagnostic")
	}
}
