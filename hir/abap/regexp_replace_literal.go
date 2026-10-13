package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// A fresh global singleton regex has no observable identity or lastIndex.
// Literal replacement is exactly substring replacement, even on multiline text.
// The legacy POSIX path cannot translate these Unicode escapes. The checked
// PCRE path handles them normally, including text containing line ends.
// This is ABAP-only; shared lowering and the default Go output are unchanged.
func (b *body) replaceFreshCharacterRegex(x *hir.Expr, n string) bool {
	if x.Op != "string.replaceRegex" || len(x.Args) != 2 {
		return false
	}
	r, replacement := x.Args[0], x.Args[1]
	if r.Kind != hir.New || r.Type.Kind != hir.RegExp || len(r.Args) != 2 || r.Args[0].Kind != hir.Lit || r.Args[1].Kind != hir.Lit || replacement.Kind != hir.Lit {
		return false
	}
	if regexNeedsJS(r) || b != nil && b.e != nil && b.e.execUsed && regexMayHaveState(r) {
		return false
	}
	pattern, patternOK := r.Args[0].Value.(string)
	flags, flagsOK := r.Args[1].Value.(string)
	with, withOK := replacement.Value.(string)
	if !patternOK || !flagsOK || !withOK || flags != "g" || strings.Contains(with, "$") {
		return false
	}
	var needle string
	switch pattern {
	case `\u00a0`:
		needle = "\u00a0"
	case `\u000b`:
		needle = "\v"
	default:
		return false
	}
	input := b.expr(x.X)
	search := b.expr(hir.L(hir.T(hir.String), needle))
	value := b.expr(replacement)
	b.line(n + " = " + input + ".")
	b.line("REPLACE ALL OCCURRENCES OF " + search + " IN " + n + " WITH " + value + ".")
	return true
}
