package golang

import (
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLiveOwnKeysRuntimeNodeOracle(t *testing.T) {
	script := `const rows=[{}, {ooName:undefined}, {foundQualified:false}, {ooType:"CLAS",ooName:"X"}, {ooName:"X",ooType:"CLAS"}];for(const row of rows) {const alias=row;const before=Object.keys(row).join(",");delete alias.ooName;alias.ooName="again";alias["10"]="ten";alias["2"]="two";alias["01"]="one";alias["4294967295"]="last";console.log(before+"|"+Object.keys(row).join(","));}`
	want, err := exec.Command("node", "-e", script).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	str := hir.T(hir.String)
	dyn := hir.T(hir.Dynamic)
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "keys", Static: true, Params: []hir.Param{{Name: "input", Type: dyn}}, Result: hir.T(hir.Array, str), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.keys", Type: hir.T(hir.Array, str), X: hir.V("input", dyn)}})}}}}}
	main := `for _,text:=range []string{"{}","{\"ooName\":null}","{\"foundQualified\":false}","{\"ooType\":\"CLAS\",\"ooName\":\"X\"}","{\"ooName\":\"X\",\"ooType\":\"CLAS\"}"} {
 d:=parseJSON(str(text));bag:=d.Value.(*orderedMap[jsString,*dynamic]);if text=="{\"ooName\":null}" {bag.set(str("ooName"),nil)}
 before:=dynamicKeys(d).join(present(str(",")));alias:=bag;alias.delete(str("ooName"));alias.set(str("ooName"),box(str("again")))
 for _,key:=range []string{"10","2","01","4294967295"} {alias.set(str(key),box(str(key)))}
 fmt.Println((before+str("|")+dynamicKeys(d).join(present(str(",")))).String())
 }`
	if got := execute(t, p, main); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
