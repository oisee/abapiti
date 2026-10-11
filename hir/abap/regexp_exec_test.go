package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestRegExpExecRuntime(t *testing.T) {
	result := hir.T(hir.Optional, hir.T(hir.Array, str))
	pattern := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, "(a)(b)"), hir.L(str, "g")}}
	m := method("exec", result, ret(rt("regexp.exec", pattern, result, hir.L(str, "zab"))))
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	files, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	text := ""
	for _, s := range files {
		text += s
	}
	for _, want := range []string{"METHOD exec.", "DATA last_index TYPE i.", "READ TABLE groups INDEX 1", "last_index = hit-offset + hit-length", "last_index = 0", "match_missing", "result->match_index = hit-offset"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, forbidden := range []string{"RESULTS DATA(", "INTO DATA(hit)", "INTO DATA(group)"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("FIND results must use DDIC declarations, found %s", forbidden)
		}
	}
	for _, want := range []string{"TYPE match_result_tab.", "TYPE match_result.", "TYPE submatch_result.", "TYPE submatch_result_tab.", "APPEND hit TO matches.", "groups = hit-submatches."} {
		if !strings.Contains(text, want) {
			t.Errorf("missing DDIC result typing: %s", want)
		}
	}
	if !strings.Contains(text, "FIND FIRST OCCURRENCE OF PCRE search") || !strings.Contains(text, "METHOD unit_text.") {
		t.Fatal("JS exec needs the PCRE/UTF-16 path, not legacy POSIX matching")
	}
}
