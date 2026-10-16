package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestReplaceGetSubstitutionNodeOracle(t *testing.T) {
	source := "class Probe {static run():string {\n" + `
 const inputs:string[]=['aba','😀a😀a','', 'a'+ '😀'.charAt(0)+'a'];
 const replacements:string[]=['$&','$$','$\x60',"$'",'$1:$01:$10:$99:$0:$00:$<name>','$&$$$&',"\\$&"];
 let out='';
 for(const s of inputs) for(const r of replacements) {
  out+=s.replace('a',r)+'|'+s.replaceAll('a',r)+'|'+s.replaceAll('',r)+';';
  out+=s.replace(/(a)(b)?/g,r)+'|'+s.replace(/(a)(b)?/,r)+';';
 }
 out+='b'.replace(/a*/g,'[$&]')+';'+'a'.replace(/a*/g,'[$&]')+';';
 out+=('x'.repeat(1500)+'aa').replace(/(a)/g,'$1$1').length+';';
 out+='aba'.replace(/(?<first>a)(?<second>b)?/g,'$<second>:$<first>:$<absent>:$1')+';';
 out+='ab'.replace(/(a)(b)/,'$2$1:$12:$02:$99')+';';
 out+='x+y'.replace(/[^\w]/g,"\\$&")+';';
 out+='-000123'.replace(/^(-?)0+(?=\d)/,'')+';'+'-000123'.replace(/^(-?)0+(?=\d)/,'$&')+';';
 const quotes:string[]=["'",'x'];for(const quote of quotes){const q=quote[0];out+='xx'.replaceAll(q+q,q)+';';}
 out+='a\rb\rc'.replace(/\r/g,'$&$&')+';'+'a\nb\nc'.replace(/\n/g,'_')+';';
 const stateful=/(a)/g;stateful.lastIndex=4;'aba'.replace(stateful,'$1');out+='idx='+stateful.lastIndex+';';
 out+='-000123'.replace(/^(-?)0+(?=\d)/,'$1');
 return JSON.stringify(out);
 }}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	// Exercise both digits all the way through capture 99.
	pattern := strings.Repeat("(a)", 99)
	source = strings.Replace(source, "return JSON.stringify(out);", "out+='"+strings.Repeat("a", 99)+"'.replace(/"+pattern+"/,'$99:$98:$09:$90:$100:$00');return JSON.stringify(out);", 1)
	want, err = exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceSubsetLocatedRefusals(t *testing.T) {
	for _, tc := range []struct{ body, reason string }{
		{`let r:RegExp=/a/;return 'a'.replace(r,()=> 'x');`, "literal RegExp"},
		{`return 'a'.replace(/a/,1 as any);`, "string replacement"},
		{`return 'a'.replaceAll(/a/,'x');`, "global RegExp"},
		{`const r:RegExp=/a/g;return 'a'.replaceAll(r,'x');`, "literal global RegExp"},
	} {
		ds := lowerFamilyDiagnostics(t, `class Probe {static run():string {`+tc.body+`}}`)
		found := false
		for _, d := range ds {
			if strings.Contains(d.Message, tc.reason) && d.Loc != "" {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing located %s refusal: %v", tc.reason, ds)
		}
	}
}

// Keep the critic's original dollar probes as executable regressions now that
// their former refusal is the feature under test.
func TestCriticReplacementSubstitutionNodeOracles(t *testing.T) {
	for _, name := range []string{"regex_replacement", "regex_sideeffect"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "critic-r1", name, "input.ts"))
			if err != nil {
				t.Fatal(err)
			}
			source := strings.Replace(string(raw), "export ", "", 1) + "\nclass Oracle {static run():string {return new Probe().run('aba');}}"
			want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Oracle.run())")`, source).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(want))
			}
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			got := runGoHIR(t, p, "fmt.Println("+goEntry("probe.ts.Oracle", "run")+"())")
			if got != string(want) {
				t.Fatalf("Go %q Node %q", got, want)
			}
			if _, err := emitABAP(t, p); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReplaceLiteralNativeLowering(t *testing.T) {
	for _, tc := range []struct{ call, op, statement string }{
		{`raw.replace(/\r/g, "")`, "string.replaceAll", "REPLACE ALL OCCURRENCES OF"},
		{`raw.replaceAll("a", "_")`, "string.replaceAll", "REPLACE ALL OCCURRENCES OF"},
		{"raw.replaceAll(\"a\", `_`)", "string.replaceAll", "REPLACE ALL OCCURRENCES OF"},
		{`raw.replace("a", "_")`, "string.replaceFirst", "REPLACE FIRST OCCURRENCE OF"},
	} {
		t.Run(tc.call, func(t *testing.T) {
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": `class Probe {static run(raw:string):string {return ` + tc.call + `;}}`}, []string{"probe.ts"})
			var method *hir.Method
			for _, c := range p.Classes {
				if c.Name == "probe.ts.Probe" {
					for _, m := range c.Methods {
						if m.Name == "run" {
							method = m
						}
					}
				}
			}
			if method == nil || len(method.Body.List) != 1 {
				t.Fatalf("expected a single native return: %s", hir.Dump(p))
			}
			ret := method.Body.List[0]
			if ret.Kind != hir.Return || ret.X == nil || ret.X.Kind != hir.RuntimeOp || ret.X.Op != tc.op || ret.X.X.Kind != hir.Local || ret.X.Args[0].Kind != hir.Lit || ret.X.Args[1].Kind != hir.Lit {
				t.Fatalf("expected native op without temporaries: %s", hir.Dump(p))
			}
			files, err := emitABAP(t, p)
			if err != nil {
				t.Fatal(err)
			}
			var code strings.Builder
			for _, contents := range files {
				code.WriteString(contents)
			}
			if !strings.Contains(code.String(), tc.statement) || strings.Contains(code.String(), "=>literal(") {
				t.Fatalf("expected native ABAP replacement: %s", code.String())
			}
		})
	}
}
