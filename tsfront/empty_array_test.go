package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestEmptyArrayExpressionContexts(t *testing.T) {
	source := `
export function returned(): string[] { return []; }
export function defaulted(xs: string[] = []): number { xs.push("x"); return xs.length; }
function size(xs: string[]): number { return xs.length; }
export function argument(): number { return size([]); }
export function conditional(flag: boolean): string[] { return flag ? [] : ["x"]; }
export function optional(xs?: string[]): string[] { return xs ?? []; }
export function tagged(): any { return []; }
export function taggedIsArray(): boolean { return Array.isArray(tagged()); }
export function jsonFallback(raw: string): number {
  const parsed = JSON.parse(raw);
  let count = 0;
  for (const value of parsed.values ?? []) { count += 1; }
  return count;
}

export function probe(): string {
  const first = returned(); first.push("one");
  return first.length + "," + returned().length + "," + defaulted() + "," + defaulted()
    + "," + argument() + "," + conditional(true).length + "," + conditional(false).length
    + "," + optional().length + "," + taggedIsArray() + "," + jsonFallback("{}")
    + "," + jsonFallback('{"values":["ab","c"]}');
}
`
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if t.Failed() {
		return
	}
	if _, err := emitABAP(t, prog); err != nil {
		t.Fatal(err)
	}
	node := exec.Command("node", "--input-type=module-typescript")
	node.Stdin = strings.NewReader(source + "\nconsole.log(probe());")
	want, err := node.CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v\n%s", err, want)
	}
	if got := runGoHIR(t, prog, "fmt.Println("+goEntry("probe.ts.module", "probe")+"().String())"); got != string(want) {
		t.Fatalf("Go %q; Node %q", got, want)
	}
	main := `defer func(){if _,ok:=recover().(rangeFault);!ok{panic("non-array iterable must refuse")};fmt.Println("refused")}();` + goEntry("probe.ts.module", "jsonFallback") + `(str("{\"values\":\"bad\"}"))`
	if got := runGoHIR(t, prog, main); got != "refused\n" {
		t.Fatal(got)
	}
}

func TestEmptyArrayWithoutElementContextRefuses(t *testing.T) {
	dir := t.TempDir()
	for name, source := range map[string]string{
		"input.ts":      `export function probe(): void { []; }`,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, diags := lowerProbe(t, dir)
	for _, d := range diags {
		if d.Category == "unsupported-array" && strings.Contains(d.Message, "inferable contextual element type") {
			return
		}
	}
	t.Fatalf("missing explicit empty-array refusal: %v", diags)
}

func TestEmptyArrayPreciseContextOverridesTaggedHint(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
export function precise(raw: string): number {
  const values: string[] = JSON.parse(raw).values ?? [];
  return values.length;
}
`}, []string{"probe.ts"})
	if !strings.Contains(hir.Dump(prog), "new array<string>") {
		t.Fatalf("precise string-array context lost: %s", hir.Dump(prog))
	}
}
