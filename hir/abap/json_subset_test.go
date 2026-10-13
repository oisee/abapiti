package abap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// These fixtures execute the emitted runtime operation, not a Go serializer.
// ABAPITI_TEST_OUT retains ABAP Unit classes for the pinned OSG gogen runner.
func TestJSONSubsetFixtures(t *testing.T) {
	inputs := []struct{ input, gap string }{
		{`{"z":"quote\" slash\\ \b\f\n\r\t\u0000 \ud800x\udfff😀\u2028\u2029","10":10,"2":2,"01":1,"4294967295":1,"4294967294":2,"":0,"nested":[{},[],null,false,true]}`, ""},
		{`{"numbers":[-0,1e-7,1e-6,1e20,1e21,5e-324,2.2250738585072014e-308,1.7976931348623157e308,-1.2345678901234567,1000000000000000128,-8.102659436467237e253,8.788831760549637e-96,7.057884183898141e108,9476877.387300245,-7.940113795973939e-22,-1485038013007472.2,1837942576686376.2,31701645333072.062,-1967415756624292.2,1553908620950297.2]}`, "  "},
		{`{"b":[{"x":"\\\"{},:[]"},[],{}],"a":{}}`, "\t"},
		{`{"b":[1,2],"a":{}}`, "abcdefghijklm"},
		{`{}`, "😀😀😀😀😀😀"},
		{`{"a":[1,{"b":2}]}`, "123456789😀"},
	}
	out := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out = filepath.Join(base, t.Name())
	}
	dyn := hir.T(hir.Dynamic)
	// Unsupported cases have observable Node behavior; accepting them would
	// require callable toJSON, undefined-result and cyclic-error ABIs.
	negativeOracle, err := exec.Command("node", "-e", `const a={};a.self=a;let cyclic=false;try{JSON.stringify(a)}catch(e){cyclic=e instanceof TypeError}if(!cyclic||JSON.stringify(undefined)!==undefined||JSON.stringify(class F{})!==undefined||JSON.stringify({toJSON(){return "called"}})!=='"called"'||JSON.stringify(new Map([["x",1]]))!=="{}")process.exit(1)`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(negativeOracle))
	}
	hookOracle, err := exec.Command("node", "-e", `class Hook{static toJSON(){return "hook"}}class Child extends Hook{}if(JSON.stringify({__proto__:{toJSON(){return "proto"}}})!=='"proto"')process.exit(2);class Getter{static get toJSON(){return ()=>"getter"}}if(JSON.stringify({fn:Getter})!=='{"fn":"getter"}'||JSON.stringify({fn:Child})!=='{"fn":"hook"}'||JSON.stringify([Child])!=='["hook"]')process.exit(1)`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(hookOracle))
	}
	for i, c := range inputs {
		script := `const o=JSON.parse(process.argv[1]);o.gone=undefined;o.fn=class F{};o.undefArray=[undefined,class F{}];process.stdout.write(JSON.stringify(JSON.stringify(o,null,process.argv[2])))`
		oracle, err := exec.Command("node", "-e", script, c.input, c.gap).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(oracle))
		}
		graph := local("graph", dyn)
		arrayType := hir.T(hir.Array, dyn)
		p := &hir.Program{Classes: []*hir.Class{{Name: "json_fixture", Methods: []*hir.Method{{Name: "run", Static: true, Result: str}}}, {Name: "Function"}, {Name: "Hook", Methods: []*hir.Method{{Name: "toJSON", Static: true, Result: str, Body: hir.B(ret(hir.L(str, "hook")))}}}, {Name: "InheritedHook", Super: "Hook"}, {Name: "GetterHook", PrototypeAccessors: true, ViewUnsafe: true, Methods: []*hir.Method{{Name: "get.toJSON", Static: true, Result: str, Body: hir.B(ret(hir.L(str, "getter")))}}}, {Name: "Opaque", CoercionUnsafe: true}, {Name: "Unproven", Methods: []*hir.Method{{Name: "toJSON", Result: str, Body: hir.B(ret(hir.L(str, "called")))}}}}}
		body := hir.B(decl("undefined", dyn, nil), decl("graph", dyn, rt("json.parseSubset", hir.L(str, c.input), dyn)),
			run(rt("dynamic.put", graph, hir.T(hir.Void), hir.L(str, "gone"), local("undefined", dyn))),
			decl("fn", dyn, rt("dynamic.of", &hir.Expr{Kind: hir.ClassOf, Owner: "Function", Type: hir.T(hir.ClassValue)}, dyn)),
			run(rt("dynamic.put", graph, hir.T(hir.Void), hir.L(str, "fn"), local("fn", dyn))),
			decl("array", arrayType, newObj(arrayType)),
			run(rt("array.push", local("array", arrayType), i32, local("undefined", dyn))),
			run(rt("array.push", local("array", arrayType), i32, local("fn", dyn))),
			run(rt("dynamic.put", graph, hir.T(hir.Void), hir.L(str, "undefArray"), rt("dynamic.of", local("array", arrayType), dyn))))
		text := rt("json.stringifySubset", graph, str)
		text = rt("json.indent", text, str, hir.L(str, c.gap))
		body.List = append(body.List, ret(text))
		p.Classes[0].Methods[0].Body = body
		p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "write", Static: true, Params: []hir.Param{{Name: "value", Type: dyn}}, Result: str, Body: hir.B(ret(rt("json.stringifySubset", local("value", dyn), str)))})
		scalars := []struct {
			typ   hir.Type
			value any
			js    string
		}{
			{str, "literal", `"literal"`}, {boolean, true, "true"}, {i32, int32(-42), "-42"},
			{hir.T(hir.I64), int64(9007199254740991), "9007199254740991"},
			{hir.T(hir.Number), float64(1e21), "1e21"},
			{hir.T(hir.Optional, str), "present", `"present"`},
		}
		for j, scalar := range scalars {
			p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: fmt.Sprintf("scalar%d", j), Static: true, Result: str, Body: hir.B(ret(rt("json.stringifySubset", hir.L(scalar.typ, scalar.value), str)))})
		}
		at := hir.T(hir.Array, hir.T(hir.Optional, str))
		ar := local("items", at)
		p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "array", Static: true, Result: str, Body: hir.B(decl("items", at, newObj(at)), run(rt("array.push", ar, i32, hir.L(hir.T(hir.Optional, str), nil))), run(rt("array.push", ar, i32, hir.L(hir.T(hir.Optional, str), "value"))), ret(rt("json.stringifySubset", ar, str)))})
		for _, entry := range []struct {
			name  string
			value *hir.Expr
		}{
			{"function", &hir.Expr{Kind: hir.ClassOf, Owner: "Function", Type: hir.T(hir.ClassValue)}},
			{"absent", hir.L(hir.T(hir.Optional, str), nil)},
			{"unsafe", hir.L(hir.T(hir.I64), int64(9007199254740992))},
		} {
			p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: entry.name, Static: true, Result: str, Body: hir.B(ret(rt("json.stringifySubset", entry.value, str)))})
		}
		hook := &hir.Expr{Kind: hir.ClassOf, Owner: "InheritedHook", Type: hir.T(hir.ClassValue)}
		p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "hook_property", Static: true, Result: str, Body: hir.B(decl("value", dyn, rt("json.parseSubset", hir.L(str, `{}`), dyn)), run(rt("dynamic.put", local("value", dyn), hir.T(hir.Void), hir.L(str, "fn"), rt("dynamic.of", hook, dyn))), ret(rt("json.stringifySubset", local("value", dyn), str)))})
		functionArray := hir.T(hir.Array, hir.T(hir.ClassValue))
		p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "hook_array", Static: true, Result: str, Body: hir.B(decl("value", functionArray, newObj(functionArray)), run(rt("array.push", local("value", functionArray), i32, hook)), ret(rt("json.stringifySubset", local("value", functionArray), str)))})
		for _, name := range []string{"GetterHook", "Opaque"} {
			fn := &hir.Expr{Kind: hir.ClassOf, Owner: name, Type: hir.T(hir.ClassValue)}
			p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "refuse_" + name, Static: true, Result: str, Body: hir.B(decl("value", dyn, rt("json.parseSubset", hir.L(str, `{}`), dyn)), run(rt("dynamic.put", local("value", dyn), hir.T(hir.Void), hir.L(str, "fn"), rt("dynamic.of", fn, dyn))), ret(rt("json.stringifySubset", local("value", dyn), str)))})
		}
		files, names, err := EmitNamed(p)
		if err != nil {
			t.Fatal(err)
		}
		cls := names.Get("json_fixture")
		var b strings.Builder
		fmt.Fprintf(&b, "CLASS ltcl_json DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_json IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nDATA expected TYPE string.\nDATA oraclechar TYPE c LENGTH 1.\nactual = %s=>%s( ).\n", cls, names.Get("member.run"))
		jsonABAPExpected(&b, string(oracle))
		fmt.Fprintf(&b, "DATA oracle TYPE REF TO %s.\noracle = %s=>parse( expected ).\nexpected = oracle->sval.\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\n", names.Get("runtime.dynamic"), names.Get("runtime.jsonSubset"))
		for j, scalar := range scalars {
			expected, err := exec.Command("node", "-e", `process.stdout.write(JSON.stringify(JSON.parse(process.argv[1])))`, "--", scalar.js).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(expected))
			}
			fmt.Fprintf(&b, "actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `%s` ).\n", cls, names.Get(fmt.Sprintf("member.scalar%d", j)), expected)
		}
		arrayOracle, err := exec.Command("node", "-e", `process.stdout.write(JSON.stringify([undefined,"value"]))`).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(arrayOracle))
		}
		fmt.Fprintf(&b, "actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `%s` ).\n", cls, names.Get("member.array"), arrayOracle)
		// Runtime checks must reject an undefined root, native toJSON, unknown
		// dynamic tags, and cycles. A refusal cannot produce a default string.
		fmt.Fprintf(&b, "DATA graph TYPE REF TO %s.\nDATA native TYPE REF TO %s.\nDATA child TYPE REF TO %s.\nDATA caught TYPE abap_bool.\n", names.Get("runtime.dynamic"), names.Get("Unproven"), names.Get("runtime.dynamic"))
		writer := names.Get("member.write")
		for _, setup := range []string{"CLEAR graph.", fmt.Sprintf("CREATE OBJECT graph.\ngraph->tag = %s=>tag_ref.\nCREATE OBJECT native.\ngraph->oval = native.", names.Get("runtime.dynamic")), "CREATE OBJECT graph.\ngraph->tag = 999.", fmt.Sprintf("CREATE OBJECT graph.\ngraph->tag = %s=>tag_class.", names.Get("runtime.dynamic")), fmt.Sprintf("graph = %s=>parse( `{}` ).\ngraph->put( p0 = `self` p1 = graph ).", names.Get("runtime.jsonSubset")), fmt.Sprintf("graph = %s=>parse( `{}` ).\nCREATE OBJECT child.\nchild->tag = %s=>tag_class.\ngraph->put( p0 = `toJSON` p1 = child ).", names.Get("runtime.jsonSubset"), names.Get("runtime.dynamic")), fmt.Sprintf("graph = %s=>parse( `{}` ).\nCREATE OBJECT child.\nchild->tag = %s=>tag_class.\ngraph->put( p0 = `fn` p1 = child ).", names.Get("runtime.jsonSubset"), names.Get("runtime.dynamic")), fmt.Sprintf("graph = %s=>parse( `{\"__proto__\":null}` ).", names.Get("runtime.jsonSubset"))} {
			fmt.Fprintf(&b, "%s\nCLEAR caught.\nTRY.\nactual = %s=>%s( graph ).\nCATCH %s.\ncaught = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( caught ).\n", setup, cls, writer, names.Get("exception.RegistryJSONSubsetError"))
		}
		for _, method := range []string{"function", "absent", "unsafe", "hook_property", "hook_array", "refuse_GetterHook", "refuse_Opaque"} {
			fmt.Fprintf(&b, "CLEAR caught.\nTRY.\nactual = %s=>%s( ).\nCATCH %s.\ncaught = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( caught ).\n", cls, names.Get("member."+method), names.Get("exception.RegistryJSONSubsetError"))
		}
		b.WriteString("ENDMETHOD.\nENDCLASS.\n")
		files[cls+".clas.testclasses.abap"] = b.String()
		dir := filepath.Join(out, fmt.Sprintf("case%d", i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestJSONSubsetShapeRefusals(t *testing.T) {
	for _, typ := range []hir.Type{hir.Ref("Unknown"), hir.T(hir.OrderedMap, str, hir.T(hir.Dynamic)), hir.T(hir.OrderedSet, str), hir.T(hir.RegExp), hir.T(hir.Array, hir.Ref("Unknown"))} {
		p := &hir.Program{Classes: []*hir.Class{{Name: "Unknown"}, {Name: "Probe", Methods: []*hir.Method{{Name: "write", Static: true, Params: []hir.Param{{Name: "value", Type: typ}}, Result: str, Body: hir.B(ret(rt("json.stringifySubset", local("value", typ), str)))}}}}}
		if _, err := Emit(p); err == nil || !strings.Contains(err.Error(), "json.stringifySubset ABAP own-property order/toJSON is unproven") {
			t.Fatalf("%s: %v", typ, err)
		}
	}
}

func TestJSONSubsetMaterializedFixtures(t *testing.T) {
	dyn, opt := hir.T(hir.Dynamic), hir.T(hir.Optional, str)
	shape := hir.Ref("Parsed")
	c := &hir.Class{Name: shape.Name, Structural: true, Ctor: &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B()}}
	field := func(x *hir.Expr, key string) *hir.Expr {
		return &hir.Expr{Kind: hir.FieldGet, Type: opt, Name: key, X: x}
	}
	for _, key := range []string{"a", "b", "c", "d"} {
		c.Fields = append(c.Fields, hir.Field{Name: key, Type: opt})
		c.Ctor.Params = append(c.Ctor.Params, hir.Param{Name: key, Type: opt})
		c.Ctor.Body.List = append(c.Ctor.Body.List, &hir.Stmt{Kind: hir.Assign, X: field(&hir.Expr{Kind: hir.This, Type: shape}, key), Y: local(key, opt)})
	}
	input := `{"2":2,"unknown":{"z":true},"b":"second","a":"first"}`
	out := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out = filepath.Join(base, t.Name())
	}
	for mode := 0; mode < 6; mode++ {
		// Build a fresh method body: ABAP target passes mutate HIR in place.
		if mode == 5 {
			c.Fields = append(c.Fields, hir.Field{Name: "toJSON", Type: dyn})
			c.Ctor.Params = append(c.Ctor.Params, hir.Param{Name: "toJSON", Type: dyn})
			c.Ctor.Body.List = append(c.Ctor.Body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: dyn, Name: "toJSON", X: &hir.Expr{Kind: hir.This, Type: shape}}, Y: local("toJSON", dyn)})
		}
		body := hir.B(decl("graph", dyn, rt("json.parseSubset", hir.L(str, input), dyn)), decl("typed", shape, rt("dynamic.materialize", local("graph", dyn), shape)))
		obj := local("typed", shape)
		for _, change := range []struct {
			key   string
			value any
		}{{"b", "changed"}, {"a", nil}, {"c", "new"}} {
			body.List = append(body.List, &hir.Stmt{Kind: hir.Assign, X: field(obj, change.key), Y: hir.L(opt, change.value)})
		}
		value := obj
		switch mode {
		case 1:
			body.List = append(body.List, &hir.Stmt{Kind: hir.Assign, X: field(obj, "d"), Y: hir.L(opt, "two new keys")})
		case 2:
			native := newObj(shape)
			for range c.Fields {
				native.Args = append(native.Args, hir.L(opt, nil))
			}
			body.List = append(body.List, assign("typed", shape, native))
		case 3:
			value = rt("dynamic.of", obj, dyn)
		case 5:
			body.List = append(body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: dyn, Name: "toJSON", X: obj}, Y: rt("dynamic.of", &hir.Expr{Kind: hir.ClassOf, Owner: "Callable", Type: hir.T(hir.ClassValue)}, dyn)})
		case 4:
			at := hir.T(hir.Array, shape)
			a := local("list", at)
			body.List = append(body.List, decl("list", at, newObj(at)), run(rt("array.push", a, i32, obj)))
			value = a
		}
		body.List = append(body.List, ret(rt("json.stringifySubset", value, str)))
		p := &hir.Program{Classes: []*hir.Class{c, {Name: "Callable"}, {Name: "json_fixture", Methods: []*hir.Method{{Name: "run", Static: true, Result: str, Body: body}}}}}
		files, names, err := EmitNamed(p)
		if err != nil {
			t.Fatal(err)
		}
		cls := names.Get("json_fixture")
		var b strings.Builder
		fmt.Fprintf(&b, "CLASS ltcl_json DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_json IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nDATA expected TYPE string.\nDATA oraclechar TYPE c LENGTH 1.\n")
		if mode == 1 || mode == 2 || mode == 5 {
			fmt.Fprintf(&b, "DATA caught TYPE abap_bool.\nTRY.\nactual = %s=>%s( ).\nCATCH %s.\ncaught = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( caught ).\n", cls, names.Get("member.run"), names.Get("exception.RegistryJSONSubsetError"))
		} else {
			script := `const o=JSON.parse(process.argv[1]);o.b="changed";o.a=undefined;o.c="new";process.stdout.write(JSON.stringify(process.argv[2]==="4"?[o]:o))`
			oracle, err := exec.Command("node", "-e", script, input, fmt.Sprint(mode)).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(oracle))
			}
			fmt.Fprintf(&b, "actual = %s=>%s( ).\n", cls, names.Get("member.run"))
			jsonABAPExpected(&b, string(oracle))
			b.WriteString("cl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\n")
		}
		b.WriteString("ENDMETHOD.\nENDCLASS.\n")
		files[cls+".clas.testclasses.abap"] = b.String()
		dir := filepath.Join(out, fmt.Sprintf("case%d", mode))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestJSONSubsetConfigFixture(t *testing.T) {
	input := `{"rules":{"nested":{"10":10,"2":2,"a":[{},[],null]}},"global":{"files":"old","unknown":true},"syntax":{"version":"v702"},"extra":false}`
	p := nativeConfigFixture(t, `{"global":{"files":"old"},"syntax":{"version":"v702"},"rules":{}}`)
	dyn, root := hir.T(hir.Dynamic), hir.Ref("src/_config.ts.IConfig")
	obj := local("typed", root)
	global := &hir.Expr{Kind: hir.FieldGet, Type: hir.Ref("src/_config.ts.IGlobalConfig"), Name: "global", X: obj}
	filesField := &hir.Expr{Kind: hir.FieldGet, Type: dyn, Name: "files", X: global}
	body := hir.B(decl("typed", root, rt("dynamic.materialize", rt("json.parseSubset", hir.L(str, input), dyn), root)),
		&hir.Stmt{Kind: hir.Assign, X: filesField, Y: rt("dynamic.of", hir.L(str, "changed"), dyn)},
		ret(rt("json.indent", rt("json.stringifySubset", obj, str), str, hir.L(str, "  "))))
	p.Classes[len(p.Classes)-1].Methods[0].Body = body
	oracle, err := exec.Command("node", "-e", `const o=JSON.parse(process.argv[1]);o.global.files="changed";process.stdout.write(JSON.stringify(JSON.stringify(o,null,2)))`, input).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(oracle))
	}
	files, names, err := EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	cls := names.Get("json_fixture")
	var b strings.Builder
	fmt.Fprintf(&b, "CLASS ltcl_json DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_json IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nDATA expected TYPE string.\nDATA oraclechar TYPE c LENGTH 1.\nDATA oracle TYPE REF TO %s.\nactual = %s=>%s( ).\n", names.Get("runtime.dynamic"), cls, names.Get("member.run"))
	jsonABAPExpected(&b, string(oracle))
	fmt.Fprintf(&b, "oracle = %s=>parse( expected ).\nexpected = oracle->sval.\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\nENDMETHOD.\nENDCLASS.\n", names.Get("runtime.jsonSubset"))
	files[cls+".clas.testclasses.abap"] = b.String()
	out := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out = filepath.Join(base, t.Name())
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
