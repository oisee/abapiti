package tsfront

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestNativeJSONCreationOrderNodeOracle(t *testing.T) {
	source := `class Base {z="base"; "10"=10; "2"=2; later?:string; tail="tail"; #hidden=7; private visible="private-keyword"; static notOwn="static"; constructor(){this.later="constructor";}}
class Derived extends Base {a="derived"; "01"=1; "😀"="unicode-key"; constructor(){super();this.z="updated";}}
class Empty {}
class UndefinedField {first:number;second=1;constructor(){this.first=2;}}
class Probe {
 static calls=0;
 static receiver(x:Derived):Derived {Probe.calls++;return x;}
 static run():string {const x=new Derived();Probe.receiver(x).later="assigned";return JSON.stringify(x)+"/"+JSON.stringify(new Empty())+"/"+Probe.calls;}
 static refused():string {return JSON.stringify(new UndefinedField());}
}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	// This source creates first as undefined before second, even though its
	// later required-storage assignment runs after second's initializer.
	// Keep Dell's undefined-storage handoff explicit rather than reorder it.
	out, err := exec.Command("node", "-e", `class C {first;second=1;constructor(){this.first=2}}console.log(JSON.stringify(new C()))`).CombinedOutput()
	if err != nil || string(out) != "{\"first\":2,\"second\":1}\n" {
		t.Fatal(err, string(out))
	}
	got = runGoHIR(t, p, fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok{panic("undefined define-field order escaped")};fmt.Print("refused")}();%s()`, goEntry("probe.ts.Probe", "refused")))
	if got != "refused" {
		t.Fatal(got)
	}
}

func TestJSONSyntaxErrorNodeOracle(t *testing.T) {
	source := `class Probe {
 static parse(s:string):string {try {return JSON.stringify(JSON.parse(s));}catch(e){return (e instanceof SyntaxError)+":"+(e instanceof Error)+":"+e.constructor.name+":"+e.message;}}
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	inputs := []string{"sdfsdf", "x", "", " \r\n\t", "undefined", "NaN", "Infinity", "foo", "nul", "nux", "[x]", "[1,]", `{"a":}`, "+1", ".2", "null", `{"x":1,"x":2,"__proto__":3}`, `[-0,1e-300,"\ud800","😀"]`}
	var main strings.Builder
	for _, s := range inputs {
		fmt.Fprintf(&main, "fmt.Println(%s(str(%s)))\n", goEntry("probe.ts.Probe", "parse"), strconv.Quote(s))
	}
	got := runGoHIR(t, p, main.String())
	script := `for(const s of JSON.parse(process.argv[1])){try{console.log(JSON.stringify(JSON.parse(s)))}catch(e){console.log((e instanceof SyntaxError)+":"+(e instanceof Error)+":"+e.constructor.name+":"+e.message)}}`
	// Preserve source UTF-16 escapes by passing each JSON text as a JS string.
	var list []string
	for _, s := range inputs {
		list = append(list, strconv.Quote(s))
	}
	out, err := exec.Command("node", "-e", script, "["+strings.Join(list, ",")+"]").CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != string(out) {
		t.Fatalf("Go %q Node %q", got, out)
	}
	// Unsupported diagnostics/nonfinite conversion are still traps, not caught
	// SyntaxErrors (valid nonfinite JSON numbers are a value-domain ABI boundary).
	for _, s := range []string{`{"a":1,}`, "1e9999", strings.Repeat("x", 40)} {
		got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok{panic("unsupported domain was caught")};fmt.Print("refused")}();%s(str(%s))`, goEntry("probe.ts.Probe", "parse"), strconv.Quote(s)))
		if got != "refused" {
			t.Fatal(s, got)
		}
	}
}
