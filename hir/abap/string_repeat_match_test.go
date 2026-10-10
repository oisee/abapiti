package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestEmitStringRepeatAndMatch(t *testing.T) {
	methods := []*hir.Method{}
	for i, c := range []struct {
		s string
		n float64
	}{{"ab", 0}, {"ab", 1}, {"ab", 3}, {"", 4}, {"x", 2.9}, {"x", -0.9}} {
		m := method("repeat"+string(rune('a'+i)), str, ret(rt("string.repeat", hir.L(str, c.s), str, hir.L(hir.T(hir.Number), c.n))))
		m.Static = true
		methods = append(methods, m)
	}
	result := hir.T(hir.Optional, hir.T(hir.Array, str))
	for i, c := range []struct{ s, p, f string }{{"xyz", "(a)(b)", ""}, {"zab", "(a)(b)", ""}, {"&&&", "&", "g"}} {
		pattern := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, c.p), hir.L(str, c.f)}}
		m := method("match"+string(rune('a'+i)), result, ret(rt("string.match", hir.L(str, c.s), result, pattern)))
		m.Static = true
		methods = append(methods, m)
	}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: methods}}}
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
	for _, want := range []string{"trunc(", "repeat( val = ", "cx_sy_range_out_of_bounds", "METHOD match.", "FIND FIRST OCCURRENCE OF REGEX", "FIND ALL OCCURRENCES OF REGEX", "LOOP AT hit-submatches", "IF matches IS INITIAL.\n  RETURN.", "DATA match_index TYPE i.", "result->match_index = hit-offset."} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q", want)
		}
	}
}
