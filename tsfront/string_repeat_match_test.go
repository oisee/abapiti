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

// Emitted to a dedicated directory for hir-unit.sh's osgo/OSG-JS Node gate.
func TestEmitStringRepeatMatch(t *testing.T) {
	p, err := Load("testdata/string-repeat-match/tsconfig.json")
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"probe.ts"}, nil, nil)
	if err != nil || hasBlocking(diags) {
		t.Fatal(err, diags)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	raw, err := os.ReadFile("testdata/string-repeat-match/oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle map[string]string
	if err := json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	want, ok := oracle["run"]
	if !ok {
		t.Fatal("missing Node observation")
	}
	if got := strings.TrimSuffix(runGoHIR(t, prog, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"().String())"), "\n"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	class := names.Get("probe.ts.Probe")
	driver := "CLASS ltcl_strings DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS observations FOR TESTING.\nENDCLASS.\nCLASS ltcl_strings IMPLEMENTATION.\nMETHOD observations.\nDATA expected TYPE string.\nDATA ch TYPE c LENGTH 1.\nDATA actual TYPE string.\n"
	for _, line := range abapStringBuild("expected", "ch", want) {
		driver += line + "\n"
	}
	driver += fmt.Sprintf("actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\nENDMETHOD.\nENDCLASS.\n", class, names.Get("member.run"))
	files[class+".clas.testclasses.abap"] = driver
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for file, text := range files {
			if err := os.WriteFile(filepath.Join(out, file), []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestStringMatchUnsupportedFlags(t *testing.T) {
	dir := t.TempDir()
	for file, text := range map[string]string{"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2020"},"files":["probe.ts"]}`, "probe.ts": `export class Probe { run(): RegExpMatchArray | null { return "a".match(/a/y); } }`} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, diags, err := p.LowerWithReachability([]string{"probe.ts"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Category == "unsupported-regex" && strings.Contains(d.Message, "only i and g") {
			return
		}
	}
	t.Fatalf("missing clear unsupported match diagnostic: %v", diags)
}
