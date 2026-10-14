package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Node is the oracle; export the emitted ABAP Unit fixture for pinned OSGO.
func TestEmitPOSIXRegexJSClasses(t *testing.T) {
	source, err := os.ReadFile("testdata/posix-regex-js/probe.ts")
	if err != nil {
		t.Fatal(err)
	}
	// Erase only the fixture's TypeScript declarations, preserving regex/input text.
	js := strings.NewReplacer("export class", "class", "pattern: string", "pattern", "input: string", "input", "): boolean", ")", "): string", ")").Replace(string(source))
	raw, err := exec.Command("node", "-e", js+"; process.stdout.write(Probe.run());").CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v: %s", err, raw)
	}
	want := string(raw)
	p, err := Load("testdata/posix-regex-js/tsconfig.json")
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"probe.ts"}, nil, nil)
	if err != nil || hasBlocking(diags) {
		t.Fatal(err, diags)
	}
	files, names, err := emitABAPNamed(t, prog)
	if err != nil {
		t.Fatal(err)
	}
	class := names.Get("probe.ts.Probe")
	driver := "CLASS ltcl_regex DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS observations FOR TESTING.\nMETHODS complement_refusal FOR TESTING.\nENDCLASS.\nCLASS ltcl_regex IMPLEMENTATION.\nMETHOD observations.\nDATA expected TYPE string.\nDATA ch TYPE c LENGTH 1.\nDATA actual TYPE string.\n"
	for _, line := range abapStringBuild("expected", "ch", want) {
		driver += line + "\n"
	}
	driver += fmt.Sprintf("actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\nENDMETHOD.\n", class, names.Get("member.run"))
	// Dynamic class complements fail at construction, not as a wrong match.
	driver += fmt.Sprintf("METHOD complement_refusal.\nDATA regex TYPE REF TO %s.\nDATA refused TYPE abap_bool.\nTRY.\nCREATE OBJECT regex EXPORTING pattern = '[\\S]' flags = ``.\nCATCH cx_sy_range_out_of_bounds.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_equals( act = refused exp = abap_true ).\nENDMETHOD.\nENDCLASS.\n", names.Get("runtime.regexp"))
	files[class+".clas.testclasses.abap"] = driver
	if out := fixtureOutputDir(t); out != "" {
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
