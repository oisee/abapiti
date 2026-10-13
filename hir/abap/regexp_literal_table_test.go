package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/jsregex"
)

func TestRegExpLiteralSubsetTable(t *testing.T) {
	const pattern = `^'(?:[^'\\]|''|\\'|\\\\|\\(?!'))*'$`
	before := len(jsregex.Reviewed)
	r := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, pattern), hir.L(str, "")}}
	result := hir.T(hir.Optional, hir.T(hir.Array, str))
	m := method("run", result, ret(rt("regexp.exec", r, result, hir.L(str, "'he''llo'"))))
	p := &hir.Program{Classes: []*hir.Class{{Name: "CDSStringProbe", Methods: []*hir.Method{m}}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	source := files[names.Get("runtime.regexp_js")+".clas.abap"]
	if !strings.Contains(source, "WHEN `"+pattern+"/`") || !strings.Contains(source, `\\{2,}`) {
		t.Fatal("CDS string grammar must use Go's certified TranslateSubset language")
	}
	_, pcre, _ := strings.Cut(source, "METHOD pcre_pattern.")
	pcre, _, _ = strings.Cut(pcre, "ENDMETHOD.")
	if !strings.Contains(pcre, `(?![\s\S])`) || strings.Contains(pcre, `\z`) {
		t.Fatal("emitted ABAP must retain JS-compatible absolute assertions")
	}
	if len(jsregex.Reviewed) != before {
		t.Fatal("ABAP modified the shared Go regex table")
	}
}
