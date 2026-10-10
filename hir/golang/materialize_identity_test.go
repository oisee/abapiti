package golang

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestMaterializedNestedSourceIdentity(t *testing.T) {
	number := hir.T(hir.Number)
	shape := func(name string, fields []hir.Field) *hir.Class {
		cls := &hir.Class{Name: name, Fields: fields}
		var params []hir.Param
		var body []*hir.Stmt
		for _, f := range fields {
			params = append(params, hir.Param{Name: f.Name, Type: f.Type})
			body = append(body, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: f.Type, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}, Name: f.Name}, Y: hir.V(f.Name, f.Type)})
		}
		cls.Ctor = &hir.Method{Name: "constructor", Params: params, Result: hir.T(hir.Void), Body: &hir.Stmt{Kind: hir.Block, List: body}}
		return cls
	}
	leaf := shape("JSONLeaf", []hir.Field{{Name: "value", Type: number}})
	top := shape("JSONTop", []hir.Field{{Name: "children", Type: hir.T(hir.Array, hir.Ref("JSONLeaf"))}, {Name: "name", Type: str}})
	cold := shape("ColdShape", []hir.Field{{Name: "value", Type: number}})
	materialize := method("materialize", hir.Ref("JSONTop"), ret(rt("dynamic.materialize", hir.V("input", hir.T(hir.Dynamic)), hir.Ref("JSONTop"))))
	materialize.Params = []hir.Param{{Name: "input", Type: hir.T(hir.Dynamic)}}
	materialize.Static = true
	p := &hir.Program{Classes: []*hir.Class{leaf, top, cold, {Name: "JSONFactory", Methods: []*hir.Method{materialize}}}}
	names := hir.NewNames()
	main := fmt.Sprintf(`
 d:=parseJSON(str("{\"children\":[{\"value\":1,\"extra\":2}],\"name\":\"x\",\"unknown\":true}"))
 a:=%s(d);if box(a)!=d {panic("root source identity")}
 child:=castRef[%s](a.%s.Items[0]);original:=d.get(str("children")).Value.(*array[*dynamic]).Items[0]
 if box(child)!=original || box(child).get(str("extra")).Value.(float64)!=2 {panic("nested source identity")}
 c:=%s(3);other:=%s(3);if c==other || c.dynamicSource()!=nil || dynRef(box(c))!=c {panic("cold shape identity")}
 fmt.Println("ok")`, entry("JSONFactory", "materialize"), names.Get("ref.JSONLeaf"), names.Get("member.children"), names.Get("new.ColdShape"), names.Get("new.ColdShape"))
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
