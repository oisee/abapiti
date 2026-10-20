package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

const regexpOptionalCapturesSource = `class Probe {
 static run(): string {
  const re = /^(&?)([\w\/~]+)(?:(->|-)([\w]+))?:(\w+)$/;
  const specs = ["&P_OBJECT:ref", "P_OBJECT->COMP:val", "X-Y:z"];
  const out: string[] = [];
  for (const spec of specs) {
   const m = re.exec(spec)!;
   const [, ref, name, sel, comp = "default", kind] = m;
   out.push(ref + ":" + name + ":" + (sel ?? "missing") + ":" + comp + ":" + kind);
   out.push("index:" + String(m[3] === undefined) + ":" + String(sel === "->") + ":" + String(sel === "-"));
   out.push("mapped:" + m.map(value => value ?? "missing").join("/"));
   for (const value of m) out.push("iterated:" + String(value ?? "missing"));
  }
  const matched = "a".match(/(a)(b)?/)!;
  const named = /(?<letter>a)(?<missing>b)?/.exec("a")!;
  const groups = named.groups!;
  return out.join("|") + "|" + JSON.stringify(matched) + "|" + groups.letter + ":" + (groups.missing ?? "missing");
 }
}`

func TestRegExpOptionalCapturesStrictTSNodeOracle(t *testing.T) {
	want := regexpOptionalCapturesNode(t, regexpOptionalCapturesSource)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": regexpOptionalCapturesSource}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	storageOracleABAP(t, p, want)
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestRegExpOptionalCapturesJSDocMjsNodeOracle(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"types.ts":      "export type Unused = string;\n",
		"probe.mjs":     regexpOptionalCapturesSource,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","allowJs":true,"checkJs":true,"noEmit":true},"files":["types.ts","probe.mjs"]}`,
	}
	for file, content := range files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	project, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, diags, err := project.Lower([]string{"probe.mjs"})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range diags {
		if !strings.HasPrefix(diagnostic.Category, "note-") {
			t.Error(diagnostic)
		}
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	want := regexpOptionalCapturesNode(t, regexpOptionalCapturesSource)
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.mjs.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageOracleABAP(t, p, want)
}

func TestRegExpOptionalCapturesDoNotWeakenTypedArrays(t *testing.T) {
	source := `class Probe {
 static required(values: string[]): string {const [first] = values;return first;}
 static run(): string {return Probe.required([]);}
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("ordinary array destructuring must refuse absent scalar")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "run"))
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
}

func regexpOptionalCapturesNode(t *testing.T, source string) string {
	t.Helper()
	return conversionOracle(t, source)
}

// A continue in a for-of over a match array must still advance the index,
// including when it leaves a try/finally. The watchdog turns a hang into a
// failure.
func TestRegExpMatchArrayForOfContinueNodeOracle(t *testing.T) {
	source := `class Probe {
 static run(): string {
  const m = /(a)(b)?(c)/.exec("ac")!;
  let out = "";
  for (const g of m) { if (g === undefined) continue; out += g; }
  let fin = "";
  for (const g of m) { try { if (g === undefined) continue; fin += g; } finally { fin += "|"; } }
  let stop = "";
  for (const g of m) { try { if (g === undefined) continue; stop += g; } finally { if (g === undefined) break; } }
  return out + ":" + fin + ":" + stop;
 }
}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(watchdog(%s))", goEntry("probe.ts.Probe", "run")), func(files map[string]string) {
		files["zz_watchdog.go"] = "package main\nimport (\"os\";\"time\")\nfunc watchdog[T any](f func() T) T {done:=make(chan T,1);go func(){done<-f()}();select{case s:=<-done:return s;case <-time.After(20*time.Second):os.Stdout.WriteString(\"timeout\\n\");os.Exit(3)};var zero T;return zero}\n"
	})
	storageOracleABAP(t, p, want)
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
