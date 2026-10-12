package golang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func json5PackagePath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	core := os.Getenv("TSFRONT_ABAPLINT")
	if core == "" {
		core = filepath.Join(home, "dev/abaplint/packages/core")
	}
	pkg := filepath.Join(core, "node_modules/json5")
	if _, err := os.Stat(filepath.Join(pkg, "package.json")); err != nil {
		t.Skip("requires installed original json5 package")
	}
	return pkg
}

func TestJSON5PackageNodeOracle(t *testing.T) {
	pkg := json5PackagePath(t)
	cases := []string{
		`{ // comment\n syntax: {version: 'v702',}, rules: {},}`,
		"{ // a comment\n \"global\": {\"files\": \"src/*\",}, \"rules\": {},}",
		`{unquoted:'single', $a:0xFF, _b:+.5, decimal:1., exponent:-2.e+2, trailing:[1,2,],}`,
		"/*c*/['a\\\nb','x\\\r\ny','z\\\u2028w',]//eof",
		`{'10':1,'2':2,'01':3, __proto__: {x:1}, constructor:4, a:1,a:2}`,
		`{'escape':'\x41\uD800\v\0\q\"\'\\',\u0061:1,a\u0030:2}`,
		"\ufeff\u00a0\v\f\u2003[true,false,null,-0,1e-400,0X123456789abcdef0123456789]",
		`"raw` + string(rune(1)) + `control"`,
		`''`, `0`, `-0`, `+.123`, `1.e2`, `0x20000000000001`, `-0x0`,
		`{é:1, 中文:2, 𐐀:3, á:4, a‌:5, \u03b1:6}`, `{😀:1}`, `{a😀:1}`, `{\ud801\udc00:1}`, `{\u200c:1}`, `{\u0300:1}`,
		"['raw\u2028separator']",
		"", " ", "/* unterminated", "/x", "{", "[", "{a", "{a:", "{a:1", "[1", "{a:1,,}", "[1,,]", "{1:2}", "{a 1}", "[01]", "01", "1e", "1e+", ".", "+.", "0x", "0xg", "truX", "undefined", "{}x", "{'x':'unterminated}", "'\\1'", "'\\01'", "'\\x1z'", "'\\u00zz'", "{\\u0030:1}", "{a\\u002d:1}", "{\\x61:1}", "'a\nb'", "[\n 1,\n !]", "[\r\n@]", "\"😀\" !", "Infinity!", "NaN,",
	}
	// Replace only the literal newline in the first comment fixture.
	cases[0] = strings.ReplaceAll(cases[0], `\n`, "\n")
	script := `const p=require(process.argv[1]);if(require(process.argv[1]+"/package.json").version!=="2.2.3")throw Error("pin");for(const s of JSON.parse(process.argv[2])){try{const d=p.parse(s);console.log("value:"+JSON.stringify(d))}catch(e){console.log(e.name+":"+e.message)}}`
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "-e", script, pkg, string(data)).Output()
	if err != nil {
		t.Fatal(err, string(out))
	}
	want := string(out)
	main := "for _,s:=range []string{"
	for _, s := range cases {
		main += strconv.Quote(s) + ","
	}
	main += `} {func(){defer func(){if x:=recover();x!=nil {if message,ok:=x.(jsString);ok {fmt.Println("SyntaxError:"+message.String())}else{panic(x)}}}();d:=parseJSON5(str(s),func(message jsString){panic(message)});fmt.Println("value:"+stringifyConfig(d).String())}()}`
	got := execute(t, configJSONFixture(), main)
	if got != want {
		t.Fatalf("JSON5 differential mismatch:\nGo:\n%s\nNode:\n%s", got, want)
	}
}

func TestJSON5NonfiniteAndABIRefusals(t *testing.T) {
	pkg := json5PackagePath(t)
	out, err := exec.Command("node", "-e", `const p=require(process.argv[1]);if(require(process.argv[1]+"/package.json").version!=="2.2.3")throw Error("pin");function nf(v){return typeof v==='number' ? !Number.isFinite(v) : v && typeof v==='object' && Object.values(v).some(nf)};for(const s of ['NaN','+NaN','-NaN','Infinity','+Infinity','-Infinity','1e9999','{x:NaN}','[Infinity]'])console.log(nf(p.parse(s)));`, pkg).CombinedOutput()
	if err != nil || string(out) != strings.Repeat("true\n", 9) {
		t.Fatal("nonfinite Node oracle", err, string(out))
	}
	p := configJSONFixture()
	p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "parse5", Static: true, Params: []hir.Param{{Name: "s", Type: hir.T(hir.String)}}, Result: hir.T(hir.Dynamic), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "json5.parse", Type: hir.T(hir.Dynamic), X: hir.V("s", hir.T(hir.String))}})})
	if _, err := abap.Emit(p); err == nil {
		t.Fatal("ABAP must refuse JSON5")
	}
	main := `for _,s:=range []string{"NaN","+NaN","-NaN","Infinity","+Infinity","-Infinity","1e9999","{x:NaN}","[Infinity]"} {func(){defer func(){x:=recover();if _,ok:=x.(trap);!ok {panic("nonfinite must be a typed refusal")};fmt.Println(x)}();parseJSON5(str(s),func(message jsString){panic(message)});panic("escaped nonfinite")}()};`
	out, err = exec.Command("node", "-e", `const p=require(process.argv[1]);for(const s of ['-0','-0x0','-0.0','-1e-9999'])console.log(Object.is(p.parse(s),-0));`, pkg).CombinedOutput()
	if err != nil || string(out) != strings.Repeat("true\n", 4) {
		t.Fatal("signed-zero Node oracle", err, string(out))
	}
	main += `for _,s:=range []string{"-0","-0x0","-0.0","-1e-9999"} {v:=parseJSON5(str(s),func(message jsString){panic(message)}).Value.(float64);fmt.Println(1/v<0)}`
	if got := execute(t, p, main); got != strings.Repeat("JSON5 nonfinite Number representation is unsupported\n", 9)+strings.Repeat("true\n", 4) {
		t.Fatal(got)
	}
}
