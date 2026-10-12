package golang

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Deletion is owned by the property-storage layer. Exercise its ordering
// contract directly here: remove a key from the shared ordered own-key bag;
// a subsequent ordinary HIR field assignment must create it at the end.
func TestNativeJSONDeleteReaddOrderingNodeOracle(t *testing.T) {
	c := &hir.Class{Name: "C", Fields: []hir.Field{{Name: "z", Type: hir.T(hir.String)}, {Name: "a", Type: hir.T(hir.String)}}}
	self := &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}
	assign := func(key, value string) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Name: key, Type: hir.T(hir.String), X: self}, Y: hir.L(hir.T(hir.String), value)}
	}
	c.Ctor = &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B(assign("z", "first"), assign("a", "second"))}
	c.Methods = []*hir.Method{{Name: "write", Result: hir.T(hir.String), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "json.stringifySubset", Type: hir.T(hir.String), X: self}})}, {Name: "readd", Result: hir.T(hir.Void), Body: hir.B(assign("z", "last"))}}
	p := &hir.Program{Classes: []*hir.Class{c}}
	n := hir.NewNames()
	main := `v:=` + n.Get("new.C") + `();fmt.Println(v.` + n.Get("member.write") + `().String());v.ownKeys.delete(str("z"));fmt.Println(v.` + n.Get("member.write") + `().String());v.` + n.Get("member.readd") + `();fmt.Println(v.` + n.Get("member.write") + `().String());`
	out, err := exec.Command("node", "-e", `class C {z="first";a="second"}const v=new C();console.log(JSON.stringify(v));delete v.z;console.log(JSON.stringify(v));v.z="last";console.log(JSON.stringify(v));`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got := execute(t, p, main); got != string(out) {
		t.Fatalf("Go %q Node %q", got, out)
	}
	// Static fields and prototype methods are absent from the serialized bag.
	if strings.Contains(string(out), "readd") {
		t.Fatal(string(out))
	}
}
