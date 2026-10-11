package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func TestRulesXMLValidatorGoMatchesNode(t *testing.T) {
	source := `function external(xml: string): any { return true; }
export class Probe {
 static validate(xml: string): any { return external(xml); }
 static issue(xml: string): string {
  const res = Probe.validate(xml);
  if (res !== true) { return "XML parser error: " + res.err.msg; }
  return "";
 }
}`
	span := source[strings.Index(source, "export class Probe"):]
	registry, err := overrides.New(overrides.Entry{ID: "xml-validator-probe", Key: overrides.Key{File: "input.ts", Symbol: "Probe", Kind: "KindClassDeclaration"}, SHA256: overrides.Fingerprint(span), Rationale: "exact external validation result/message adapter used by XMLConsistency.run", Patterns: &overrides.Patterns{Expressions: map[string]func() *hir.Expr{
		"external(xml)": func() *hir.Expr { return overrides.XMLValidationResult(hir.V("xml", hir.T(hir.String))) },
		"res !== true":  func() *hir.Expr { return overrides.XMLValidationFailed(hir.V("res", hir.T(hir.Dynamic))) },
		"res.err.msg":   func() *hir.Expr { return overrides.XMLValidationMessage(hir.V("res", hir.T(hir.Dynamic))) },
	}}})
	if err != nil {
		t.Fatal(err)
	}
	dir := sourceProbe(t, source)
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"input.ts"}, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	cases := []string{"", " ", "x", "<", "<>", "< a/>", "<1/>", "<a>", "<a><b>", "<a></b>", "</a>", "<a></a>", "<a/><b/>", "<a></a><b/>", "<a></a><b></b>", "<a></a> x", "<a/>", "<a/ >", "<a", "<a/", "<a><b/>", "<a>&amp;</a>", "<a>&oops</a>", "<a>&#;</a>", "<a>&#x;</a>", "<a>&#z;</a>", "<a>&;</a>", "<a a='v'/>", "<a a='v' a='w'/>", "<a a='v'b='w'/>", "<a a=bad/>", "<a boolean/>", "<a a='open>", "<a 1bad='x'/>", "<a __proto__='x' toString='y'/>", "<a a='>'/>", "<a></a x='y'>", "<a></a", "<?xml version='1.0'?><a/>", "<a><?xml version='1.0'?></a>", "<!--comment--><a/>", "<!DOCTYPE a [<!ENTITY foo 'bar'>]><a>&foo;</a>", "<a><![CDATA[<&text]]></a>", "<a><!--open", "\ufeff<a/>", "<é 名='字'/>", "<𐀀/>", "<a>😀\r\n<b></c></a>", "<a>\r\n<b>\n<c>", "<a a='😀' a='again'/>", "<abapGit version='v1.0.0'><asx:abap xmlns:asx='http://www.sap.com/abapxml'><asx:values/></asx:abap></abapGit>"}
	upstreamRaw, err := os.ReadFile("testdata/xml-validator-rule-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var upstream []string
	if err := json.Unmarshal(upstreamRaw, &upstream); err != nil {
		t.Fatal(err)
	}
	cases = append(cases, upstream...)
	// Deterministic deletions/insertions exercise malformed input without
	// requiring npm or network for the oracle.
	for _, seed := range []string{`<a x="1"><b/>text&amp;</a>`, "<a>\r\n<b y='v'></b>\n</a>"} {
		for i := 0; i < len(seed); i++ {
			cases = append(cases, seed[:i]+seed[i+1:])
			for _, c := range []string{"<", ">", "'", "&", " ", "="} {
				cases = append(cases, seed[:i]+c+seed[i:])
			}
		}
	}
	raw, _ := json.Marshal(cases)
	input := filepath.Join(t.TempDir(), "cases.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	oracle, _ := filepath.Abs("../hir/golang/testdata/xml-validator/validator.mjs")
	script := `import {validate} from ` + fmt.Sprintf("%q", oracle) + `;import fs from 'node:fs';const cases=JSON.parse(fs.readFileSync(process.argv[1]));console.log(JSON.stringify(cases.map(xml=>{const result=validate(xml);return {result,issue:result===true?'':'XML parser error: '+result.err.msg}})));`
	out, err := exec.Command("node", "--input-type=module", "-e", script, input).CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v: %s", err, out)
	}
	var want []any
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatal(err)
	}
	main := `var cases []string;b,err:=os.ReadFile(` + fmt.Sprintf("%q", input) + `);if err!=nil {panic(err)};json.Unmarshal(b,&cases);out:=[]any{};for _,xml:=range cases {d:=` + goEntry("input.ts.Probe", "validate") + `(str(xml));var result any=true;if !dynEqual(d,box(true)) {e:=d.get(str("err"));fields:=map[string]any{"code":dynString(e.get(str("code"))).String(),"msg":dynString(e.get(str("msg"))).String(),"line":dynNumber(e.get(str("line")))};if col:=e.get(str("col"));col!=nil {fields["col"]=dynNumber(col)};result=map[string]any{"err":fields}};out=append(out,map[string]any{"result":result,"issue":` + goEntry("input.ts.Probe", "issue") + `(str(xml)).String()})};b,_=json.Marshal(out);fmt.Print(string(b))`
	var got []any
	if err := json.Unmarshal([]byte(runGoHIR(t, prog, main)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatal("count mismatch")
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("XML %q: Go %#v, Node %#v", cases[i], got[i], want[i])
			if t.Failed() && i > 100 {
				break
			}
		}
	}
	t.Logf("%d validator and issue observations match Node", len(cases))
}
