package golang

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/jsregex"
)

// Each accepted language has been reviewed for UTF-16 JavaScript semantics.
// Everything else fails before emission when literal, or at construction when dynamic.
func reviewedRegexp(pattern, flags string) bool {
	_, ok := reviewedPatterns[pattern+"/"+flags]
	return ok
}
func (e *emitter) checkRegexp(x *hir.Expr) {
	if len(x.Args) == 0 {
		return
	}
	pattern := x.Args[0]
	flags := ""
	if pattern.Kind != hir.Lit {
		// Dynamic constructors must certify the same grammar at runtime,
		// even when no unrelated literal happened to request the helper.
		e.extendedRegex = true
		return
	}
	p, ok := pattern.Value.(string)
	if !ok {
		return
	}
	if len(x.Args) > 1 {
		if x.Args[1].Kind != hir.Lit {
			e.extendedRegex = true
			return
		}
		flags, ok = x.Args[1].Value.(string)
		if !ok {
			return
		}
	}
	if !reviewedRegexp(p, flags) {
		if _, ok := jsregex.TranslateSubset(p, flags); ok {
			e.extendedRegex = true
		} else {
			e.unsupported(x.Node, "JavaScript regexp /"+p+"/"+flags)
		}
	}
}
