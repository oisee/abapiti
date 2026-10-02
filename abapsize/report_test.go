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

func TestLineLengthUTF16(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		long bool
	}{
		{"255 umlauts", strings.Repeat("ä", 255), false},
		{"256 umlauts", strings.Repeat("ä", 256), true},
		{"127 emoji", strings.Repeat("😀", 127), false},
		{"128 emoji", strings.Repeat("😀", 128), true},
	} {
		r := Report(map[string]string{"u.abap": tc.line + "\n"})
		if got := len(r.LongLines) > 0; got != tc.long {
			t.Errorf("%s: long=%t want %t (max-line=%d)", tc.name, got, tc.long, r.Files["u.abap"].MaxLineLength)
		}
	}
}

// One-line routines, lowercase keywords and a comment after ENDMETHOD must all
// end the routine, or the following lines are counted into it.
func TestRoutineBounds(t *testing.T) {
	src := strings.Join([]string{
		"METHOD a. x = 1. ENDMETHOD.",
		"other.", "other.", "other.",
		"method b.",
		"  x = 1.",
		"endmethod.",
		"other.", "other.", "other.",
		"FORM c.",
		"ENDFORM. \" done",
		"other.", "other.", "other.",
	}, "\n") + "\n"
	f := Report(map[string]string{"r.abap": src}).Files["r.abap"]
	if f.Routines != 3 || f.LongestRoutine != 3 {
		t.Fatalf("routines=%d longest=%d, want 3 and 3", f.Routines, f.LongestRoutine)
	}
}
