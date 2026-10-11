package golang

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestChaiThrowRuntimeBoundaries(t *testing.T) {
	thunk := hir.Type{Kind: hir.InterfaceRef, Name: "closure.thunk"}
	op := rt("chai.throwMatch", hir.V("action", thunk), boolean, hir.L(hir.T(hir.String), "boom"))
	p := &hir.Program{Interfaces: []*hir.Interface{{Name: thunk.Name, Methods: []*hir.Method{{Name: "call", Result: hir.T(hir.Void)}}}}, Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "run", Static: true, Params: []hir.Param{{Name: "action", Type: thunk}}, Result: boolean, Body: hir.B(ret(op))}}}}}
	main := `calls:=0;fmt.Println(chaiThrowMatch(func(){calls++;panic(payload[jsString]{Type:"string",Value:str("prefix boom suffix")})},str("boom")),calls);
 fmt.Println(chaiThrowMatch(func(){calls++},str("")),calls);
 func(){defer func(){if _,ok:=recover().(trap);!ok{panic("refusal swallowed")}}();chaiThrowMatch(func(){panic(newTrap("refusal"))},str(""))}();
 func(){defer func(){if recover()!="host fault"{panic("host fault swallowed")}}();chaiThrowMatch(func(){panic("host fault")},str(""))}();
 func(){defer func(){if _,ok:=recover().(trap);!ok{panic("unsupported payload must refuse")}}();chaiThrowMatch(func(){panic(payload[int32]{Type:"i32",Value:1})},str(""))}();`
	if got := execute(t, p, main); got != "true 1\nfalse 2\n" {
		t.Fatal(got)
	}
}
