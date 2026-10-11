package golang

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestXMLParsedArrayNativeView(t *testing.T) {
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "parse", Static: true, Result: hir.T(hir.Dynamic), Params: []hir.Param{{Name: "text", Type: hir.T(hir.String)}}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "xml.parseSubset", Type: hir.T(hir.Dynamic), X: hir.V("text", hir.T(hir.String))}})}}}}}
	main := `d:=parseXML(str("<root><x>first</x><x>second</x></root>")).get(str("root")).get(str("x"))
 a:=castRef[*array[any]](dynRef(d));b:=castRef[*array[any]](dynRef(d))
 if a!=b || !dynEqual(box(a),d) {panic("lost parsed-array identity")}
 a.put(0,box(str("changed")));b.push(box(str("third")))
 if dynString(d.get(str("0")))!=str("changed") || dynNumber(d.get(str("length")))!=3 {panic("lost live storage")}
 fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
