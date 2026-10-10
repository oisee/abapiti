package golang

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Refusals retain the originating TS method across Go runtime helpers and
// unwinding; they are never swallowed by an ABAP/JS exception catch.
func TestRuntimeRefusalLocations(t *testing.T) {
	str, dyn := hir.T(hir.String), hir.T(hir.Dynamic)
	c := &hir.Class{Name: "probe.ts.Probe"}
	add := func(name string, result hir.Type, x *hir.Expr) {
		m := method(name, result, ret(x))
		m.Static = true
		m.Source = "/disposable/closure/src/probe.ts:42:3"
		c.Methods = append(c.Methods, m)
	}
	add("json", dyn, &hir.Expr{Kind: hir.RuntimeOp, Op: "json.parseSubset", Type: dyn, X: hir.L(str, "{broken")})
	add("xml", dyn, &hir.Expr{Kind: hir.RuntimeOp, Op: "xml.parseSubset", Type: dyn, X: hir.L(str, "<!DOCTYPE a><a/>")})
	add("numeric", str, &hir.Expr{Kind: hir.RuntimeOp, Op: "number.toString", Type: str, X: hir.L(hir.T(hir.Number), 0.5)})
	m := method("regex", hir.T(hir.RegExp), ret(&hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.V("pattern", str)}}))
	m.Static = true
	m.Source = "/disposable/closure/src/probe.ts:42:3"
	m.Params = []hir.Param{{Name: "pattern", Type: str}}
	c.Methods = append(c.Methods, m)
	add("dollar", str, &hir.Expr{Kind: hir.RuntimeOp, Op: "string.replaceRegex", Type: str, X: hir.L(str, "abc"), Args: []*hir.Expr{{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, "a.c"), hir.L(str, "i")}}, hir.L(str, "$&")}})
	trapped := method("pruned", hir.T(hir.Void), &hir.Stmt{Kind: hir.Trap, Name: "unexecuted body"})
	trapped.Static = true
	trapped.Source = "/disposable/closure/src/probe.ts:42:3"
	c.Methods = append(c.Methods, trapped)
	factoryClass := &hir.Class{Name: "probe.ts.Required", Ctor: &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "required", Type: str}}, Body: hir.B()}}
	add("factory", hir.Ref(factoryClass.Name), &hir.Expr{Kind: hir.RuntimeOp, Op: "classvalue.new", Type: hir.Ref(factoryClass.Name), X: &hir.Expr{Kind: hir.ClassOf, Owner: factoryClass.Name, Type: hir.T(hir.ClassValue)}})
	main := ""
	for _, name := range []string{"json", "xml", "numeric", "regex", "dollar", "pruned", "factory"} {
		args := ""
		if name == "regex" {
			args = `str("(?=a)a")`
		}
		main += fmt.Sprintf(`func(){defer func(){x:=recover();if x==nil{panic("missing refusal")};fmt.Println(x)}();%s(%s)}();`, entry(c.Name, name), args)
	}
	out := execute(t, &hir.Program{Classes: []*hir.Class{c, factoryClass}}, main)
	if strings.Count(out, "src/probe.ts:42:3:") != 7 {
		t.Fatalf("missing source location: %s", out)
	}
	for _, reason := range []string{"strict JSON subset", "abapGit XML subset", "numeric", "unreviewed JavaScript regexp", "dollar substitutions", "unexecuted body", "concrete zero-argument constructor"} {
		if !strings.Contains(out, reason) {
			t.Errorf("missing %q: %s", reason, out)
		}
	}
}
