package rewrite

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestCopyPropagate(t *testing.T) {
	i := hir.T(hir.I32)
	v := func(name string) *hir.Expr { return hir.V(name, i) }
	decl := func(name string, x *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.VarDecl, Name: name, Type: i, X: x}
	}
	ret := func(x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Return, X: x} }
	set := func(name string, x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, X: v(name), Y: x} }
	seq := func(y *hir.Expr, stmts ...*hir.Stmt) *hir.Expr {
		return &hir.Expr{Kind: hir.Seq, Type: i, Stmt: hir.B(stmts...), Y: y}
	}
	shared := v("y")
	for _, c := range []struct {
		name    string
		body    *hir.Stmt
		removed int
		want    string
	}{
		{"reads in a Seq and its result", hir.B(decl("y", hir.L(i, 1)), ret(seq(v("x"), decl("x", v("y")), decl("z", v("x"))))), 2, ""},
		{"nil statements", hir.B(nil, decl("y", hir.L(i, 1)), nil, decl("x", v("y")), ret(v("x"))), 1, "return (y):i32"},
		{"source assigned later", hir.B(decl("y", hir.L(i, 1)), decl("x", v("y")), set("y", hir.L(i, 2)), ret(v("x"))), 0, "var x:i32 = (y):i32"},
		{"copy assigned later", hir.B(decl("y", hir.L(i, 1)), decl("x", v("y")), set("x", hir.L(i, 2)), ret(v("x"))), 0, "var x:i32 = (y):i32"},
		{"source shadowed", hir.B(decl("y", hir.L(i, 1)), decl("x", v("y")), hir.B(decl("y", hir.L(i, 2)), ret(v("x")))), 0, "var x:i32 = (y):i32"},
		{"node reachable twice", hir.B(decl("y", hir.L(i, 1)), decl("x", shared), ret(shared)), 0, "var x:i32 = (y):i32"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "m", Static: true, Result: i, Body: c.body}}}}}
			if got := CopyPropagate(p); got != c.removed {
				t.Fatalf("removed %d, want %d:\n%s", got, c.removed, hir.Dump(p))
			}
			if c.want == "" {
				// z = x becomes a copy of y as well; the Seq yields y.
				if x := c.body.List[1].X; len(x.Stmt.List) != 0 || x.Y.Name != "y" {
					t.Fatalf("Seq not propagated:\n%s", hir.Dump(p))
				}
			} else if dump := hir.Dump(p); !strings.Contains(dump, c.want) {
				t.Fatalf("no %q in\n%s", c.want, dump)
			}
		})
	}
}
