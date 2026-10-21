package golang

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestChaiDeepRuntimeNodeOracle(t *testing.T) {
	root := os.Getenv("ABAPITI_TESTS_UPSTREAM")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		root = filepath.Join(home, "dev", "abaplint")
	}
	chai := filepath.Join(root, "node_modules", "chai")
	if _, err := os.Stat(chai); err != nil {
		chai = filepath.Join(root, "packages", "core", "node_modules", "chai")
	}
	if _, err := os.Stat(chai); err != nil {
		t.Skip("pinned upstream Chai unavailable")
	}
	script := `const {expect}=require(process.argv[1]);
 function eq(a,b){try{expect(a).deep.equal(b);return true}catch(e){return false}}
 function inc(a,b){try{expect(a).deep.include(b);return true}catch(e){return false}}
 const a={},b={};a.self=a;b.self=b;
 console.log(eq(NaN,NaN),eq(0,-0),eq(Infinity,Infinity),eq(-Infinity,Infinity),eq(a,b));
 b.x=1;console.log(eq(a,b));
 const holes=new Array(2);console.log(eq(holes,[undefined,undefined]),inc(holes,undefined));holes[1]=undefined;console.log(inc(holes,undefined));
 const c={x:undefined};console.log(eq(c,{}),inc(c,{x:undefined}),inc({},{x:undefined}));
 function shallow(a,b){try{expect(a).contain(b);return true}catch(e){return false}}
 const item={x:1},nested=[1];
 console.log(shallow([NaN],NaN),shallow([0],-0),shallow(new Array(1),undefined),shallow([undefined],undefined),shallow([item],item),shallow([item],{x:1}),shallow([nested],nested));`
	want, err := exec.Command("node", "-e", script, chai).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	dyn := hir.T(hir.Dynamic)
	boxed := rt("dynamic.of", hir.L(boolean, true), dyn)
	op := rt("chai.deepEqual", boxed, boolean, boxed)
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "run", Static: true, Result: boolean, Body: hir.B(ret(op))}}}}}
	main := `z:=float64(0);nan:=z/z;inf:=1/z;negzero:=-z;
 a,b:=parseJSON(str("{}")),parseJSON(str("{}"));a.put(str("self"),a);b.put(str("self"),b);
 fmt.Println(chaideepEqual(box(nan),box(nan)),chaideepEqual(box(z),box(negzero)),chaideepEqual(box(inf),box(inf)),chaideepEqual(box(-inf),box(inf)),chaideepEqual(a,b));
 b.put(str("x"),box(int32(1)));fmt.Println(chaideepEqual(a,b));
 holes:=&array[*dynamic]{Items:[]*dynamic{nil,nil},extra:&arrayExtra[*dynamic]{holes:map[int]bool{0:true,1:true}}};values:=&array[*dynamic]{Items:[]*dynamic{nil,nil}};
 fmt.Println(chaideepEqual(box(holes),box(values)),chaideepInclude(box(holes),nil)==1);holes.put(1,nil);fmt.Println(chaideepInclude(box(holes),nil)==1);
 c:=parseJSON(str("{}"));c.put(str("x"),nil);empty:=parseJSON(str("{}"));fmt.Println(chaideepEqual(c,empty),chaideepInclude(c,c)==1,chaideepInclude(empty,c)==1);
 item:=parseJSON(str("{\"x\":1}"));other:=parseJSON(str("{\"x\":1}"));nested:=parseJSON(str("[1]"));
 fmt.Println(chaiarrayInclude(box(&array[*dynamic]{Items:[]*dynamic{box(nan)}}),box(nan)),chaiarrayInclude(box(&array[*dynamic]{Items:[]*dynamic{box(z)}}),box(negzero)),chaiarrayInclude(box(&array[*dynamic]{Items:[]*dynamic{nil},extra:&arrayExtra[*dynamic]{holes:map[int]bool{0:true}}}),nil),chaiarrayInclude(box(&array[*dynamic]{Items:[]*dynamic{nil}}),nil),chaiarrayInclude(box(&array[*dynamic]{Items:[]*dynamic{item}}),item),chaiarrayInclude(box(&array[*dynamic]{Items:[]*dynamic{item}}),other),chaiarrayInclude(box(&array[*dynamic]{Items:[]*dynamic{nested}}),nested));
 func(){defer func(){if _,ok:=recover().(trap);!ok {panic("nonarray shallow include must refuse")}}();chaiarrayInclude(item,item)}();
 func(){defer func(){if _,ok:=recover().(trap);!ok {panic("unproven native keys must refuse")}}();chaideepEqual(box(&orderedMap[jsString,*dynamic]{}),empty)}();
 func(){defer func(){if _,ok:=recover().(trap);!ok {panic("inherited property must refuse")}}();q:=parseJSON(str("{}"));q.put(str("toString"),nil);chaideepInclude(empty,q)}();`
	if got := execute(t, p, main); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
