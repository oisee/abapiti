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

func json5SubsetFixture() *hir.Program {
	graph := rt("json5.parse", local("input", str), hir.T(hir.Dynamic))
	config := hir.Ref("src/_config.ts.IConfig")
	body := hir.B(decl("config", config, newObj(config)),
		&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: local("config", config), Name: "value", Type: hir.T(hir.Dynamic)}, Y: graph},
		ret(rt("json.stringifyConfig", local("config", config), str, hir.L(hir.T(hir.Optional, hir.T(hir.Dynamic)), nil), hir.L(hir.T(hir.Optional, str), nil))))
	m := method("roundtrip", str, body)
	m.Static, m.Params = true, []hir.Param{{Name: "input", Type: str}}
	return &hir.Program{Classes: []*hir.Class{{Name: "src/_config.ts.IConfig", Fields: []hir.Field{{Name: "value", Type: hir.T(hir.Dynamic)}}}, {Name: "JSON5Probe", Methods: []*hir.Method{m}}}}
}

func TestJSON5SubsetEmission(t *testing.T) {
	files, names, err := emitABAPNamed(t, json5SubsetFixture())
	if err != nil {
		t.Fatal(err)
	}
	source := files[names.Get("runtime.json5Subset")+".clas.abap"]
	for _, fragment := range []string{"window CO white", "window CN plain", "window CA string_end", "sy-fdpos", "SORT changes BY offset", "CONCATENATE LINES OF fragments", "comma_can_end", names.Get("runtime.jsonSubset") + "=>parse"} {
		if !strings.Contains(source, fragment) {
			t.Fatal("missing normalizer boundary", fragment)
		}
	}
	for _, forbidden := range []string{"REGEX", "PCRE", "REPLACE"} {
		if strings.Contains(source, forbidden) {
			t.Fatal("normalization must use the finite-state scanner", forbidden)
		}
	}
	for name, source := range files {
		for _, line := range strings.Split(source, "\n") {
			if len(line) > 255 {
				t.Fatalf("%s: overlong line", name)
			}
		}
	}
}

// Real JSON5 supplies successful value answers. Invalid and unsupported inputs
// must raise a coverage refusal, never become values or fabricated SyntaxError.
func TestJSON5SubsetFixtures(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(home, "dev/abaplint/packages/core/node_modules/json5")
	if _, err := os.Stat(filepath.Join(pkg, "package.json")); err != nil {
		t.Skip("original JSON5 package not installed")
	}
	valid := []string{
		`{"a":1}`, `/*comment*/{"a":1,}`, "//head\n[1,2,] //tail",
		`{"url":"https://host/a//b", "text":"/*not a comment*/ ,}",}`, "{\"a\":1,//x\r\n\"b\":[true,false,null,],}",
		`[[],{},[1,],{"nested":[{"x":2,},],},]`, `{"a":1,/*c*/}`, `{"a":1/*c*/,/*c*/}`, `{"a":1,"a":2,}`,
		`{"quote":"\" // retained", "slash":"\\ /* retained */",}`, `0/*separate*/`, "0//comment\u2028", "0//comment\u2029",
		`[0,-0,0.5,-2e+2,]`, `"😀 /* literal */ // literal"`, "/*a**/0", "/*a***/0", "0// eof",
	}
	for _, n := range []int{126, 127, 128, 129, 254, 255, 256, 257} {
		valid = append(valid, strings.Repeat(" ", n)+`/* boundary */{"a":[1,],}`)
		valid = append(valid, `{"a":"`+strings.Repeat("x", n)+`\"//still string",}`)
		valid = append(valid, `/*`+strings.Repeat("x", n)+`*/[1,]`)
		valid = append(valid, "//"+strings.Repeat("x", n)+"\n[1,]")
	}
	invalid := []string{"", " ", "/*", "/*x*", "/", `"unfinished`, `"escaped\`, "[1,,]", "{,}", "[,]", `{"a":,}`, `[1,/*x*/,]`, `{"a":1,,}`, "1/*x*/2", "tr/*x*/ue", "0//c\u2028garbage", "0//c\u2029garbage", "{}x", `[1,}`, `{"a":[1,}`, "0 //c\n 1", "\"raw\nline\""}
	unsupported := []string{`"bad\q"`, `{a:1}`, `{'a':1}`, `"\x41"`, "+1", "0x10", ".5", "1.", "NaN", "Infinity", "[NaN]"}
	data, _ := json.Marshal(valid)
	oracle, err := exec.Command("node", "-e", `const p=require(process.argv[1]);if(require(process.argv[1]+"/package.json").version!=="2.2.3")throw Error("pin");console.log(JSON.stringify(JSON.parse(process.argv[2]).map(s=>JSON.stringify(p.parse(s)))))`, pkg, string(data)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(oracle))
	}
	var answers []string
	if err := json.Unmarshal(oracle, &answers); err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(invalid)
	check, err := exec.Command("node", "-e", `const p=require(process.argv[1]);for(const s of JSON.parse(process.argv[2])){try{p.parse(s);throw Error("accepted invalid: "+s)}catch(e){if(e.name!=="SyntaxError")throw e}}console.log("ok")`, pkg, string(data)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(check))
	}
	data, _ = json.Marshal(unsupported)
	check, err = exec.Command("node", "-e", `const p=require(process.argv[1]);for(const s of JSON.parse(process.argv[2]))p.parse(s);console.log("ok")`, pkg, string(data)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(check))
	}
	files, names, err := emitABAPNamed(t, json5SubsetFixture())
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("JSON5Probe")
	var test strings.Builder
	test.WriteString("CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nDATA refused TYPE abap_bool.\n")
	fmt.Fprintf(&test, "DATA graph TYPE REF TO %s.\nDATA writer TYPE REF TO %s.\nCREATE OBJECT writer.\n", names.Get("runtime.dynamic"), names.Get("runtime.configJSON"))
	serial := 0
	literal := func(text string) string {
		serial++
		encoded, _ := json.Marshal(text)
		variable := fmt.Sprintf("literal%d", serial)
		value := fmt.Sprintf("decoded%d", serial)
		fmt.Fprintf(&test, "DATA %s TYPE string.\nDATA %s TYPE REF TO %s.\n", variable, value, names.Get("runtime.dynamic"))
		chars := []rune(string(encoded))
		for len(chars) > 0 {
			n := 64
			if len(chars) < n {
				n = len(chars)
			}
			fmt.Fprintf(&test, "CONCATENATE %s `%s` INTO %s RESPECTING BLANKS.\n", variable, strings.ReplaceAll(string(chars[:n]), "`", "``"), variable)
			chars = chars[n:]
		}
		fmt.Fprintf(&test, "%s = %s=>parse( %s ).\n", value, names.Get("runtime.jsonSubset"), variable)
		return value + "->sval"
	}

	for i, input := range valid {
		fmt.Fprintf(&test, "TRY.\ngraph = %s=>parse( %s ).\nCATCH %s.\nactual = %s=>normalize( %s ).\nCONCATENATE 'valid JSON5 case %d refused: ' actual INTO actual RESPECTING BLANKS.\ncl_abap_unit_assert=>fail( msg = actual ).\nENDTRY.\nactual = writer->%s( graph ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = %s msg = 'JSON5 subset case %d' ).\n", names.Get("runtime.json5Subset"), literal(input), names.Get("exception.JSON5SubsetRefusal"), names.Get("runtime.json5Subset"), literal(input), i, names.Get("json.write."+hir.T(hir.Dynamic).String()), literal(answers[i]), i)
	}
	for i, input := range append(invalid, unsupported...) {
		fmt.Fprintf(&test, "CLEAR refused.\nTRY.\ngraph = %s=>parse( %s ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( act = refused msg = 'JSON5 refusal %d' ).\n", names.Get("runtime.json5Subset"), literal(input), names.Get("exception.JSON5SubsetRefusal"), i)
	}
	fmt.Fprintf(&test, "actual = %s=>normalize( `-0` ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `-0` ).\n", names.Get("runtime.json5Subset"))
	test.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = test.String()
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		out = filepath.Join(out, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			formatted, err := Format(source)
			if err != nil {
				t.Fatal(name, err)
			}
			if err := os.WriteFile(filepath.Join(out, name), []byte(formatted), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
