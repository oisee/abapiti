package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	gohir "github.com/oisee/abapiti/hir/golang"
)

func TestAnonymousObjectLiterals(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
   take(x: {a: string, b: number}): string { return x.a; }
   run(): string {
     const x = {a: "first", b: 1};
     const y = {b: 2, a: "second"};
     return this.take({a: x.a + y.a, b: 3});
   }
   inferred() { return {text: "result", count: 4}; }
   optional(): {label: string, extra?: number} { return {label: "ok"}; }
   readonly(): {readonly name: string} { return {name: "fixed"}; }
   nested() { return {child: {value: 1}}; }
   empty() { return {}; }
   union(flag: boolean): {title: string} | {size: number} {
     if (flag) { return {title: "named"}; }
     return {size: 2};
   }
   other() { return {a: "different", b: "type"}; }
   record(x?: {[name: string]: number}): number {
     let total = 0;
     for (const key in x || {}) { total += 1; }
     return total;
   }
 }
 `}, []string{"probe.ts"})
	var pairs int
	var readonly bool
	for _, c := range prog.Classes {
		for _, f := range c.Fields {
			if f.Name == "name" && f.Readonly {
				readonly = true
			}
		}
		if strings.HasPrefix(c.Name, "shape.") && len(c.Fields) == 2 {
			foundA, foundB := false, false
			for _, f := range c.Fields {
				foundA = foundA || f.Name == "a" && f.Type.Kind == hir.String
				foundB = foundB || f.Name == "b" && f.Type.Kind == hir.Number
			}
			if foundA && foundB {
				pairs++
			}
		}
	}
	if !readonly {
		t.Fatal("anonymous readonly field lost its metadata")
	}
	if pairs != 1 {
		t.Fatalf("got %d copies of the a/b shape, want one", pairs)
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	if _, err := gohir.Emit(prog); err != nil {
		t.Fatal(err)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
}
