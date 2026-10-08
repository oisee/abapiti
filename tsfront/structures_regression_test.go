package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestStructuresShortCircuitValues(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
  static hits: number = 0;
  static fallback(): string { Probe.hits = Probe.hits + 1; return "fallback"; }
  static run(): string {
   Probe.hits = 0;
   const absent: string | undefined = undefined;
   const present: string = "x";
   const a = absent || Probe.fallback();
   const b = present || Probe.fallback();
   return a + b + Probe.hits.toString();
  }
 }
 `}, []string{"probe.ts"})
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	driver := `CLASS ltcl_shortcircuit DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS values FOR TESTING.
ENDCLASS.
CLASS ltcl_shortcircuit IMPLEMENTATION.
METHOD values.
DATA actual TYPE string.
actual = CLASSNAME=>RUNNAME( ).
cl_abap_unit_assert=>assert_equals( act = actual exp = ` + "`fallbackx1`" + ` ).
ENDMETHOD.
ENDCLASS.
`
	class := names.Get("probe.ts.Probe")
	driver = strings.ReplaceAll(driver, "CLASSNAME", class)
	driver = strings.ReplaceAll(driver, "RUNNAME", names.Get("member.run"))
	files[class+".clas.testclasses.abap"] = driver
	if out := os.Getenv("STRUCTURES_REGRESSION_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestStructuresInfiniteLoopReturn(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe { static run(): number { while(true) { while(true) { break; } return 1; } } }
 `}, []string{"probe.ts"})
	if errors := hir.Verify(prog); len(errors) != 0 {
		t.Fatal(errors)
	}
	// A break in the outer loop permits fallthrough and must still be rejected.
	var run *hir.Method
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			if m.Name == "run" {
				run = m
			}
		}
	}
	if run == nil {
		t.Fatal("no run")
	}
	run.Body = hir.B(&hir.Stmt{Kind: hir.While, X: hir.L(hir.T(hir.Bool), true), Body: hir.B(&hir.Stmt{Kind: hir.Break})})
	if errors := hir.Verify(prog); len(errors) == 0 {
		t.Fatal("loop with an exit accepted without a return")
	}
}

func TestStructuresCorpusIntegrity(t *testing.T) {
	cases, err := loadStructuresCorpus(filepath.Join("testdata", "structurescorpus"))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, c := range cases {
		for _, line := range strings.Split(c.Dump, "\n") {
			parts := strings.Split(line, "|")
			if len(parts) == 3 && strings.HasPrefix(parts[1], "S") {
				kinds[parts[1][1:]] = true
			}
		}
	}
	if len(cases) != 192 || len(kinds) != 59 {
		t.Fatalf("%d cases, %d structure kinds", len(cases), len(kinds))
	}
}

func TestStructuresMixedShortCircuitIsDiagnostic(t *testing.T) {
	dir := t.TempDir()
	for name, source := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`,
		"input.ts":      `export class Probe { static run(flag: boolean): string | boolean { return flag && "selected"; } }`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, diags := lowerProbe(t, dir)
	for _, d := range diags {
		if d.Category == "unsupported-shortcircuit" && strings.Contains(d.Loc, "input.ts:") {
			return
		}
	}
	t.Fatalf("mixed short-circuit operands were silently lowered: %v", diags)
}
