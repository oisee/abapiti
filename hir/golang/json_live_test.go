package golang

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

// Parsed input proves order, not values: direct native writes must survive a
// second stringify, including nested writes and fields outside the projection.
func TestJSONParsedShapeLiveFields(t *testing.T) {
	p := configJSONFixture()
	c := &hir.Class{Name: "probe.Row", Fields: []hir.Field{{Name: "a", Type: hir.T(hir.String)}, {Name: "b", Type: hir.T(hir.String)}}}
	c.Ctor = &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: hir.T(hir.String)}, {Name: "b", Type: hir.T(hir.String)}}}
	c.Ctor.Body = hir.B()
	for _, f := range c.Fields {
		c.Ctor.Body.List = append(c.Ctor.Body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Name: f.Name, Type: f.Type, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}}, Y: hir.V(f.Name, f.Type)})
	}
	p.Classes = append(p.Classes, c)
	n := hir.NewNames()
	main := `d:=parseJSON(str("{\"b\":\"old\",\"extra\":1,\"a\":\"x\"}"));v:=&` + n.Get("struct."+c.Name) + `{source:d,` + n.Get("member.a") + `:str("x"),` + n.Get("member.b") + `:str("new")};fmt.Println(stringifyConfig(v).String());func(){defer func(){if _,ok:=recover().(trap);!ok {panic("stale projection boxing must refuse")}}();box(v);panic("boxed stale fields")}()`
	if got := execute(t, p, main); got != "{\"b\":\"new\",\"extra\":1,\"a\":\"x\"}\n" {
		t.Fatal(got)
	}
}
