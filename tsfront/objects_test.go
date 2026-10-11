package tsfront

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestObjectsRecordAssignAndCallbackReturns(t *testing.T) {
	const source = `export class Item {
 constructor(public row: number, public col: number) {}
}
export class Probe {
 static run(): string {
  const target: {[key:string]: string} = {};
  target["keep"] = "yes";
  const source: {[key:string]: string} = {};
  source["keep"] = "overwritten";
  source["added"] = "new";
  const assigned = Object.assign(target, source);
  assigned["alias"] = "same";
  Object.assign(target, target);
	  const absent: {[key:string]: string} | undefined = undefined;
	  Object.assign(target, absent);
  const items = [new Item(2, 1), new Item(1, 3), new Item(1, 2)];
  items.sort((a,b) => {
   const rowDiff = b.row - a.row;
   if (rowDiff !== 0) { return rowDiff; }
   return b.col - a.col;
  });
  const limit = 2;
  const chosen = items.filter(x => {
   if (x.row === limit) { return false; }
   return true;
  });
  const immediate = (() => {
   if (chosen.length === limit) { return "ok"; }
   return "bad";
  })();
  return target["keep"] + ":" + target["added"] + ":" + target["alias"] + ":" + chosen[0].col + ":" + immediate;
 }
}`
	prog, ds, err := integerProbe(t, source).LowerWithOptions([]string{"probe.ts"}, LowerOptions{AssumeOnlyIntegerCalculations: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if strings.TrimSpace(got) != "overwritten:new:same:3:ok" {
		t.Fatal(got)
	}
}

func TestObjectsOptionalDynamicPrimitive(t *testing.T) {
	const source = `export class Probe {
 static value(flag: boolean): string {
  const raw: any = "present";
  const wrapped = flag ? raw : undefined;
  const typed: string | undefined = wrapped;
  return typed === undefined ? "absent" : typed;
 }
}`
	prog, ds, err := integerProbe(t, source).LowerWithOptions([]string{"probe.ts"}, LowerOptions{})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	entry := goEntry("probe.ts.Probe", "value")
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s(true), %s(false))", entry, entry))
	if strings.TrimSpace(got) != "present absent" {
		t.Fatal(got)
	}
}

func TestObjectsAssignRejectsMapEntries(t *testing.T) {
	p := integerProbe(t, `export class Probe { run() {
 const target: {[key:string]: string} = {};
 return Object.assign(target, new Map<string,string>());
} }`)
	_, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range ds {
		found = found || strings.Contains(d.Message, "Object.assign supports matching string-keyed records only")
	}
	if !found {
		t.Fatal(ds)
	}
}

func TestObjectsAssignArgumentOrder(t *testing.T) {
	const source = `export class Probe {
 private static calls = 0;
 static effect(source: {[key:string]:string}): {[key:string]:string} {
  this.calls = this.calls + 1;
  source["value"] = "after";
  return source;
 }
 static callsMade(): number { return this.calls; }
 static absentTarget(target: {[key:string]:string} | undefined) {
  const source: {[key:string]:string} = {};
  return Object.assign(target, this.effect(source));
 }
 static run(): string {
  const target: {[key:string]:string} = {};
  const source: {[key:string]:string} = {};
  source["value"] = "before";
  Object.assign(target, source, this.effect(source));
  return target["value"];
 }
}`
	prog, ds, err := integerProbe(t, source).LowerWithOptions([]string{"probe.ts"}, LowerOptions{AssumeOnlyIntegerCalculations: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	got := runGoHIR(t, prog, fmt.Sprintf(`fmt.Println(%s()); func() { defer func() { if recover() == nil { panic("absent target accepted") }; fmt.Println(%s()) }(); %s(nil) }()`, goEntry("probe.ts.Probe", "run"), goEntry("probe.ts.Probe", "callsMade"), goEntry("probe.ts.Probe", "absentTarget")))
	if strings.TrimSpace(got) != "after\n2" {
		t.Fatal(got)
	}
}
