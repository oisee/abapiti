package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func lspTokenJSONFixture() *hir.Program {
	position := "src/position.ts.Position"
	token := "src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken"
	field := func(owner, key string, typ hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(owner)}, Name: key, Type: typ}
	}
	makeClass := func(name string, fields []hir.Field, params []hir.Param) *hir.Class {
		body := hir.B()
		for _, f := range fields {
			body.List = append(body.List, &hir.Stmt{Kind: hir.Assign, X: field(name, f.Name, f.Type), Y: local(f.Name, f.Type)})
		}
		ctor := method("constructor", hir.T(hir.Void), body)
		ctor.Params = params
		return &hir.Class{Name: name, Fields: fields, Ctor: ctor}
	}
	p := makeClass(position, []hir.Field{{Name: "row", Type: i32, Private: true}, {Name: "col", Type: i32, Private: true}}, []hir.Param{{Name: "row", Type: i32}, {Name: "col", Type: i32}})
	tok := makeClass(token, []hir.Field{{Name: "start", Type: hir.Ref(position), Private: true}, {Name: "str", Type: str, Private: true}, {Name: "strUpper", Type: str, Private: true}}, []hir.Param{{Name: "start", Type: hir.Ref(position)}, {Name: "str", Type: str}, {Name: "strUpper", Type: str}})
	tok.Abstract = true
	tok.ViewUnsafe = true // Full closures seal prototype-bearing structural assertions.
	child := &hir.Class{Name: "src/abap/1_lexer/tokens/identifier.ts.Identifier", Super: token}
	pos := newObj(hir.Ref(position))
	pos.Args = []*hir.Expr{lit(1), lit(2)}
	obj := newObj(hir.Ref(child.Name))
	obj.Args = []*hir.Expr{pos, hir.L(str, "select"), hir.L(str, "SELECT")}
	probe := method("run", str, ret(rt("json.stringifySubset", obj, str)))
	probe.Static = true
	return &hir.Program{Classes: []*hir.Class{p, tok, child, {Name: "TokenJSONProbe", Methods: []*hir.Method{probe}}}}
}

func TestLSPTokenJSONDeclarationOrder(t *testing.T) {
	want, err := exec.Command("node", "-e", `class Position{constructor(row,col){this.row=row;this.col=col}}class AbstractToken{constructor(start,str,strUpper){this.start=start;this.str=str;this.strUpper=strUpper}}class Identifier extends AbstractToken{}process.stdout.write(JSON.stringify(new Identifier(new Position(1,2),"select","SELECT")))`).Output()
	if err != nil {
		t.Fatal(err)
	}
	p := lspTokenJSONFixture()
	base := p.Classes[1].Name
	unknown := &hir.Class{Name: "UnknownTokenLayout", Super: base, Fields: []hir.Field{{Name: "extra", Type: str}}}
	p.Classes = append(p.Classes, unknown)
	bad := newObj(hir.Ref(unknown.Name))
	position := newObj(hir.Ref(p.Classes[0].Name))
	position.Args = []*hir.Expr{lit(1), lit(2)}
	bad.Args = []*hir.Expr{position, hir.L(str, "select"), hir.L(str, "SELECT")}
	refused := method("refused", str, hir.B(decl("token", hir.Ref(base), bad), ret(rt("json.stringifySubset", local("token", hir.Ref(base)), str))))
	refused.Static = true
	view := method("view", hir.Ref(base), ret(rt("dynamic.classView", local("input", hir.T(hir.Dynamic)), hir.Ref(base))))
	view.Static = true
	view.Params = []hir.Param{{Name: "input", Type: hir.T(hir.Dynamic)}}
	p.Classes[3].Methods = append(p.Classes[3].Methods, refused, view)
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("TokenJSONProbe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_token DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_token IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nactual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `%s` ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.run"), want)
	files[owner+".clas.testclasses.abap"] = strings.Replace(files[owner+".clas.testclasses.abap"], "ENDMETHOD.", fmt.Sprintf("DATA refused TYPE abap_bool.\nTRY.\nactual = %s=>%s( ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nENDMETHOD.", owner, names.Get("member.refused"), names.Get("exception.RegistryJSONSubsetError")), 1)
	files[owner+".clas.testclasses.abap"] = strings.Replace(files[owner+".clas.testclasses.abap"], "ENDMETHOD.", fmt.Sprintf("DATA graph TYPE REF TO %s.\nDATA view TYPE REF TO %s.\nCREATE OBJECT graph.\ngraph->tag = %s=>tag_object.\nCLEAR refused.\nTRY.\nview = %s=>%s( graph ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nENDMETHOD.", names.Get("runtime.dynamic"), names.Get(base), names.Get("runtime.dynamic"), owner, names.Get("member.view"), names.Get("view.refusal")), 1)
	storageHostFixture(t, files, owner)
}

func TestLSPTokenJSONCertificateRefusals(t *testing.T) {
	for _, scenario := range []string{"order", "missing", "escape", "hook", "accessor", "scoped", "privateSlot", "prototype", "optionalSlot", "dynamicSlot", "subclass"} {
		t.Run(scenario, func(t *testing.T) {
			p := lspTokenJSONFixture()
			c := p.Classes[1]
			switch scenario {
			case "order":
				c.Ctor.Body.List[0], c.Ctor.Body.List[1] = c.Ctor.Body.List[1], c.Ctor.Body.List[0]
			case "missing":
				c.Ctor.Body.List = c.Ctor.Body.List[:2]
			case "escape":
				c.Ctor.Body.List[0].Y = &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}
			case "hook":
				c.Methods = append(c.Methods, method("toJSON", str, ret(hir.L(str, "hook"))))
			case "accessor":
				c.PrototypeAccessors = true
			case "scoped":
				c.Scoped = true
			case "privateSlot", "prototype":
				key := "#slot"
				if scenario == "prototype" {
					key = "__proto__"
				}
				c.Fields[0].Name = key
				c.Ctor.Body.List[0].X.Name = key
			case "optionalSlot":
				c.Fields[0].Type = hir.T(hir.Optional, c.Fields[0].Type)
			case "dynamicSlot":
				c.Fields[0].Type = hir.T(hir.Dynamic)
			case "subclass":
				p.Classes[2].Fields = []hir.Field{{Name: "extra", Type: str}}
			}
			e := &emitter{p: p}
			if _, ok := e.lspTokenJSONOrder(p.Classes[2]); ok {
				t.Fatal("unproven layout certified")
			}
		})
	}
}
