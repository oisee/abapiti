package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/abap"
)

func TestSmallRootRefusals(t *testing.T) {
	cases := []struct{ body, category, reason string }{
		{`const a = [1]; return a.map(x => { while (x > 0) { return x; } return 0; }).join(",");`, "unsupported-callback", "completion ABI"},
		{`function f(n: number): number { return f(n); } return f(1).toString();`, "unsupported-local-function", "recursive capture ABI"},
		{`function f(): string { return this.toString(); } return f();`, "unsupported-local-function", "dynamic call-receiver"},
		{`if (true) { function f(): string { return "x"; } return f(); } return "";`, "unsupported-local-function", "lexical function-binding"},
		{`return parseFloat("1.5xyz").toString();`, "unsupported-call", "longest-prefix"},
		{`return ((x: string) => x)("x");`, "unsupported-call", "zero parameters"},
	}
	for _, c := range cases {
		t.Run(c.reason, func(t *testing.T) {
			dir := t.TempDir()
			for f, s := range map[string]string{"input.ts": `export class Probe { static run(): string { ` + c.body + ` } }`, "tsconfig.json": `{"compilerOptions":{"strict":true},"files":["input.ts"]}`} {
				if err := os.WriteFile(filepath.Join(dir, f), []byte(s), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, diags := lowerProbe(t, dir)
			for _, d := range diags {
				if d.Category == c.category && strings.Contains(d.Message, c.reason) {
					return
				}
			}
			t.Fatal(diags)
		})
	}
}

func TestInlineCallbackReturnBoundary(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
 static run(): string {
  const a = [3,1,2];
  a.sort((x,y) => { const diff = x-y; if (diff !== 0) { return diff; } return 0; });
  const b = a.map(x => { if (x === 2) { return 9; } return x; });
  let visits = 0;
  const n = (() => {
    if (false) return (() => { visits += 1; return 99; })();
    const x = 4; if (x > 0) { return x; } return 0;
  })();
  return b.join(",") + ":" + n + ":" + visits;
 }
 }`}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run")))
	if strings.TrimSpace(got) != "1,9,3:4:0" {
		t.Fatal(got)
	}
}

func TestStringArrayCoercionAndLastIndexOf(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe { static run(): string {
  const a = ["", "x", "", "y"];
  const last = a.lastIndexOf("");
  const missing = a.lastIndexOf("missing");
  const empty: string[] = [];
  return "a=" + a + ":" + last + ":" + missing + ":" + empty;
 } }`}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run")))
	if strings.TrimSpace(got) != "a=,x,,y:2:-1:" {
		t.Fatal(got)
	}
}

func TestNestedFunctionDeclaration(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
 static run(): string {
  const positions: number[] = [1];
  if (!has(2)) { positions.push(2); }
  function has(element: number): boolean { return positions.some(a => a === element); }
  if (!has(2)) { positions.push(2); }
  return positions.join(",");
 }
 }`}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run")))
	if strings.TrimSpace(got) != "1,2" {
		t.Fatal(got)
	}
}
