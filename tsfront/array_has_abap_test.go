package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// Presence-checked callbacks (forEach/every/find/lastIndexOf) emit array.has.
// abaplint test files register every mocha case from such a forEach, so a
// missing ABAP emitter left whole describe bodies unregistered on OSGO.
// ABAP emits has as a bounds check and keeps its arrays dense: every
// hole-creating write refuses before padding, never fabricating a slot.
func TestArrayHasABAPNodeOracle(t *testing.T) {
	source := `class Probe {
 static run():string {
  const xs=["a","b","c"];let out="";
  xs.forEach((x,i)=>{out+=x+i;if(i===0){xs.push("d");}});
  const cases=[{abap:"IF.",n:1},{abap:"ENDIF.",n:2}];
  cases.forEach(c=>{out+="|"+c.abap+c.n;});
  out+="|"+xs.every(x=>x.length===1)+"|"+[1,2,1,3].lastIndexOf(1)+"|"+[4,5].lastIndexOf(6);
  const shrink=["p","q","r"];shrink.forEach(s=>{out+=s;shrink.pop();});
  const tail:string[]=[];tail[0]="t";tail.forEach(s=>{out+=s;});
  return out;
 }
 static sparse():string {
  const xs:string[]=[];xs[2]="x";let out="";
  xs.forEach(x=>{out+=x;});
  return out+xs.length;
 }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run()+'/'+Probe.sparse())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	expected := strings.TrimSpace(string(want))
	run, sparse, ok := strings.Cut(expected, "/")
	if !ok || sparse != "x3" {
		t.Fatalf("unexpected Node observation %q", expected)
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if len(hir.CheckRuntimeOps(p, "probe", func(op string) bool { return op != "array.has" })) == 0 {
		t.Fatal("probe no longer exercises array.has")
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Printf(\"%%v/%%v\\n\", %s(), %s())", goEntry("probe.ts.Probe", "run"), goEntry("probe.ts.Probe", "sparse"))); strings.TrimSpace(got) != expected {
		t.Fatalf("Go %q Node %q", got, expected)
	}
	files, names, err := abap.EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	class := names.Get("probe.ts.Probe")
	src := files[class+".clas.abap"]
	refusal := names.Get("exception.chaiRefusal")
	gate, pad := strings.Index(src, "RAISE EXCEPTION TYPE "+refusal), strings.Index(src, "WHILE lines(")
	if gate < 0 || pad < 0 || gate > pad || files[refusal+".clas.abap"] == "" {
		t.Fatal("presence-checked programs must refuse sparse writes before dense padding")
	}
	files[class+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_has DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS dense FOR TESTING.
METHODS sparse FOR TESTING.
ENDCLASS.
CLASS ltcl_has IMPLEMENTATION.
METHOD dense.
cl_abap_unit_assert=>assert_equals( act = %s=>%s( ) exp = `+"`%s`"+` ).
ENDMETHOD.
METHOD sparse.
DATA refused TYPE abap_bool.
TRY.
%s=>%s( ).
CATCH %s.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
ENDMETHOD.
ENDCLASS.
`, class, names.Get("member.run"), run, class, names.Get("member.sparse"), refusal)
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		out = filepath.Join(out, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			formatted, err := abap.Format(content)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name), []byte(formatted), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
