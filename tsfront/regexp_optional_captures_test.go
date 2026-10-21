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

// Each element of a match array is a group string or undefined: JS
// destructures the characters of the string (and throws on undefined), not
// the match array itself. That is refused rather than lowered to wrong values.
func TestRegExpMatchArrayForOfDestructuringRefused(t *testing.T) {
	for _, pattern := range []string{"[a, b]", "{length}"} {
		source := `class Probe {
 static run(): string {
  const m = /(ab)(cd)/.exec("abcd")!;
  let out = "";
  for (const ` + pattern + ` of m) out += String(` + strings.Trim(strings.Split(pattern, ",")[0], "[{}") + `);
  return out;
 }
}`
		if pattern == "[a, b]" {
			// Node: characters of each group, not the groups of the match.
			if got := conversionOracle(t, source); got != "aac\n" {
				t.Fatalf("Node %q", got)
			}
		}
		dir := t.TempDir()
		files := map[string]string{"probe.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`}
		for file, content := range files {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		project, err := Load(filepath.Join(dir, "tsconfig.json"))
		if err != nil {
			t.Fatal(err)
		}
		_, diags, err := project.Lower([]string{"probe.ts"})
		if err != nil {
			t.Fatal(err)
		}
		refused := false
		for _, diagnostic := range diags {
			if diagnostic.Category == "unsupported-binding" && strings.Contains(diagnostic.Message, "match array") {
				refused = true
			}
		}
		if !refused {
			t.Errorf("%s: destructuring a match-array element was not refused: %v", pattern, diags)
		}
	}
}

// Named groups take their capture-group number, counting unnamed groups but
// not escaped parentheses, class members or (?:. Lookarounds are refused for
// literals, so they reach the ABAP runtime only through its constructor.
const regexpNamedGroupNumbersSource = `class Probe {
 static named(re: RegExp, text: string, names: string[]): string {
  const groups = re.exec(text)!.groups!;
  return names.map(name => name + "=" + (groups[name] ?? "missing")).join(",");
 }
 static run(): string {
  return [
   Probe.named(/(a)(?<x>b)/, "ab", ["x"]),
   Probe.named(/(a)(?<x>b)(c)?(?<yy>d)/, "abd", ["x", "yy"]),
   Probe.named(/\(?<y>(?<z>c)/, "<y>c", ["y", "z"]),
   Probe.named(/[(?<w>]+(?<v>d)/, "w>d", ["w", "v"]),
   Probe.named(/[\]()](?<u>e)/, ")e", ["u"]),
   Probe.named(/(?:n)(?<p>q)(t)?/, "nq", ["p"]),
  ].join("|");
 }
}`

var regexpNamedGroupLookarounds = [][2]string{
	{`(?:n)(?<p>q)(?=r)(?!s)(t)?(?<g>u)?`, "nqr"},
	{`(?<=x)(?<s>y)(?<!z)(?<o>w)?`, "xy"},
}

func TestRegExpNamedGroupNumbersNodeOracle(t *testing.T) {
	want := regexpOptionalCapturesNode(t, regexpNamedGroupNumbersSource)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": regexpNamedGroupNumbersSource}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("probe.ts.Probe")
	var b strings.Builder
	b.WriteString("CLASS ltcl_named DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS oracle FOR TESTING.\n")
	for i := range regexpNamedGroupLookarounds {
		fmt.Fprintf(&b, "METHODS lookaround%d FOR TESTING.\n", i)
	}
	b.WriteString("ENDCLASS.\nCLASS ltcl_named IMPLEMENTATION.\nMETHOD oracle.\nDATA expected TYPE string.\nDATA ch TYPE c LENGTH 1.\n")
	for _, line := range abapStringBuild("expected", "ch", strings.TrimSuffix(want, "\n")) {
		b.WriteString(line + "\n")
	}
	fmt.Fprintf(&b, "DATA(actual) = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\nENDMETHOD.\n", owner, names.Get("member.run"))
	// The unchecked constructor path runs the runtime scanner, not the table.
	for i, c := range regexpNamedGroupLookarounds {
		fmt.Fprintf(&b, "METHOD lookaround%d.\nDATA expected TYPE string.\nDATA actual TYPE string.\nDATA ch TYPE c LENGTH 1.\nDATA regex TYPE REF TO %s.\n", i, names.Get("runtime.regexp_js"))
		raw, err := exec.Command("node", "-e", `const g=new RegExp(process.argv[1]).exec(process.argv[2]).groups;process.stdout.write(Object.entries(g).map(([k,v])=>k+"="+(v??"missing")).join(","))`, c[0], c[1]).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(raw))
		}
		for _, line := range abapStringBuild("expected", "ch", string(raw)) {
			b.WriteString(line + "\n")
		}
		fmt.Fprintf(&b, "CREATE OBJECT regex EXPORTING pattern = `%s` flags = ``.\nDATA(hit) = regex->exec( `%s` ).\nLOOP AT hit->match_groups->entries INTO DATA(row).\nIF sy-tabix > 1.\nactual = actual && `,`.\nENDIF.\nIF row-v IS BOUND.\nactual = actual && row-k && `=` && row-v->sval.\nELSE.\nactual = actual && row-k && `=missing`.\nENDIF.\nENDLOOP.\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\nENDMETHOD.\n", c[0], c[1])
	}
	b.WriteString("ENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = b.String()
}
