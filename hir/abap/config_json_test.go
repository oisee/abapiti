package abap

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

type jsonMember struct {
	key   string
	value any
}
type jsonObject []jsonMember

func orderedJSON(t *testing.T, input string) any {
	t.Helper()
	d := json.NewDecoder(strings.NewReader(input))
	var read func() any
	read = func() any {
		token, err := d.Token()
		if err != nil {
			t.Fatal(err)
		}
		if delim, ok := token.(json.Delim); ok {
			if delim == '{' {
				o := jsonObject{}
				for d.More() {
					key, err := d.Token()
					if err != nil {
						t.Fatal(err)
					}
					o = append(o, jsonMember{key.(string), read()})
				}
				if _, err := d.Token(); err != nil {
					t.Fatal(err)
				}
				return o
			}
			a := []any{}
			for d.More() {
				a = append(a, read())
			}
			if _, err := d.Token(); err != nil {
				t.Fatal(err)
			}
			return a
		}
		return token
	}
	return read()
}

// The snapshot is from tools/config-json-oracle.mjs at upstream
// 577f875ebec44cfaf64841cfe71c8ab8dc32622e (194 default rule configs).
// Build native classes/arrays/records first, then exercise the actual runtime
// operation and typed projection; no pre-parsed source metadata is supplied.
func nativeConfigFixture(t *testing.T, input string) *hir.Program {
	t.Helper()
	dyn := hir.T(hir.Dynamic)
	p := &hir.Program{Classes: []*hir.Class{{Name: "src/rules/_basic_rule_config.ts.BasicRuleConfig", Fields: []hir.Field{{Name: "exclude", Type: dyn}, {Name: "severity", Type: dyn}}}}}
	body := hir.B()
	serial := 0
	var build func(any, string) *hir.Expr
	build = func(v any, path string) *hir.Expr {
		switch x := v.(type) {
		case nil:
			return rt("dynamic.null", lit(0), dyn)
		case string:
			return rt("dynamic.of", hir.L(str, x), dyn)
		case bool:
			return rt("dynamic.of", hir.L(boolean, x), dyn)
		case float64:
			return rt("dynamic.of", hir.L(hir.T(hir.Number), x), dyn)
		}
		serial++
		n := fmt.Sprintf("value%d", serial)
		if a, ok := v.([]any); ok {
			typ := hir.T(hir.Array, dyn)
			if path == "dependencies" {
				typ = hir.T(hir.Array, hir.Ref("src/_config.ts.IDependency"))
			}
			body.List = append(body.List, decl(n, typ, newObj(typ)))
			for _, item := range a {
				value := build(item, path+"[]")
				if path == "dependencies" {
					value = rt("dynamic.asRef", value, typ.Args[0])
				}
				body.List = append(body.List, run(rt("array.push", local(n, typ), i32, value)))
			}
			return rt("dynamic.of", local(n, typ), dyn)
		}
		o := v.(jsonObject)
		if path == "rules" {
			typ := hir.T(hir.OrderedMap, str, dyn)
			body.List = append(body.List, decl(n, typ, newObj(typ)))
			for _, item := range o {
				value := build(item.value, "rules."+item.key)
				body.List = append(body.List, run(rt("map.set", local(n, typ), typ, hir.L(str, item.key), value)))
			}
			return rt("dynamic.of", local(n, typ), dyn)
		}
		name := "src/rules/json_fixture.ts.Config" + n
		switch path {
		case "":
			name = "src/_config.ts.IConfig"
		case "global":
			name = "src/_config.ts.IGlobalConfig"
		case "syntax":
			name = "src/_config.ts.ISyntaxSettings"
		case "syntax.version":
			name = "src/version.ts.VersionNew"
		case "dependencies[]":
			name = "src/_config.ts.IDependency"
		}
		if len(o) == 2 && o[0].key == "pseudo" && o[1].key == "pragma" {
			name = "shape." + n
		}
		if len(o) == 3 && o[0].key == "name" && o[1].key == "replace" && o[2].key == "from" {
			name = "src/rules/function_module_recommendations.ts.Recommendations"
		}
		c := &hir.Class{Name: name, Ctor: &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B()}}
		if strings.HasPrefix(name, "src/rules/json_fixture") {
			c.Super = p.Classes[0].Name
		}
		typ := hir.Ref(name)
		for _, item := range o {
			fieldType := dyn
			if path == "" && item.key == "global" {
				fieldType = hir.Ref("src/_config.ts.IGlobalConfig")
			}
			if path == "" && item.key == "syntax" {
				fieldType = hir.Ref("src/_config.ts.ISyntaxSettings")
			}
			if path == "" && item.key == "dependencies" {
				fieldType = hir.T(hir.Array, hir.Ref("src/_config.ts.IDependency"))
			}
			if strings.HasPrefix(name, "shape.") {
				fieldType = str
			}
			if c.Super == "" || (item.key != "exclude" && item.key != "severity") {
				c.Fields = append(c.Fields, hir.Field{Name: item.key, Type: fieldType})
			}
			c.Ctor.Params = append(c.Ctor.Params, hir.Param{Name: item.key, Type: fieldType})
			c.Ctor.Body.List = append(c.Ctor.Body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: fieldType, Name: item.key, X: &hir.Expr{Kind: hir.This, Type: typ}}, Y: local(item.key, fieldType)})
		}
		// Repeated recommendation and anonymous rows share their shape definition.
		exists := false
		for _, old := range p.Classes {
			exists = exists || old.Name == name
		}
		if !exists {
			p.Classes = append(p.Classes, c)
		}
		instance := newObj(typ)
		for _, item := range o {
			if strings.HasPrefix(name, "shape.") {
				instance.Args = append(instance.Args, hir.L(str, item.value.(string)))
			} else {
				value := build(item.value, strings.TrimPrefix(path+"."+item.key, "."))
				if path == "" && (item.key == "global" || item.key == "syntax") {
					target := "src/_config.ts.IGlobalConfig"
					if item.key == "syntax" {
						target = "src/_config.ts.ISyntaxSettings"
					}
					value = rt("dynamic.asRef", value, hir.Ref(target))
				}
				if path == "" && item.key == "dependencies" {
					value = rt("dynamic.asRef", value, hir.T(hir.Array, hir.Ref("src/_config.ts.IDependency")))
				}
				instance.Args = append(instance.Args, value)
			}
		}
		body.List = append(body.List, decl(n, typ, instance))
		if path == "" {
			return local(n, typ)
		}
		return rt("dynamic.of", local(n, typ), dyn)
	}
	root := build(orderedJSON(t, input), "")
	stringify := func(x *hir.Expr) *hir.Expr {
		return rt("json.stringifyConfig", x, str, hir.L(hir.T(hir.Optional, dyn), nil), hir.L(hir.T(hir.Optional, str), nil))
	}
	body.List = append(body.List, decl("text", str, stringify(root)))
	projected := rt("dynamic.materialize", rt("json.parseSubset", local("text", str), dyn), root.Type)
	body.List = append(body.List, decl("copy", root.Type, projected), ret(stringify(local("copy", root.Type))))
	p.Classes = append(p.Classes, &hir.Class{Name: "json_fixture", Methods: []*hir.Method{{Name: "run", Static: true, Result: str, Body: body}}})
	return p
}

// Retained backing graphs preserve unknown keys and UTF-16 units which Go's
// encoding/json would replace. Undefined is added after parsing, as in JS.
func parsedConfigFixture(t *testing.T, input string) *hir.Program {
	p := nativeConfigFixture(t, `{"global":{},"syntax":{},"rules":{}}`)
	dyn := hir.T(hir.Dynamic)
	root := hir.Ref("src/_config.ts.IConfig")
	graph := local("graph", dyn)
	body := hir.B(decl("undefined", dyn, nil), decl("graph", dyn, rt("json.parseSubset", hir.L(str, input), dyn)),
		run(rt("dynamic.put", graph, hir.T(hir.Void), hir.L(str, "gone"), local("undefined", dyn))),
		decl("array", hir.T(hir.Array, dyn), newObj(hir.T(hir.Array, dyn))),
		run(rt("array.push", local("array", hir.T(hir.Array, dyn)), i32, local("undefined", dyn))),
		run(rt("dynamic.put", graph, hir.T(hir.Void), hir.L(str, "undefArray"), rt("dynamic.of", local("array", hir.T(hir.Array, dyn)), dyn))),
		decl("typed", root, rt("dynamic.materialize", graph, root)),
		ret(rt("json.stringifyConfig", local("typed", root), str, hir.L(hir.T(hir.Optional, dyn), nil), hir.L(hir.T(hir.Optional, str), nil))))
	p.Classes[len(p.Classes)-1].Methods[0].Body = body
	return p
}

func TestConfigJSONFixtures(t *testing.T) {
	configJSONFixtures(t, 0, 1, 2, 3)
}

// Tiny limits route the reference arm through chunk methods; the Node oracle
// and the probe order (most-derived first) must not change.
func TestConfigJSONChunkedReferenceArm(t *testing.T) {
	probes, chunk := configJSONRefProbes, configJSONRefChunk
	configJSONRefProbes, configJSONRefChunk = 2, 3
	t.Cleanup(func() { configJSONRefProbes, configJSONRefChunk = probes, chunk })
	files, names, err := emitABAPNamed(t, nativeConfigFixture(t, `{"global":{"files":"x"},"syntax":{"version":"v702"},"rules":{"a":{"exclude":[],"severity":"Error"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	writer := files[names.Get("runtime.configJSON")+".clas.abap"]
	method := names.Get("json.write." + hir.T(hir.Dynamic).String())
	body := writer[strings.Index(writer, "METHOD "+method+".\n"):]
	body = body[:strings.Index(body, "ENDMETHOD.")]
	if strings.Contains(body, "CATCH cx_sy_move_cast_error.") {
		t.Fatal("chunked reference arm still probes casts inline")
	}
	var order []string
	for i := 0; ; i++ {
		part := names.Get(fmt.Sprintf("json.write.%s.ref%d", hir.T(hir.Dynamic).String(), i))
		at := strings.Index(writer, "METHOD "+part+".\n")
		if at < 0 {
			if i < 2 {
				t.Fatalf("want at least two chunks, got %d", i)
			}
			break
		}
		if !strings.Contains(body, part+"( EXPORTING p0 = p0 IMPORTING result = result found = found ).") {
			t.Fatalf("chunk %d is not dispatched", i)
		}
		chunk := writer[at:]
		chunk = chunk[:strings.Index(chunk, "ENDMETHOD.")]
		if n := strings.Count(chunk, "CATCH cx_sy_move_cast_error."); n == 0 || n > 3 {
			t.Fatalf("chunk %d has %d probes", i, n)
		}
		for _, l := range strings.Split(chunk, "\n") {
			if strings.HasSuffix(l, " ?= p0->oval.") {
				order = append(order, strings.TrimSpace(strings.TrimSuffix(l, " ?= p0->oval.")))
			}
		}
	}
	configJSONRefProbes, configJSONRefChunk = probes, chunk
	inline, inlineNames, err := emitABAPNamed(t, nativeConfigFixture(t, `{"global":{"files":"x"},"syntax":{"version":"v702"},"rules":{"a":{"exclude":[],"severity":"Error"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, l := range strings.Split(inline[inlineNames.Get("runtime.configJSON")+".clas.abap"], "\n") {
		if strings.HasSuffix(l, " ?= p0->oval.") {
			want = append(want, strings.TrimSpace(strings.TrimSuffix(l, " ?= p0->oval.")))
		}
	}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("probe order changed:\n%v\n%v", order, want)
	}
	configJSONRefProbes, configJSONRefChunk = 2, 3
	configJSONFixtures(t, 0, 1, 3)
}

func configJSONFixtures(t *testing.T, cases ...int) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for config JSON byte oracle")
	}
	snapshot, err := os.ReadFile("testdata/config_default.json")
	if err != nil {
		t.Fatal(err)
	}
	inputs := []string{strings.TrimSpace(string(snapshot)), `{"global":{"files":"quote\" slash\\ line\n tab\t nul\u0000 <>&","exclude":["one",null,false,1.25,-0]},"dependencies":[{"url":"https://example.test","folder":"deps","files":"*.abap"}],"syntax":{"version":"v702"},"rules":{"nested":{"10":10,"2":2,"01":1,"arr":[{},[],null]}}}`, `{"global":{"files":"x","exclude":[1e-7,1e-6,1e20,1e21,-1.2345678901234567,5e-324,1.234567890123456,2.2250738585072014e-308,1.7976931348623157e308,-1e-300,1.0000000000000002,1000000000000000128,-8.102659436467237e253,8.788831760549637e-96,7.057884183898141e108,9476877.387300245,-7.940113795973939e-22,-1485038013007472.2,1837942576686376.2,31701645333072.062,-1967415756624292.2,1553908620950297.2]},"syntax":{"version":"v702"},"rules":{}}`, `{"":"empty","4294967295":"ordinary","4294967294":"index","z":"\"\\\b\f\n\r\t\u0000\ud800x\udfff\ud800\u0041\ud83d\ude00<>&\u2028","a":[null,false,true],"10":10,"2":2,"01":1,"global":{"files":"x","unknown":true}}`}
	out := t.TempDir()
	if base := fixtureOutputDir(t); base != "" {
		out = filepath.Join(base, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, i := range cases {
		input := inputs[i]
		script := `process.stdout.write(JSON.stringify(JSON.parse(process.argv[1])))`
		if i == 3 {
			script = `const o=JSON.parse(process.argv[1]);o.gone=undefined;o.undefArray=[undefined];process.stdout.write(JSON.stringify(o))`
		}
		oracle, err := exec.Command(node, "-e", script, input).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(oracle))
		}
		var p *hir.Program
		if i == 3 {
			p = parsedConfigFixture(t, input)
		} else {
			p = nativeConfigFixture(t, input)
		}
		files, names, err := emitABAPNamed(t, p)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			// dependencies activates live array casts. The v702 config gate
			// loads the array as a dependency: IS INSTANCE OF in an IF there
			// destroys its parse structure and hides even dense/items/missing.
			array := files[names.Get("runtime."+hir.T(hir.Array, hir.Ref(hir.RootObject)).String())+".clas.abap"]
			if !strings.Contains(array, "INTERFACES "+names.Get("view.storage")+".") || !strings.Contains(array, "CATCH cx_sy_move_cast_error.") || strings.Contains(array, "IS INSTANCE OF") {
				t.Fatal("live reference-array runtime must remain resolvable by the config JSON v702 gate")
			}
		}
		// Cases have overlapping generated identities, so give each a separate directory.
		dir := filepath.Join(out, fmt.Sprintf("case%d", i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		class := names.Get("json_fixture")
		var b strings.Builder
		fmt.Fprintf(&b, "CLASS ltcl_json DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS roundtrip FOR TESTING.\nENDCLASS.\nCLASS ltcl_json IMPLEMENTATION.\nMETHOD roundtrip.\nDATA actual TYPE string.\nDATA expected TYPE string.\nDATA oraclechar TYPE c LENGTH 1.\nactual = %s=>%s( ).\n", class, names.Get("member.run"))
		// Short literals preserve bytes and remain below ABAP's 255-byte line limit.
		jsonABAPExpected(&b, string(oracle))
		fmt.Fprintf(&b, "DATA(probe) = NEW %s( ).\ncl_abap_unit_assert=>assert_true( xsdbool( probe IS INSTANCE OF %s ) ).\n", class, class)
		b.WriteString("cl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\n")
		if i == 3 {
			cyclic, err := exec.Command(node, "-e", `const o={};o.self=o;try {JSON.stringify(o);process.exit(1)}catch(e){if(!(e instanceof TypeError))process.exit(2)}`).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(cyclic))
			}
			fmt.Fprintf(&b, "DATA graph TYPE REF TO %s.\nDATA writer TYPE REF TO %s.\nDATA caught TYPE abap_bool.\ngraph = %s=>parse( `{}` ).\ngraph->put( p0 = `self` p1 = graph ).\nCREATE OBJECT writer.\nTRY.\nactual = writer->%s( graph ).\nCATCH %s.\ncaught = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( caught ).\n", names.Get("runtime.dynamic"), names.Get("runtime.configJSON"), names.Get("runtime.jsonSubset"), names.Get("json.write."+hir.T(hir.Dynamic).String()), names.Get("exception.RegistryJSONSubsetError"))
		}
		b.WriteString("ENDMETHOD.\nENDCLASS.\n")
		files[class+".clas.testclasses.abap"] = b.String()
		for name, src := range files {
			for _, line := range strings.Split(src, "\n") {
				if len(line) > 255 {
					t.Fatalf("%s: line exceeds 255 bytes", name)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func jsonABAPChunks(s string) []string {
	var chunks []string
	var b strings.Builder
	for _, r := range s {
		if b.Len() >= 100 {
			chunks = append(chunks, b.String())
			b.Reset()
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		chunks = append(chunks, b.String())
	}
	return chunks
}

func jsonABAPExpected(b *strings.Builder, s string) {
	var chunk strings.Builder
	flush := func() {
		for _, v := range jsonABAPChunks(chunk.String()) {
			fmt.Fprintf(b, "CONCATENATE expected `%s` INTO expected RESPECTING BLANKS.\n", strings.ReplaceAll(v, "`", "``"))
		}
		chunk.Reset()
	}
	for _, r := range s {
		if r == 0x2028 || r == 0x2029 {
			flush()
			fmt.Fprintf(b, "oraclechar = cl_abap_conv_in_ce=>uccpi( %d ).\nCONCATENATE expected oraclechar INTO expected RESPECTING BLANKS.\n", r)
		} else {
			chunk.WriteRune(r)
		}
	}
	flush()
}
