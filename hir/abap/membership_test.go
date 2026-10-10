package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A static integer set built from constants and only asked has() becomes a
// comparison chain; a set that is also read otherwise stays a set.
func TestConstantSetHas(t *testing.T) {
	setT := hir.T(hir.OrderedSet, i32)
	build := func(field string, vs ...*hir.Expr) []*hir.Stmt {
		tmp := "c_" + field
		out := []*hir.Stmt{{Kind: hir.VarDecl, Name: tmp, Type: setT, X: newObj(setT)}}
		for _, v := range vs {
			out = append(out, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "set.add", Type: setT, X: local(tmp, setT), Args: []*hir.Expr{v}}})
		}
		return append(out, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "Mod", Name: field, Type: setT}, Y: local(tmp, setT)})
	}
	static := func(n string, ty hir.Type) *hir.Expr { return &hir.Expr{Kind: hir.StaticGet, Owner: "Mod", Name: n, Type: ty} }
	has := func(field string, x *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.RuntimeOp, Op: "set.has", Type: boolean, X: static(field, setT), Args: []*hir.Expr{x}}
	}
	body := []*hir.Stmt{{Kind: hir.Assign, X: static("space", i32), Y: lit(32)}}
	body = append(body, build("splits", static("space", i32), lit(46), lit(10))...)
	body = append(body, build("shared", lit(1), lit(2))...)
	init := method("class_constructor", hir.T(hir.Void), hir.B(body...))
	init.Static = true
	c := local("c", i32)
	isSplit := method("isSplit", boolean, hir.B(ret(has("splits", c))))
	isSplit.Static, isSplit.Params = true, []hir.Param{{Name: "c", Type: i32}}
	isShared := method("isShared", boolean, hir.B(ret(has("shared", c))))
	isShared.Static, isShared.Params = true, []hir.Param{{Name: "c", Type: i32}}
	// shared is also handed out, so it may change: it must stay a set.
	leak := method("leak", setT, hir.B(ret(static("shared", setT))))
	leak.Static = true
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "Mod", Fields: []hir.Field{{Name: "space", Type: i32, Static: true}, {Name: "splits", Type: setT, Static: true}, {Name: "shared", Type: setT, Static: true}}, Methods: []*hir.Method{init, leak}},
		{Name: "Lexer", Methods: []*hir.Method{isSplit, isShared}},
	}}
	files, names, err := EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	lexer := files[names.Get("Lexer")+".clas.abap"]
	split := lexer[strings.Index(lexer, "METHOD "+names.Get("member.isSplit")):]
	split = split[:strings.Index(split, "ENDMETHOD.")]
	if !strings.Contains(split, " = 32\nOR ") || !strings.Contains(split, " = 46\nOR ") || !strings.Contains(split, " = 10\n).") {
		t.Fatal("constant set has() is not a comparison chain:\n" + split)
	}
	if strings.Contains(split, "->has(") || strings.Contains(split, names.Get("member.splits")) {
		t.Fatal("constant set is still read:\n" + split)
	}
	shared := lexer[strings.Index(lexer, "METHOD "+names.Get("member.isShared")):]
	shared = shared[:strings.Index(shared, "ENDMETHOD.")]
	if !strings.Contains(shared, "->has(") {
		t.Fatal("a set that is also read otherwise must stay a set:\n" + shared)
	}
}
