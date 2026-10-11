package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// The CI generator runs the refusal regression as well as emitting the accepted
// switch fixture for both ABAP runtimes. All expected observations come from Node.
func TestEmitSwitchGroupedLabels(t *testing.T) {
	dir := "testdata/switchgrouped"
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, ds, err := p.Lower([]string{"probe.ts"})
	if err != nil || hasBlocking(ds) {
		t.Fatalf("lower: %v %v", ds, err)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	var oracle struct{ Expected string }
	raw, err := os.ReadFile(filepath.Join(dir, "oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Expected == "" {
		t.Fatal("missing Node observation")
	}
	if got := runGoHIR(t, prog, "fmt.Print("+goEntry("probe.ts.SwitchProbe", "run")+"().String())"); got != oracle.Expected {
		t.Fatalf("Go %q, Node %q", got, oracle.Expected)
	}
	t.Run("nonempty fallthrough fails closed", func(t *testing.T) {
		refused, err := Load(filepath.Join(dir, "refused-tsconfig.json"))
		if err != nil {
			t.Fatal(err)
		}
		_, diagnostics, err := refused.Lower([]string{"refused.ts"})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range diagnostics {
			if d.Category == "unsupported-statement" && strings.Contains(d.Message, "fallthrough") {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing fallthrough diagnostic: %v", diagnostics)
		}
		trapped, ds, err := refused.LowerWithOptions([]string{"refused.ts"}, LowerOptions{TrapOnFailure: true})
		if err != nil || hasBlocking(ds) {
			t.Fatalf("trap lowering: %v %v", ds, err)
		}
		if errs := hir.Verify(trapped); len(errs) != 0 {
			t.Fatal(errs)
		}
		found = false
		for _, c := range trapped.Classes {
			for _, m := range c.Methods {
				if m.Name == "run" && isFailureBody(m.Body) {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("nonempty fallthrough did not produce a whole-body trap")
		}
		if _, err := abap.Emit(trapped); err != nil {
			t.Fatal(err)
		}
		main := `func(){defer func(){if recover()==nil{panic("missing switch trap")};fmt.Print("trapped")}();` + goEntry("refused.ts.RefusedSwitch", "run") + `(str(" "))}()`
		if got := runGoHIR(t, trapped, main); got != "trapped" {
			t.Fatal(got)
		}
	})
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	class := names.Get("probe.ts.SwitchProbe")
	var b strings.Builder
	fmt.Fprintf(&b, `CLASS ltcl_switch DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS observations FOR TESTING.
ENDCLASS.
CLASS ltcl_switch IMPLEMENTATION.
METHOD observations.
DATA actual TYPE string.
CALL METHOD %s=>%s RECEIVING result = actual.
cl_abap_unit_assert=>assert_equals( act = actual exp = `+"`%s`"+` ).
ENDMETHOD.
ENDCLASS.
`, class, names.Get("member.run"), oracle.Expected)
	files[class+".clas.testclasses.abap"] = b.String()
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "hir.txt"), []byte(hir.Dump(prog)), 0644); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
