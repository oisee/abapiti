package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

// Attach an executable ABAP Unit oracle as well as collecting emitted sources.
func storageOracleABAP(t *testing.T, p *hir.Program, want string) {
	t.Helper()
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("probe.ts.Probe")
	var b strings.Builder
	b.WriteString("CLASS ltcl_storage DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS oracle FOR TESTING.\nENDCLASS.\nCLASS ltcl_storage IMPLEMENTATION.\nMETHOD oracle.\nDATA expected TYPE string.\nDATA ch TYPE c LENGTH 1.\n")
	for _, line := range abapStringBuild("expected", "ch", strings.TrimSuffix(want, "\n")) {
		b.WriteString(line + "\n")
	}
	fmt.Fprintf(&b, "DATA(actual) = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.run"))
	files[owner+".clas.testclasses.abap"] = b.String()
}
