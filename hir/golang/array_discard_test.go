package golang

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestDiscardedSpliceAndUnshiftViews(t *testing.T) {
	main := `
 check:=func(ok bool){if !ok{panic("discard splice alias")}}
 equalItems:=func(a,b *array[int32]){check(len(a.Items)==len(b.Items));for i,v:=range a.Items{check(v==b.Items[i])}}
 for _,start:=range []int32{-9,-1,0,1,3,9} {
  for _,count:=range []int32{-3,0,1,2,9} {
   a:=&array[int32]{Items:[]int32{1,2,3}};alias:=a;reference:=a.slice0()
   reference.splice2(start,count);a.splice2_discard(start,count);equalItems(a,reference);check(alias==a)
   a.push(7);a.unshift(8);reference.push(7);reference.unshift(8);equalItems(a,reference)
   a.splice3_discard(start,count,9);reference.splice3(start,count,9);equalItems(a,reference)
  }
  a:=&array[int32]{Items:[]int32{1,2,3}};reference:=a.slice0()
  a.splice1_discard(start);reference.splice1(start);equalItems(a,reference)
  a=&array[int32]{Items:[]int32{1,2,3}};reference=a.slice0()
  a.splice1_view_discard(start);reference.splice1_view(start);equalItems(a,reference)
 }
 a:=&array[int32]{Items:[]int32{1,2,3,4,5}};tail:=a.splice1_view(2);snapshot:=tail.slice0()
 a.splice3_discard(0,1,7);a.unshift(8);a.push(9);equalItems(tail,snapshot)
 prefix:=a.slice0();tail.splice2_discard(0,2);tail.unshift(10);equalItems(a,prefix)
 fmt.Println("ok")`
	if got := execute(t, &hir.Program{}, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestDiscardedSliceEvaluationAndNilReceiver(t *testing.T) {
	arr := hir.T(hir.Array, i32)
	trace := &hir.Expr{Kind: hir.StaticGet, Owner: "DiscardProbe", Name: "trace", Type: str}
	appendTrace := func(label string) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, X: trace, Y: rt("string.concat", trace, str, hir.L(str, label))}
	}
	receiver := method("receiver", arr, &hir.Stmt{Kind: hir.Block, List: []*hir.Stmt{appendTrace("R"), ret(hir.V("items", arr))}})
	receiver.Static = true
	receiver.Params = []hir.Param{{Name: "items", Type: arr}}
	left := method("left", i32, &hir.Stmt{Kind: hir.Block, List: []*hir.Stmt{appendTrace("L"), ret(lit(0))}})
	left.Static = true
	right := method("right", i32, &hir.Stmt{Kind: hir.Block, List: []*hir.Stmt{appendTrace("U"), ret(lit(2))}})
	right.Static = true
	drop := method("drop", hir.T(hir.Void), run(rt("array.slice2", call(hir.DirectCall, nil, "DiscardProbe", "receiver", arr, hir.V("items", arr)), arr,
		call(hir.DirectCall, nil, "DiscardProbe", "left", i32), call(hir.DirectCall, nil, "DiscardProbe", "right", i32))))
	drop.Static = true
	drop.Params = []hir.Param{{Name: "items", Type: arr}}
	p := &hir.Program{Classes: []*hir.Class{{Name: "DiscardProbe", Fields: []hir.Field{{Name: "trace", Type: str, Static: true}}, Methods: []*hir.Method{receiver, left, right, drop}}}}
	traceName := hir.NewNames().Get("static.DiscardProbe.trace")
	main := fmt.Sprintf(`
 a:=&array[int32]{Items:[]int32{1,2,3}};%s(a)
 if %s!=str("RLU") || len(a.Items)!=3 {panic("discard evaluation")}
 func(){defer func(){if recover()==nil {panic("nil slice accepted")}}();%s(nil)}()
 if %s!=str("RLURLU"){panic("nil evaluation order")}
 fmt.Println("ok")`, entry("DiscardProbe", "drop"), traceName, entry("DiscardProbe", "drop"), traceName)
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
