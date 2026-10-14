package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func lspIdentifierJSONFixture() *hir.Program {
	p := lspTokenJSONFixture()
	name := "src/abap/4_file_information/_identifier.ts.Identifier"
	token := hir.Ref(p.Classes[1].Name)
	ctor := method("constructor", hir.T(hir.Void), hir.B())
	ctor.Params = []hir.Param{{Name: "token", Type: token}, {Name: "filename", Type: str}}
	for _, param := range ctor.Params {
		ctor.Body.List = append(ctor.Body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}, Name: param.Name, Type: param.Type}, Y: local(param.Name, param.Type)})
	}
	c := &hir.Class{Name: name, ViewUnsafe: true, Fields: []hir.Field{{Name: "token", Type: token, Private: true}, {Name: "filename", Type: str}}, Ctor: ctor}
	probe := p.Classes[3].Methods[0]
	obj := newObj(hir.Ref(name))
	obj.Args = []*hir.Expr{probe.Body.X.X, hir.L(str, "file \"α\".abap ")}
	probe.Body = ret(rt("json.stringifySubset", obj, str))
	p.Classes = append(p.Classes, c)
	return p
}

func TestLSPIdentifierJSONDeclarationOrder(t *testing.T) {
	want, err := exec.Command("node", "-e", `class Position{constructor(row,col){this.row=row;this.col=col}}class AbstractToken{constructor(start,str,strUpper){this.start=start;this.str=str;this.strUpper=strUpper}}class Token extends AbstractToken{}class Identifier{constructor(token,filename){this.token=token;this.filename=filename}}process.stdout.write(JSON.stringify(new Identifier(new Token(new Position(1,2),"select","SELECT"),'file "α".abap ')))`).Output()
	if err != nil {
		t.Fatal(err)
	}
	p := lspIdentifierJSONFixture()
	c := p.Classes[4]
	bad := &hir.Class{Name: "UnprovenIdentifier", Super: c.Name, Fields: []hir.Field{{Name: "extra", Type: str}}}
	p.Classes = append(p.Classes, bad)
	obj := newObj(hir.Ref(bad.Name))
	obj.Args = p.Classes[3].Methods[0].Body.X.X.Args
	refused := method("refused", str, hir.B(decl("id", hir.Ref(c.Name), obj), ret(rt("json.stringifySubset", local("id", hir.Ref(c.Name)), str))))
	refused.Static = true
	p.Classes[3].Methods = append(p.Classes[3].Methods, refused)
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("TokenJSONProbe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_identifier DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_identifier IMPLEMENTATION.
METHOD check.
DATA actual TYPE string.
DATA refused TYPE abap_bool.
actual = %s=>%s( ).
cl_abap_unit_assert=>assert_equals( act = actual exp = %s ).
TRY.
actual = %s=>%s( ).
CATCH %s.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
ENDMETHOD.
ENDCLASS.
`, owner, names.Get("member.run"), "`"+string(want)+"`", owner, names.Get("member.refused"), names.Get("exception.RegistryJSONSubsetError"))
	storageHostFixture(t, files, owner)
}

func TestLSPIdentifierJSONCertificateRefusals(t *testing.T) {
	for _, scenario := range []string{"order", "missing", "duplicate", "conditional", "escape", "hook", "accessor", "defineFields", "optional", "dynamic", "unknownRoot"} {
		t.Run(scenario, func(t *testing.T) {
			p := lspIdentifierJSONFixture()
			c := p.Classes[4]
			switch scenario {
			case "order":
				c.Ctor.Body.List[0], c.Ctor.Body.List[1] = c.Ctor.Body.List[1], c.Ctor.Body.List[0]
			case "missing":
				c.Ctor.Body.List = c.Ctor.Body.List[:1]
			case "duplicate":
				c.Ctor.Body.List = append(c.Ctor.Body.List, c.Ctor.Body.List[0])
			case "conditional":
				c.Ctor.Body.List = []*hir.Stmt{{Kind: hir.If, X: hir.L(hir.T(hir.Bool), true), Body: c.Ctor.Body}}
			case "escape":
				c.Ctor.Body.List[0].Y = &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}
			case "hook":
				c.Methods = append(c.Methods, method("toJSON", str, ret(hir.L(str, "hook"))))
			case "accessor":
				c.PrototypeAccessors = true
			case "defineFields":
				c.UndefinedDefineFields = true
			case "optional":
				c.Fields[0].Type = hir.T(hir.Optional, c.Fields[0].Type)
			case "dynamic":
				c.Fields[0].Type = hir.T(hir.Dynamic)
			case "unknownRoot":
				c.Name = "UnprovenRoot"
			}
			e := &emitter{p: p}
			if _, ok := e.lspTokenJSONOrder(c); ok {
				t.Fatal("unproven Identifier layout certified")
			}
		})
	}
}

func TestLSPIdentifierJSONTaggedPosition(t *testing.T) {
	out, err := exec.Command("node", "-e", `class Position{constructor(row,col){this.row=row;this.col=col}}class AbstractToken{constructor(start,str,strUpper){this.start=start;this.str=str;this.strUpper=strUpper}}class Token extends AbstractToken{}class Identifier{constructor(token,filename){this.token=token;this.filename=filename}}console.log(JSON.stringify(new Identifier(new Token(new Position(1,2),"select","SELECT"),'file "α".abap ')));console.log(JSON.stringify(new Position(NaN,2)));`).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(want) != 2 {
		t.Fatalf("Node: %s", out)
	}
	p := lspIdentifierJSONFixture()
	position := p.Classes[0]
	number := hir.T(hir.Number)
	for i := range position.Fields {
		position.Fields[i].Type = number
		position.Ctor.Params[i].Type = number
		position.Ctor.Body.List[i].X.Type = number
		position.Ctor.Body.List[i].Y.Type = number
	}
	probe := p.Classes[3]
	pos := probe.Methods[0].Body.X.X.Args[0].Args[0]
	for i := range pos.Args {
		pos.Args[i] = hir.L(number, float64(i+1))
	}
	nan := &hir.Expr{Kind: hir.New, Type: hir.Ref(position.Name), Args: []*hir.Expr{
		rt("string.parseFloatNumber", hir.L(str, "NaN"), number), hir.L(number, float64(2)),
	}}
	m := method("nan", str, ret(rt("json.stringifySubset", nan, str)))
	m.Static = true
	probe.Methods = append(probe.Methods, m)
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get(probe.Name)
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_tagged DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_tagged IMPLEMENTATION.
METHOD check.
DATA actual TYPE string.
actual = %s=>%s( ).
cl_abap_unit_assert=>assert_equals( act = actual exp = %s ).
actual = %s=>%s( ).
cl_abap_unit_assert=>assert_equals( act = actual exp = %s ).
ENDMETHOD.
ENDCLASS.
`, owner, names.Get("member.run"), "`"+want[0]+"`", owner, names.Get("member.nan"), "`"+want[1]+"`")
	if !strings.Contains(files[names.Get(position.Name)+".clas.abap"], "TYPE REF TO "+names.Get("runtime."+hir.T(hir.Optional, number).String())) {
		t.Fatal("fixture did not widen Position storage")
	}
	storageHostFixture(t, files, owner)
}
