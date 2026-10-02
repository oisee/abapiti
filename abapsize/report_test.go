package abapsize

import (
	"strings"
	"testing"
)

func TestReport(t *testing.T) {
	src := "METHOD example.\n" + strings.Repeat("x", 255) + "\n" + strings.Repeat("x", 256) + "\nENDMETHOD.\n"
	r := Report(map[string]string{"sample.abap": src})
	f := r.Files["sample.abap"]
	if f.Lines != 4 || f.MaxLineLength != 256 || f.LongestRoutine != 4 || f.Routines != 1 || f.Bytes != len(src) {
		t.Fatalf("wrong size: %+v", f)
	}
	errs := r.Errors()
	if len(errs) != 1 || !strings.Contains(errs[0], "sample.abap:3:256") {
		t.Fatalf("wrong errors: %v", errs)
	}
	warnings := r.Warnings(Thresholds{Lines: 3, RoutineLines: 3, Routines: 0, Bytes: 10})
	if len(warnings) != 3 {
		t.Fatalf("wrong warnings: %v", warnings)
	}
}
