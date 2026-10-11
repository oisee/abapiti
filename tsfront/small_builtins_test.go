package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

// Every case is lowered, verified, emitted to ABAP, and executed as Go against
// Node. syntaxfeatures also carries these operations through the ABAP oracle.
func TestSmallBuiltinsNodeOracle(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node absent")
	}
	cases := []struct{ name, source string }{
		{"trim", `class Probe { static run() { const s="\uFEFF \t\nA\u00A0\u3000"; return s.trimStart()+"|"+s.trimEnd()+"|"+" \t".trimEnd()+"|"+"😀 ".trimEnd(); } }`},
		{"locale-case", `class Probe { static run() { return "ABCΣİ".toLocaleLowerCase()+"|"+"ßﬀabc".toLocaleUpperCase(); } }`},
		{"flat", `class Probe { static run() { const a: number[][]=[[1,2],[],[3]]; const b=a.flat(1); b.push(4); return b.join(",")+"|"+a.length+"|"+a[0].length; } }`},
		{"flat-identity", `class Box { value=1; } class Probe { static run() { const a=[[new Box()]]; const b=a.flat(); b[0].value=7; const c=[1,2].flat(); c.push(3); return a[0][0].value+"|"+a[0].length+"|"+c.join(","); } }`},
		{"map-delete", `class Probe { static run() { const m=new Map<string,number>(); m.set("a",0); m.set("b",2); const a=m.delete("a"); const b=m.delete("a"); m.set("a",3); return a+"|"+b+"|"+m.size+"|"+m.get("a"); } }`},
		{"updates", `class Box { value=0; } class Probe { static calls=0; static box=new Box(); static receiver() { Probe.calls++; return Probe.box; } static run() { let n=0; const a=++n; const b=n++; ++n; const c=++Probe.receiver().value; const d=Probe.receiver().value++; return a+"|"+b+"|"+n+"|"+c+"|"+d+"|"+Probe.box.value+"|"+Probe.calls; } }`},
		{"labels", `class Probe { static run() { let sum=0; outer: for(let i=0;i<5;i++){ for(let j=0;j<3;j++){ for(let k=0;k<2;k++){ if(i===1){continue outer;} if(i===4){break outer;} sum++; } sum+=10; } sum+=100; } return sum; } }`},
		{"labels-forof", `class Probe { static run() { let sum=0; outer: for(const i of [0,1,2,3]) { for(const j of [0,1,2]) { if(i===1){continue outer;} if(i===3){break outer;} sum++; } sum+=10; } return sum; } }`},
		{"labels-direct", `class Probe { static run() { let sum=0; outer: for(let i=0;i<5;i++){ if(i===1){continue outer;} if(i===4){break outer;} sum+=i; } let i=0; again: while(i<3){i++;for(let j=0;j<3;j++){if(i===2){continue again;}sum++;}} return sum; } }`},
		{"nullish-nonoptional", `class Probe { static count=0; static rhs() { Probe.count++; return 99; } static run() { const zero=0; const no=false; const empty=""; const a=zero??Probe.rhs(); const b=no??true; const c=empty??"fallback"; return a+"|"+b+"|"+c+"|"+Probe.count; } }`},
		{"in-record", `class Probe { static run() { const r: {[key:string]:number}={zero:0}; return ("zero" in r)+"|"+("missing" in r); } }`},
		{"in-shape", `class Base { value=0; } class Child extends Base { other=false; } class Probe { static run() { const c=new Child(); return ("value" in c)+"|"+("other" in c)+"|"+("missing" in c); } }`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			js := strings.NewReplacer(": number[][]", "", "<string,number>", "", ": {[key:string]:number}", "").Replace(c.source)
			source := c.source
			want, err := exec.Command("node", "-e", js+`process.stdout.write(String(Probe.run())+"\n")`).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(want))
			}
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			if _, err := abap.Emit(p); err != nil {
				t.Fatal(err)
			}
			got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
			if got != string(want) {
				t.Fatalf("Go %q Node %q", got, want)
			}
		})
	}
}
