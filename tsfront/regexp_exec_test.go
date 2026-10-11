package tsfront

import (
	"encoding/json"
	"fmt"
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitRegExpExec(t *testing.T) {
	p, err := Load("testdata/regexp-exec/tsconfig.json")
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
	raw, err := os.ReadFile("testdata/regexp-exec/oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle map[string]string
	if err = json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	want := oracle["run"]
	got := strings.TrimSuffix(runGoHIR(t, prog, "fmt.Println("+goEntry("probe.ts.Probe", "run")+"().String())"), "\n")
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	class := names.Get("probe.ts.Probe")
	driver := "CLASS ltcl_regex DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS observations FOR TESTING.\nENDCLASS.\nCLASS ltcl_regex IMPLEMENTATION.\nMETHOD observations.\nDATA expected TYPE string.\nDATA ch TYPE c LENGTH 1.\nDATA actual TYPE string.\n"
	for _, line := range abapStringBuild("expected", "ch", want) {
		driver += line + "\n"
	}
	driver += fmt.Sprintf("actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\nENDMETHOD.\nENDCLASS.\n", class, names.Get("member.run"))
	files[class+".clas.testclasses.abap"] = driver
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, text := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
