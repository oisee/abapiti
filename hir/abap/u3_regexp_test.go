package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestU3RegexpFlagsAndClone(t *testing.T) {
	regex := hir.T(hir.RegExp)
	var expressions []*hir.Expr
	for _, flags := range []string{"", "g", "gi", "gy"} {
		re := &hir.Expr{Kind: hir.New, Type: regex, Args: []*hir.Expr{hir.L(str, "a"), hir.L(str, flags)}}
		expressions = append(expressions, rt("regexp.flags", re, str), rt("regexp.clone", re, regex))
	}
	src := u3Emit(t, expressions...)
	u3Want(t, src, "->flags.", "->clone( ).", "METHODS clone REDEFINITION.", "copy->sticky = sticky.", "copy->exec_pattern = exec_pattern.", "result->group_names = me->group_names.", "->last_index = nmax( val1 = 0 val2 = last_index ).")
}

func TestU3RegexpLegacyMetadataDoesNotActivateJS(t *testing.T) {
	regex := hir.T(hir.RegExp)
	re := &hir.Expr{Kind: hir.New, Type: regex, Args: []*hir.Expr{hir.L(str, "a"), hir.L(str, "gi")}}
	src := u3Emit(t, rt("regexp.flags", re, str), rt("regexp.clone", re, regex))
	u3Want(t, src, "DATA flags TYPE string.", "DATA last_index TYPE i.", "result->flags = me->flags.", "result->last_index = nmax(")
	if strings.Contains(src, "FIND PCRE") || strings.Contains(src, "METHODS clone REDEFINITION.") {
		t.Fatal("legacy metadata activated the JS engine")
	}
}
