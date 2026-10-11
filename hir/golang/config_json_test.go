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
	op := &hir.Expr{Kind: hir.RuntimeOp, Op: "json.stringifyConfig", Type: hir.T(hir.String), X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}}
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
