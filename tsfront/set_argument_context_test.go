package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir/abap"
	"os/exec"
	"testing"
)

func TestSetArgumentContextNodeOracle(t *testing.T) {
	source := `class Position {getRow():number{return 1;}getCol():number{return 2;}}
 class Token {getStart():Position{return new Position();}}
 class SyntaxNode {getTokens():Token[]{return [new Token()];}}
 class Probe {
 static effects=0;
 static condition(value:boolean):boolean {Probe.effects++;return value;}
 static values():string[] {Probe.effects++;return ["b","a","b"];}
 static choose(value:boolean):string {
  const result=new Set(Probe.condition(value)?Probe.values().map(t=>t.toUpperCase()):[]);
  const result2: Set<string>=new Set(Probe.condition(!value)?[]:Probe.values().map(t=>t.toUpperCase()));
  result.add("c");
  return Array.from(result).join(",")+":"+Array.from(result2).join(",")+":"+Probe.effects;
 }
 static run():string {
 const asNode: SyntaxNode | undefined = Probe.effects===0?new SyntaxNode():undefined;
 const tokens = new Set(asNode ? asNode.getTokens().map(t => ` + "`" + `${t.getStart().getRow()}:${t.getStart().getCol()}` + "`" + `) : []);
 return tokens.size+":"+Probe.choose(true)+"|"+Probe.choose(false);
 }
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err = abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
