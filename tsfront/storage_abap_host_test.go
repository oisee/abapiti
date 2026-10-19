package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func storageABAPProbe(t *testing.T, p *hir.Program, ownerName, want, refusedMethod string, refusalException ...string) {
	t.Helper()
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get(ownerName)
	var unit strings.Builder
	fmt.Fprint(&unit, "CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nDATA refused TYPE abap_bool.\n")
	fmt.Fprintf(&unit, "actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `%s` ).\n", owner, names.Get("member.run"), strings.ReplaceAll(want, "`", "``"))
	if refusedMethod != "" {
		exception := "view.refusal"
		if len(refusalException) > 0 {
			exception = refusalException[0]
		}
		if !strings.HasPrefix(exception, "cx_") {
			exception = names.Get(exception)
		}
		fmt.Fprintf(&unit, "TRY.\n%s=>%s( ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member."+refusedMethod), exception)
	}
	fmt.Fprint(&unit, "ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	dir := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		dir = filepath.Join(base, t.Name())
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, source := range files {
		formatted, err := abap.Format(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(formatted), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if osg := os.Getenv("ABAPITI_OSG_HOME"); osg != "" {
		cmd := exec.Command("node", filepath.Join(osg, "tools/gogen/unit.mjs"), "--input", dir, "--class", owner, "--jobs", "1", "--no-cache", "--out", t.TempDir())
		cmd.Env = append(os.Environ(), "OSG_HOME="+osg, "GOFLAGS="+os.Getenv("GOFLAGS")+" -p=2")
		out, err := cmd.Output()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				t.Fatalf("OSG: %v\n%s\n%s", err, out, exit.Stderr)
			}
			t.Fatal(err)
		}
		var report struct {
			Compiled int `json:"compiled"`
			Rows     []struct {
				Status string `json:"status"`
			} `json:"rows"`
		}
		if err := json.Unmarshal(out, &report); err != nil {
			t.Fatalf("OSG JSON: %v\n%s", err, out)
		}
		if report.Compiled != 1 || len(report.Rows) != 1 || report.Rows[0].Status != "SUCCESS" {
			t.Fatalf("OSG probe: %s", out)
		}
		t.Log("OSG: 1 SUCCESS row")
	}
}
