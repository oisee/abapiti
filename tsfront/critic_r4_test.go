package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestCriticR4ErasedVirtualDispatch(t *testing.T) {
	source, err := os.ReadFile("testdata/critic-r4/dispatch.ts")
	if err != nil {
		t.Fatal(err)
	}
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": string(source)}, []string{"probe.ts"})
	var probe *hir.Class
	for _, c := range prog.Classes {
		if c.Name == "probe.ts.Probe" {
			probe = c
		}
	}
	if probe == nil {
		t.Fatal("no Probe")
	}
	// A renamed declaring-class implementation is never a virtual call target
	// in the ordinary caller; only bridges and super calls may use it.
	body := hir.Dump(&hir.Program{Classes: []*hir.Class{probe}})
	if strings.Contains(body, "instantiated") {
		t.Fatal("ordinary call bypasses virtual slot", body)
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	files[names.Get(probe.Name)+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_dispatch DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS dispatch FOR TESTING.
ENDCLASS.
CLASS ltcl_dispatch IMPLEMENTATION.
METHOD dispatch.
DATA actual TYPE string.
actual = %s=>%s( ).
cl_abap_unit_assert=>assert_equals( act = actual exp = `+"`sub|sub|sub|node|sub`"+` ).
ENDMETHOD.
ENDCLASS.
`, names.Get(probe.Name), names.Get("member.run"))
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		dir := filepath.Join(out, "critic-r4", "dispatch")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for file, src := range files {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCriticR4LiftedDefaultsFailClosed(t *testing.T) {
	source, err := os.ReadFile("testdata/critic-r4/defaults.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{string(source), `export class Probe { static run(): number { const captured = 3; const first = (n: number = captured): number => n; const second = (): number => first(); return second(); } }`} {
		requireDiagnostic(t, sourceProbe(t, src), "unsupported-lifted-default")
	}
}

func TestCriticR4IncludeOracle(t *testing.T) {
	cases, err := LoadStatementsCorpus("testdata/stmtsregressions")
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 1 || cases[0].Abap != "INCLUDE zfoo." || cases[0].Statements != 1 || cases[0].Tokens != 3 || !strings.Contains(cases[0].Dump, "Include|") {
		t.Fatal("missing upstream INCLUDE regression", cases)
	}
}
