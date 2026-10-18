package golang

import (
	"github.com/oisee/abapiti/hir"
	"os/exec"
	"testing"
)

func TestBoxedMapStorageIdentityAndNoncomparablePayload(t *testing.T) {
	typ := hir.T(hir.OrderedMap, hir.T(hir.Dynamic), hir.T(hir.Dynamic))
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "make", Static: true, Result: typ, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: typ}})}}}}}
	main := `
 check:=func(ok bool){if !ok {panic("boxed Map semantics")}}
 m:=&orderedMap[*dynamic,*dynamic]{}
 m.set(box(int32(1)),box(str("old")));m.set(box(int64(1)),box(str("wide")));m.set(box(float64(1)),box(str("new")))
 check(len(m.Entries)==1 && dynString(m.get(box(int32(1))).Value)==str("new"))
 type object struct {fields map[string]int;items []int}
 a:=&object{fields:map[string]int{"x":1},items:[]int{1}};b:=&object{fields:map[string]int{"x":1},items:[]int{1}}
 m.set(box(a),box(str("a")));m.set(box(b),box(str("b")));a.items=append(a.items,2)
 check(dynString(m.get(box(a)).Value)==str("a") && dynString(m.get(box(b)).Value)==str("b"))
 parsed:=parseJSON(str("{\"x\":[1]}"));array:=parsed.get(str("x"))
 m.set(parsed,box(str("parsed")));m.set(array,box(str("array")))
 check(m.has(box(parsed.Value)) && m.has(box(array.Value)))
 m.set(nil,nil);check(m.get(nil).Has && m.get(nil).Value==nil && !m.get(dynNull()).Has)
 m.set(dynNull(),box(str("null")));check(m.has(dynNull()))
 for _,value:=range []any{[]int{1},map[string]int{"x":1},func(){}} {
  func(){defer func(){if _,ok:=recover().(trap);!ok {panic("unchecked noncomparable key")}}();m.set(&dynamic{Tag:2,Value:value},nil)}()
 }
 check(m.has(box(a)));fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestBoxedMapZeroKeysNodeOracle(t *testing.T) {
	node := `const m=new Map();m.set(-0,"first").set(0,"second");console.log(Object.is([...m.keys()][0],0),m.size,m.get(-0),m.delete(0),m.has(-0));`
	want, err := exec.Command("node", "-e", node).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	typ := hir.T(hir.OrderedMap, hir.T(hir.Dynamic), hir.T(hir.Dynamic))
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "make", Static: true, Result: typ, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: typ}})}}}}}
	got := execute(t, p, `m:=&orderedMap[*dynamic,*dynamic]{};z:=float64(0);m.set(box(-z),box(str("first"))).set(box(z),box(str("second")));fmt.Println(1/dynNumber(m.keys().Items[0])>0,len(m.Entries),dynString(m.get(box(-z)).Value).String(),m.delete(box(z)),m.has(box(-z)))`)
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
