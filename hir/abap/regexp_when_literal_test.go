package abap

import (
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A real kernel refuses an expression after WHEN; long dispatch keys must be
// compared in the IF chain, never split with && in a WHEN operand.
func TestRegExpDynamicWhenOperandsAreLiterals(t *testing.T) {
	long := strings.Repeat("a", 120)
	r := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, "^(?!(?:DEPTH|MULTIPLE)$)(\\/\\w+\\/)?(\\*?\\w+~(\\/\\w+\\/)?(\\w+|\\*)|\\w+)$"), hir.L(str, "i")}}
	_ = long
	result := hir.T(hir.Optional, hir.T(hir.Array, str))
	m := method("run", result, ret(rt("regexp.exec", r, result, hir.L(str, "x"))))
	p := &hir.Program{Classes: []*hir.Class{{Name: "WhenProbe", Methods: []*hir.Method{m}}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	src := files[names.Get("runtime.regexp_js")+".clas.abap"]
	if src == "" {
		t.Skip("no dynamic regexp runtime emitted")
	}
	if regexp.MustCompile("(?m)^\\s*WHEN [^\\n]*&&\\s*$").MatchString(src) {
		t.Fatal("WHEN operand split with && (refused by a real kernel)")
	}
	if strings.Contains(src, "&&\n") && !strings.Contains(src, "IF dispatch_key = ") {
		t.Fatal("long dispatch keys must be compared in the IF chain")
	}
}
