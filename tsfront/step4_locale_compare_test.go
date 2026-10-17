package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// gogen's frontend walks its input folders sorted by
// x.name.localeCompare(y.name). The names are the real file and directory
// names of the eight step4 input folders; Go must order them as Node's ICU
// root collation does.
func TestStep4LocaleCompareFileNamesNodeOracle(t *testing.T) {
	names, err := os.ReadFile(filepath.Join("testdata", "step4-locale", "names.json"))
	if err != nil {
		t.Fatal(err)
	}
	source := `const names: string[] = ` + string(names) + `;
const pairs: string[][] = [["a","A"],["A","a"],["Ab","aB"],["aB","Ab"],["zcl_a","ZCL_A"],["ZCL_A","zcl_b"],
 ["_","-"],["-","."],[".","/"],["/","#"],["#","0"],["9","a"],["Z","_"],["a_b","a.b"],["a-b","a.b"],["#a#b","a"],["x#y","x_y"],
 ["abc","ab"],["ab","abc"],["Ab","abc"],["abc","AB"],["a","a"],["","a"],["a.clas.abap","a.clas.testclasses.abap"],
 ["zcl_foo.clas.abap","zcl_foo.clas.xml"],["package.devc.xml","Package.devc.xml"],["a1","aa"],["a9","a_"],["1","A"],["aA","Aa"]];
class Probe {
 static run(): string {
  const sorted = [...names].reverse().sort((x, y) => x.localeCompare(y));
  const signs = pairs.map((p) => String(p[0].localeCompare(p[1])));
  return sorted.join("\n") + "\n" + signs.join(",");
 }
}`
	want := nodePlatformOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go and Node order differ:\nGo   %.400q\nNode %.400q", got, want)
	}
}

// Code units outside the verified collation domain are refused, not guessed.
func TestStep4LocaleCompareRefusesOutsideDomain(t *testing.T) {
	for _, other := range []string{"a b", "a~", "a$", "a+b", "ä", "a(1)", "a,b"} {
		source := `class Probe {static invalid(): number {return "zcl_a.clas.abap".localeCompare(` + fmt.Sprintf("%q", other) + `);}}`
		p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
		main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("localeCompare outside the domain escaped")};fmt.Println("refused")}();fmt.Println(%s())`, goEntry("probe.ts.Probe", "invalid"))
		if got := runGoHIR(t, p, main); got != "refused\n" {
			t.Fatalf("%q: got %q", other, got)
		}
	}
}
