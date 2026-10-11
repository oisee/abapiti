package golang

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func configJSONFixture() *hir.Program {
	c := &hir.Class{Name: "src/_config.ts.IConfig"}
	op := &hir.Expr{Kind: hir.RuntimeOp, Op: "json.stringifyConfig", Type: hir.T(hir.String), X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}, Args: []*hir.Expr{hir.L(hir.T(hir.Optional, hir.T(hir.Dynamic)), nil), hir.L(hir.T(hir.Optional, hir.T(hir.String)), nil)}}
	c.Methods = []*hir.Method{{Name: "write", Result: hir.T(hir.String), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: op})}}
	return &hir.Program{Classes: []*hir.Class{c}}
}

func TestConfigJSONV8(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for the config JSON oracle")
	}
	// Exercise bytes, not a host JSON decoder: escaping and own-key order matter.
	script := `const o={z:"\"\\\b\t\n\f\r\u0000\ud800x\udfff😀<>&\u2028",a:[undefined,null,false,true,-0,1.25,1e-7,1e-6,1e20,1e21,NaN,Infinity],gone:undefined};o["10"]=10;o["2"]=2;o["01"]=1;console.log(JSON.stringify(o));`
	want, err := exec.Command(node, "-e", script).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	main := `d:=parseJSON(str("{\"z\":\"\\\"\\\\\\b\\t\\n\\f\\r\\u0000\\ud800x\\udfff😀<>&\\u2028\"}"));a:=&array[*dynamic]{};a.push(nil);a.push(dynNull());a.push(box(false));a.push(box(true));a.push(box(float64(0)));a.push(box(1.25));a.push(box(1e-7));a.push(box(1e-6));a.push(box(1e20));a.push(box(1e21));a.push(box(mathNaN()));a.push(box(mathInf()));d.put(str("a"),&dynamic{tagArray,a});d.put(str("gone"),nil);d.put(str("10"),box(int32(10)));d.put(str("2"),box(int32(2)));d.put(str("01"),box(int32(1)));fmt.Println(stringifyConfig(d).String());`
	// Test helper functions live in main.go, which has only fmt imported.
	main = strings.ReplaceAll(main, "mathNaN()", "func() float64 {v:=float64(0);return v/v}()")
	main = strings.ReplaceAll(main, "mathInf()", "func() float64 {v:=float64(0);return 1/v}()")
	if got := execute(t, configJSONFixture(), main); got != string(want) {
		t.Fatalf("got %q; V8 %q", got, want)
	}
}

func TestConfigJSONRefusals(t *testing.T) {
	p := configJSONFixture()
	if _, err := abap.Emit(p); err == nil || !strings.Contains(err.Error(), "json.stringifyConfig") {
		t.Fatalf("ABAP must refuse explicitly: %v", err)
	}
	main := `d:=parseJSON(str("{}"));d.put(str("self"),d);func(){defer func(){if _,ok:=recover().(trap);!ok {panic("cycle must trap")}}();stringifyConfig(d)}();fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestConfigJSONNativeGraph(t *testing.T) {
	p := configJSONFixture()
	root := p.Classes[0]
	strT := hir.T(hir.String)
	arrT := hir.T(hir.Array, strT)
	optBool := hir.T(hir.Optional, hir.T(hir.Bool))
	global := &hir.Class{Name: "src/_config.ts.IGlobalConfig", Fields: []hir.Field{{Name: "files", Type: strT}, {Name: "skipGeneratedBOPFInterfaces", Type: optBool}, {Name: "exclude", Type: arrT}, {Name: "noIssues", Type: arrT}}}
	base := &hir.Class{Name: "src/rules/_basic_rule_config.ts.BasicRuleConfig", Fields: []hir.Field{{Name: "exclude", Type: arrT}, {Name: "severity", Type: strT}}}
	conf := &hir.Class{Name: "src/rules/probe.ts.ProbeConf", Super: base.Name, Fields: []hir.Field{{Name: "enabled", Type: hir.T(hir.Bool)}}}
	root.Fields = []hir.Field{{Name: "global", Type: hir.Ref(global.Name)}, {Name: "rules", Type: hir.T(hir.OrderedMap, strT, hir.T(hir.Dynamic))}}
	p.Classes = append(p.Classes, global, base, conf)
	n := hir.NewNames()
	obj := func(s string) string { return n.Get("struct." + s) }
	f := func(s string) string { return n.Get("member." + s) }
	main := `g:=&` + obj(global.Name) + `{` + f("files") + `:str("/src/**/*.*"),` + f("exclude") + `:&array[jsString]{},` + f("noIssues") + `:&array[jsString]{},` + f("skipGeneratedBOPFInterfaces") + `:present(false)};`
	main += `c:=&` + obj(conf.Name) + `{` + obj(base.Name) + `:` + obj(base.Name) + `{` + f("exclude") + `:&array[jsString]{},` + f("severity") + `:str("Error")},` + f("enabled") + `:true};rules:=&orderedMap[jsString,*dynamic]{};rules.set(str("probe"),box(c));`
	main += `original:=&` + obj(root.Name) + `{` + f("global") + `:g,` + f("rules") + `:rules};text:=stringifyConfig(original);copy:=parseJSON(text);g.` + f("files") + `=str("changed");c.` + f("enabled") + `=false;if copy.get(str("global")).get(str("files")).Value.(jsString)!=str("/src/**/*.*") {panic("clone aliased original")};fmt.Println(stringifyConfig(copy).String());`
	want := "{\"global\":{\"files\":\"/src/**/*.*\",\"exclude\":[],\"noIssues\":[],\"skipGeneratedBOPFInterfaces\":false},\"rules\":{\"probe\":{\"exclude\":[],\"severity\":\"Error\",\"enabled\":true}}}\n"
	if got := execute(t, p, main); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestConfigJSONTypedVersionProjection(t *testing.T) {
	p := configJSONFixture()
	shape := func(name string, fields []hir.Field) *hir.Class {
		c := &hir.Class{Name: name, Fields: fields, Ctor: &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B()}}
		for _, f := range fields {
			c.Ctor.Params = append(c.Ctor.Params, hir.Param{Name: f.Name, Type: f.Type})
			c.Ctor.Body.List = append(c.Ctor.Body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: f.Type, Name: f.Name, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}}, Y: hir.V(f.Name, f.Type)})
		}
		return c
	}
	strT, dynT := hir.T(hir.String), hir.T(hir.Dynamic)
	version := shape("src/version.ts.VersionNew", []hir.Field{{Name: "release", Type: strT}, {Name: "language", Type: strT}})
	syntax := shape("src/_config.ts.ISyntaxSettings", []hir.Field{{Name: "version", Type: hir.T(hir.Optional, dynT)}, {Name: "errorNamespace", Type: strT}})
	graph := &hir.Expr{Kind: hir.RuntimeOp, Op: "json.parseSubset", Type: dynT, X: hir.V("text", strT)}
	project := &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.materialize", Type: hir.Ref(syntax.Name), X: graph}
	p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "parse", Static: true, Params: []hir.Param{{Name: "text", Type: strT}}, Result: project.Type, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: project})})
	p.Classes = append(p.Classes, version, syntax)
	n := hir.NewNames()
	field := func(s string) string { return n.Get("member." + s) }
	parse := entry(p.Classes[0].Name, "parse")
	main := `s:=` + parse + `(str("{\"version\":{\"release\":\"792\",\"language\":\"Normal\"},\"errorNamespace\":\".*\"}"));v:=castRef[` + n.Get("ref."+version.Name) + `](dynRef(s.` + field("version") + `));if v.` + field("release") + `!=str("792")||v.` + field("language") + `!=str("Normal") {panic("typed version projection")};fmt.Println(stringifyConfig(s).String());`
	main += `old:=` + parse + `(str("{\"version\":\"v702\",\"errorNamespace\":\".*\"}"));if old.` + field("version") + `.Tag!=1 {panic("string version changed")};missing:=` + parse + `(str("{\"errorNamespace\":\".*\"}"));if missing.` + field("version") + `!=nil {panic("absent version changed")};`
	main += `v.source=nil;func(){defer func(){if _,ok:=recover().(trap);!ok {panic("unproven native version order must trap")}}();stringifyConfigChecked(s,box(v),optional[jsString]{})}();`
	want := "{\"version\":{\"release\":\"792\",\"language\":\"Normal\"},\"errorNamespace\":\".*\"}\n"
	if got := execute(t, p, main); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
