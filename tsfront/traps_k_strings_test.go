package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPadStartAndFiniteParseFloatNodeOracle(t *testing.T) {
	source := `export class Probe {
 static order: string = "";
 static recv(): string {Probe.order += "R"; return "x";}
 static size(): number {Probe.order += "N"; return 4.9;}
 static fill(): string {Probe.order += "F"; return "ab";}
 static parse(s: string): number {return parseFloat(s);}
 static contextual(): number | undefined {const d:any=JSON.parse('{"p":"10.0"}');const n:number|undefined=parseFloat(d.p || "");return n;}
 static pad(n: number): string {return "x".padStart(n,"0");}
 static run(): string {
  const effect = Probe.recv().padStart(Probe.size(),Probe.fill());
  const pads=["a".padStart(4,"0"),"a".padStart(4,"xy"),"a".padStart(2,"😀"),"😀".padStart(3,"x"),"x".padStart(4),"x".padStart(),"x".padStart(-3,"0"),"x".padStart(0,"0"),"x".padStart(1.9,"0"),"x".padStart(4,""),"x".padStart(1e100,""),effect,Probe.order];
  const numbers=[parseFloat(" 10.0tail"),parseFloat("\ufeff\u2007-.125e2x"),parseFloat("+1.e2!"),parseFloat("1e+"),parseFloat(".5suffix"),parseFloat("0xFF"),parseFloat("1_000"),parseFloat("-0"),parseFloat("1e-400"),parseFloat("9007199254740993"),Probe.contextual()!];
  return JSON.stringify(pads)+"/"+JSON.stringify(numbers);
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	dir := t.TempDir()
	path := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(path, []byte(source+"\nconsole.log(Probe.run());"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if got != strings.TrimSpace(string(out)) {
		t.Fatalf("Go %q Node %q", got, out)
	}
	// Compare nonfinite Number values with Node through JS string conversion.
	for _, s := range []string{"", "abc", "Infinity", "-Infinity", "1e400"} {
		out, err := exec.Command("node", "-e", `console.log(String(parseFloat(process.argv[1])))`, "--", s).CombinedOutput()
		if err != nil {
			t.Fatal(err)
		}
		want := strings.TrimSpace(string(out))
		if want != "NaN" && want != "Infinity" && want != "-Infinity" {
			t.Fatal(s, want)
		}
		got := runGoHIR(t, prog, "fmt.Print(numberString("+goEntry("input.ts.Probe", "parse")+"(str("+strconv.Quote(s)+"))).String())")
		if got != want {
			t.Fatal(s, got)
		}
	}
	got = runGoHIR(t, prog, "defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();fmt.Print("+goEntry("input.ts.Probe", "pad")+"(2000000))")
	if !strings.Contains(got, "padStart allocation exceeds") {
		t.Fatal(got)
	}
}

func TestPadStartAndParseFloatRefuseCoercion(t *testing.T) {
	for _, source := range []string{
		`export class Probe {run(x:any):string{return "x".padStart(x,"0");}}`,
		`export class Probe {run(x:any):string{return "x".padStart(3,x);}}`,
		`export class Probe {run(x:number):number{return parseFloat(x);}}`,
	} {
		requireDiagnostic(t, sourceProbe(t, source), "unsupported-call")
	}
}
