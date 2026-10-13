package abap

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func stringXMLFixture() *hir.Program {
	number, graph, regex := hir.T(hir.Number), hir.T(hir.Dynamic), hir.T(hir.RegExp)
	methods := []*hir.Method{}
	add := func(name, op string, result hir.Type, params ...hir.Param) {
		args := []*hir.Expr{}
		for _, p := range params {
			args = append(args, local(p.Name, p.Type))
		}
		m := method(name, result, ret(rt(op, local("text", str), result, args...)))
		m.Static, m.Params = true, append([]hir.Param{{Name: "text", Type: str}}, params...)
		methods = append(methods, m)
	}
	add("pad", "string.padStart", str, hir.Param{Name: "target", Type: number}, hir.Param{Name: "fill", Type: str})
	add("float", "string.parseFloatNumber", number)
	add("sha", "string.sha1Hex", str)
	add("split", "string.splitRegex", hir.T(hir.Array, str), hir.Param{Name: "pattern", Type: regex}, hir.Param{Name: "limit", Type: number})
	add("xml", "xml.parseWithErrors", graph)
	// Generate the existing graph writer for observing XML values (not the
	// independently owned stringifySubset operation).
	config := hir.Ref("src/_config.ts.IConfig")
	m := method("writer", str, ret(rt("json.stringifyConfig", newObj(config), str, hir.L(hir.T(hir.Optional, graph), nil), hir.L(hir.T(hir.Optional, str), nil))))
	m.Static = true
	methods = append(methods, m)
	errorCtor := method("constructor", hir.T(hir.Void), hir.B())
	errorCtor.Params = []hir.Param{{Name: "message", Type: hir.T(hir.Optional, str)}}
	return &hir.Program{Classes: []*hir.Class{
		{Name: "StringXMLProbe", Methods: methods},
		{Name: "builtin.Error", Fields: []hir.Field{{Name: "message", Type: str}}, Ctor: errorCtor},
		{Name: "src/_config.ts.IConfig", Fields: []hir.Field{{Name: "value", Type: graph}}},
	}}
}

func TestStringXMLEmission(t *testing.T) {
	files, names, err := EmitNamed(stringXMLFixture())
	if err != nil {
		t.Fatal(err)
	}
	text := files[names.Get("runtime.stringOps")+".clas.abap"]
	for _, want := range []string{"METHOD pad.", "METHOD float_prefix.", "METHOD sha1.", "METHOD split.", "4294967296", "token+0(1) = '-'", "RAISE EXCEPTION TYPE " + names.Get("exception.StringOpRefusal")} {
		if !strings.Contains(text, want) {
			t.Fatal("missing string boundary", want)
		}
	}
	xml := files[names.Get("runtime.xmlWithErrors")+".clas.abap"]
	for _, want := range []string{"readTagExp returned undefined", "fault->payload = native", "closed = abap_true", "part <> `?xml `"} {
		if !strings.Contains(xml, want) {
			t.Fatal("missing XML boundary", want)
		}
	}
	for path, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Fatal(path, "line too long")
			}
		}
	}
	plain, err := Emit(&hir.Program{Classes: []*hir.Class{{Name: "Plain", Methods: []*hir.Method{method("run", i32, ret(lit(1)))}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range plain {
		if strings.Contains(src, "float_prefix") || strings.Contains(src, "readTagExp") {
			t.Fatal("runtime escaped usage gate")
		}
	}
}

func TestStringXMLFixtures(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	parser := filepath.Join(home, "dev/abaplint/packages/core/node_modules/fast-xml-parser")
	if _, err := os.Stat(filepath.Join(parser, "package.json")); err != nil {
		t.Skip("pinned XML Node oracle not installed")
	}
	type padCase struct {
		Text   string
		Target float64
		Fill   string
	}
	pads := []padCase{{"abc", 8, "01"}, {"", 4, "xy"}, {" x ", 7, " "}, {"😀", 5, "abc"}, {"a", 4, "😀"}, {"a", 4.9, "0"}, {"abc", -1, "0"}, {"abc", 2e9, ""}, {"already", 3, "0"}, {"", 0, "x"}, {"a", -0.9, "x"}, {"a", 9, "ab "}}
	floats := []string{"1.5tail", "  -12.25kg", "+.5e2!", "1e+garbage", "0x10", ".125", "000123.5", "12.", "1e-3", "9007199254740993", "1.2345678901234567", "5e-324", "1.7976931348623157e308", "0", "1e-9999", "\u00a0\ufeff\u2000+42 rest"}
	hashes := []string{"", "abc", " x ", "é", "😀", "a\x00b", "a\nβ", "中文", strings.Repeat("x", 1024)}
	inputs := []string{"", "a b", " a b ", "a,b:(c", "\t\n", "a\u00a0b\ufeffc", "a\u180eb", "😀 x 😀", "one\u2028two", "a\r\nb", strings.Repeat("x", 128) + " " + strings.Repeat("y", 129)}
	patterns := []string{`\s+`, `[\s.,:(]+`}
	limits := []float64{0, 1, 2, 3, 4294967295, -1, 4294967296, 4294967297, 2.9, -2.9, 9007199254740991, -9007199254740991}
	xmls := []string{`<root id="ignored"><item> 001 </item><item>true</item><empty/></root>`, `<?xml version="1.0"?><a><b>A &amp; B</b></a>`, "<root><text>A\r\nB😀</text></root>", `<foo></bar>`, `<a><b></a>`, `<root>`, `<root>tail`, `<root><leaf/>`, "", `<a>committed<b/>tail`, `<a x='q>r'>x</a>`}
	errors := []string{"<", " <", "<root x=\"oops", "<a><", "<abc" + strings.Repeat("x", 100)}
	data, _ := json.Marshal([]any{pads, floats, hashes, inputs, patterns, limits, xmls, errors})
	script := `const [pads,floats,hashes,inputs,patterns,limits,xmls,errors]=JSON.parse(process.argv[2]);const {XMLParser}=require(process.argv[1]);if(require(process.argv[1]+"/package.json").version!=="5.10.1")throw Error("pin");const p=new XMLParser({parseTagValue:false,ignoreAttributes:true,trimValues:false});console.log(JSON.stringify({pads:pads.map(x=>x.Text.padStart(x.Target,x.Fill)),floats:floats.map(parseFloat),hashes:hashes.map(s=>require("crypto").createHash("sha1").update(s).digest("hex")),splits:patterns.flatMap(p=>inputs.flatMap(s=>limits.map(n=>s.split(new RegExp(p,"g"),n)))),xmls:xmls.map(s=>JSON.stringify(p.parse(s))),errors:errors.map(s=>{try{p.parse(s);throw Error("accepted")}catch(e){return e.message}})}))`
	out, err := exec.Command("node", "-e", script, parser, string(data)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	var answers struct {
		Pads   []json.RawMessage
		Floats []float64
		Hashes []string
		Splits [][]string
		Xmls   []string
		Errors []string
	}
	if err := json.Unmarshal(out, &answers); err != nil {
		t.Fatal(err, string(out))
	}
	files, names, err := EmitNamed(stringXMLFixture())
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("StringXMLProbe")
	d := names.Get("runtime.dynamic")
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nDATA numeric TYPE f.\nDATA expected TYPE f.\nDATA refused TYPE abap_bool.\n")
	fmt.Fprintf(&unit, "DATA graph TYPE REF TO %s.\nDATA writer TYPE REF TO %s.\nCREATE OBJECT writer.\nDATA pieces TYPE %s.\nDATA part TYPE string.\nDATA pattern TYPE REF TO %s.\nDATA fault TYPE REF TO %s.\n", d, names.Get("runtime.configJSON"), "REF TO "+names.Get("runtime."+hir.T(hir.Array, str).String()), names.Get("runtime.regexp"), names.Get("exception."+hir.Ref("builtin.Error").String()))
	// JSON decoding creates exact UTF-16 fixture inputs, including control units.
	serial := 0
	literalJSON := func(encoded string) string {
		serial++
		variable := fmt.Sprintf("literal%d", serial)
		value := fmt.Sprintf("decoded%d", serial)
		fmt.Fprintf(&unit, "DATA %s TYPE string.\nDATA %s TYPE REF TO %s.\n", variable, value, d)
		chars := []rune(strings.ReplaceAll(encoded, "\ufeff", `\ufeff`))
		for len(chars) > 0 {
			n := 64
			if len(chars) < n {
				n = len(chars)
			}
			fmt.Fprintf(&unit, "CONCATENATE %s `%s` INTO %s RESPECTING BLANKS.\n", variable, strings.ReplaceAll(string(chars[:n]), "`", "``"), variable)
			chars = chars[n:]
		}
		fmt.Fprintf(&unit, "%s = %s=>parse( %s ).\n", value, names.Get("runtime.jsonSubset"), variable)
		return value + "->sval"
	}
	literal := func(s string) string {
		encoded, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return literalJSON(string(encoded))
	}
	invoke := func(m string, params ...string) string {
		return owner + "=>" + names.Get("member."+m) + "( " + strings.Join(params, " ") + " )"
	}
	param := func(k, v string) string { return names.Get("param."+k) + " = " + v }
	equal := func(call, want, msg string) {
		fmt.Fprintf(&unit, "actual = %s.\ncl_abap_unit_assert=>assert_equals( act = actual exp = %s msg = '%s' ).\n", call, literal(want), msg)
	}
	for i, c := range pads {
		expected := literalJSON(string(answers.Pads[i]))
		fmt.Fprintf(&unit, "actual = %s.\ncl_abap_unit_assert=>assert_equals( act = actual exp = %s msg = 'pad %d' ).\n", invoke("pad", param("text", literal(c.Text)), param("target", "CONV f( '"+strconv.FormatFloat(c.Target, 'g', -1, 64)+"' )"), param("fill", literal(c.Fill))), expected, i)
		if i == 4 {
			unit.WriteString("DATA converter TYPE REF TO cl_abap_conv_out_ce.\nDATA bytes TYPE xstring.\ncl_abap_unit_assert=>assert_equals( act = strlen( actual ) exp = 4 ).\npart = actual+2(1).\nconverter = cl_abap_conv_out_ce=>create( encoding = '4103' ).\nconverter->convert( EXPORTING data = part IMPORTING buffer = bytes ).\ncl_abap_unit_assert=>assert_equals( act = bytes exp = CONV xstring( '3DD8' ) ).\n")
		}
	}
	for i, s := range floats {
		fmt.Fprintf(&unit, "numeric = %s.\nexpected = '%s'.\ncl_abap_unit_assert=>assert_equals( act = numeric exp = expected msg = 'float %d' ).\n", invoke("float", param("text", literal(s))), strconv.FormatFloat(answers.Floats[i], 'g', -1, 64), i)
	}
	for i, s := range hashes {
		equal(invoke("sha", param("text", literal(s))), answers.Hashes[i], fmt.Sprintf("sha %d", i))
	}
	splitCase := 0
	for _, p := range patterns {
		fmt.Fprintf(&unit, "CREATE OBJECT pattern EXPORTING pattern = `%s` flags = 'g'.\npattern->global = abap_true.\n", p)
		for _, s := range inputs {
			input := literal(s)
			for _, limit := range limits {
				fmt.Fprintf(&unit, "pieces = %s.\ncl_abap_unit_assert=>assert_equals( act = lines( pieces->items ) exp = %d msg = 'split length %d' ).\n", invoke("split", param("text", input), param("pattern", "pattern"), param("limit", "CONV f( '"+strconv.FormatFloat(limit, 'f', -1, 64)+"' )")), len(answers.Splits[splitCase]), splitCase)
				for j, want := range answers.Splits[splitCase] {
					exp := literal(want)
					fmt.Fprintf(&unit, "READ TABLE pieces->items INDEX %d INTO part.\ncl_abap_unit_assert=>assert_equals( act = part exp = %s msg = 'split %d item %d' ).\n", j+1, exp, splitCase, j)
				}
				splitCase++
			}
		}
		unit.WriteString("cl_abap_unit_assert=>assert_true( act = pattern->global ).\n")
	}
	fmt.Fprintf(&unit, "CREATE OBJECT pattern EXPORTING pattern = `\\s+` flags = ''.\npieces = %s.\ncl_abap_unit_assert=>assert_equals( act = lines( pieces->items ) exp = 2 msg = 'plain split flags' ).\nREAD TABLE pieces->items INDEX 1 INTO part.\ncl_abap_unit_assert=>assert_equals( act = part exp = `a` ).\nREAD TABLE pieces->items INDEX 2 INTO part.\ncl_abap_unit_assert=>assert_equals( act = part exp = `b` ).\ncl_abap_unit_assert=>assert_false( act = pattern->global ).\n", invoke("split", param("text", "`a b`"), param("pattern", "pattern"), param("limit", "CONV f( '4294967295' )")))
	for i, s := range xmls {
		fmt.Fprintf(&unit, "graph = %s.\n", invoke("xml", param("text", literal(s))))
		equal("writer->"+names.Get("json.write."+hir.T(hir.Dynamic).String())+"( graph )", answers.Xmls[i], fmt.Sprintf("xml %d", i))
	}
	for i, s := range errors {
		fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\ngraph = %s.\nCATCH %s INTO fault.\nrefused = abap_true.\nactual = fault->payload->%s.\nENDTRY.\ncl_abap_unit_assert=>assert_true( act = refused msg = 'XML Error %d' ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = %s ).\n", invoke("xml", param("text", literal(s))), names.Get("exception."+hir.Ref("builtin.Error").String()), names.Get("member.message"), i, literal(answers.Errors[i]))
	}
	for _, s := range []string{"", "NaN", "Infinity", "-Infinity", "1e9999", "-0", "-1e-9999", "."} {
		fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nnumeric = %s.\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( act = refused ).\n", invoke("float", param("text", literal(s))), names.Get("exception.StringOpRefusal"))
	}
	for _, s := range []string{`<a>&unknown;</a>`, `<a><![CDATA[x]]></a>`, `<!DOCTYPE a><a/>`, `<__proto__/>`, `<?XML version="1.0"?><a/>`} {
		fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\ngraph = %s.\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( act = refused ).\n", invoke("xml", param("text", literal(s))), names.Get("exception.RegistryXMLSubsetError"))
	}
	// Preserve lone surrogates in the input, too. Go's JSON string decoding
	// would turn these into U+FFFD and accidentally weaken the boundary test.
	for _, raw := range []string{`"\ud800"`, `"\udc00"`, `"\ud800x\udfff"`} {
		fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nactual = %s.\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( act = refused msg = 'SHA1 lone surrogate refusal' ).\n", invoke("sha", param("text", literalJSON(raw))), names.Get("exception.StringOpRefusal"))
	}
	fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nactual = %s.\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( act = refused msg = 'pad allocation limit' ).\n", invoke("pad", param("text", "`a`"), param("target", "CONV f( '1048577' )"), param("fill", "`x`")), names.Get("exception.StringOpRefusal"))
	for _, p := range []string{`(a)`, `(?:)`, `\s*`, `[a-z]+`} {
		fmt.Fprintf(&unit, "CREATE OBJECT pattern EXPORTING pattern = `%s` flags = ''.\nCLEAR refused.\nTRY.\npieces = %s.\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( act = refused msg = 'split unchecked grammar' ).\n", p, invoke("split", param("text", "`a b`"), param("pattern", "pattern"), param("limit", "CONV f( '4294967295' )")), names.Get("exception.StringOpRefusal"))
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
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
	t.Logf("Node oracles: pad %d, float %d, SHA1 %d, split %d, XML %d values / %d Errors", len(pads), len(floats), len(hashes), splitCase, len(xmls), len(errors))
}
