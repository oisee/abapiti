package golang

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestLeafArrayStorageAndCovariantAlias(t *testing.T) {
	leaf := hir.Ref("ArrayLeaf")
	arr := hir.T(hir.Array, leaf)
	local := hir.V("items", arr)
	fresh := func() *hir.Expr { return &hir.Expr{Kind: hir.New, Type: leaf} }
	makeMethod := method("make", arr, &hir.Stmt{Kind: hir.Block, List: []*hir.Stmt{
		{Kind: hir.VarDecl, Name: "items", Type: arr, X: &hir.Expr{Kind: hir.New, Type: arr}},
		{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Type: i32, Op: "array.push", X: local, Args: []*hir.Expr{fresh()}}},
		ret(local),
	}})
	makeMethod.Static = true
	owner := &hir.Class{Name: "ArrayMaker", Methods: []*hir.Method{makeMethod}}
	cls := &hir.Class{Name: "ArrayLeaf", Fields: []hir.Field{{Name: "value", Type: i32}}}
	p := &hir.Program{Classes: []*hir.Class{cls, owner}}
	files, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	typed := "*array[" + hir.NewNames().Get("ref.ArrayLeaf") + "]"
	if !strings.Contains(files["hir.go"], typed) {
		t.Fatal("leaf array was not specialized")
	}
	main := fmt.Sprintf(`
 check:=func(ok bool){if !ok{panic("leaf array alias")}}
 a:=%s();b:=%s();old:=a.Items[0];check(a!=b && old!=b.Items[0])
 tail:=a.splice1_view(0);a.push(b.Items[0]);tail.push(old)
 check(len(a.Items)==1 && len(tail.Items)==2 && tail.Items[0]==old)
 copy:=tail.slice0();tail.put(0,b.Items[0]);check(copy.Items[0]==old)
 fmt.Println("ok")`, entry("ArrayMaker", "make"), entry("ArrayMaker", "make"))
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
	// A covariance boundary anywhere in the program must retain one common
	// container representation, including writes through the widened alias.
	root := hir.T(hir.Array, hir.Ref(hir.RootObject))
	bridge := method("widen", root, ret(&hir.Expr{Kind: hir.Narrow, Type: root, X: hir.V("items", arr)}))
	bridge.Params = []hir.Param{{Name: "items", Type: arr}}
	bridge.Static = true
	owner.Methods = append(owner.Methods, bridge)
	files, err = Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(files["hir.go"], typed) {
		t.Fatal("covariant array retained typed storage")
	}
	main = fmt.Sprintf(`a:=%s();b:=%s(a);if any(a)!=any(b){panic("widen copied")};old:=a.Items[0];b.push(old);b.put(0,nil);if len(a.Items)!=2 || a.Items[0]!=nil || a.Items[1]!=old{panic("widen alias")};fmt.Println("ok")`, entry("ArrayMaker", "make"), entry("ArrayMaker", "widen"))
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
	// Dynamic round trips must share the same ABI and preserve container identity.
	dyn := hir.T(hir.Dynamic)
	roundtrip := method("roundtrip", arr, ret(rt("dynamic.asRef", rt("dynamic.of", hir.V("items", arr), dyn), arr)))
	roundtrip.Static = true
	roundtrip.Params = []hir.Param{{Name: "items", Type: arr}}
	owner.Methods = []*hir.Method{makeMethod, roundtrip}
	files, err = Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(files["hir.go"], typed) {
		t.Fatal("dynamic array retained typed storage")
	}
	main = fmt.Sprintf(`a:=%s();b:=%s(a);if a!=b{panic("dynamic copied")};b.put(0,nil);if a.Items[0]!=nil{panic("dynamic alias")};fmt.Println("ok")`, entry("ArrayMaker", "make"), entry("ArrayMaker", "roundtrip"))
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
