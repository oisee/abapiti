package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Rows built with a concrete class in an interface-typed field must reach a
// callback declared with the interface field type as a widening, not a
// narrowing between unrelated shapes (abaplint structure_errors.ts).
func TestRowInterfaceFieldCallbackView(t *testing.T) {
	p, ds := asyncProbe(t, `
interface IStructure { name():string; }
class If implements IStructure { name():string { return "if"; } }
class While implements IStructure { name():string { return "while"; } }
const cases = [
 {abap: "a", structure: new If(), n: 1},
 {abap: "b", structure: new If(), n: 2},
];
export class Probe { static run():string {
 let out = "";
 cases.forEach((c: {abap: string, structure: IStructure, n: number}) => {
  out = out + c.abap + c.structure.name() + c.n;
 });
 return out;
} }
`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	out := runGoHIR(t, p, "fmt.Println("+goEntry("input.ts.Probe", "run")+"())")
	if strings.TrimSpace(out) != "aif1bif2" {
		t.Fatal(out)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
