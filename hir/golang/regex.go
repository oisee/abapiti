package golang

import (
	"strings"

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
	if x.Name == "iteratorGrammar" {
		e.regexA16 = true
	}
	pattern := x.Args[0]
	flags := ""
	if pattern.Kind != hir.Lit {
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
	if !reviewedRegexp(p, flags) && (strings.Contains(flags, "u") || strings.Contains(p, "(?<") || p == "" || p == "(?:)" || p == jsregex.IteratorLabelPattern || p == jsregex.BindingScannerPattern || p == jsregex.BindingMaskPattern) {
		e.regexA16 = true
	}
	if !reviewedRegexp(p, flags) {
		translate := jsregex.TranslateSubset
		if e.regexA16 {
			translate = jsregex.TranslateIteratorSubset
		}
		if _, ok := translate(p, flags); ok {
			e.extendedRegex = true
		} else if _, ok := jsregex.TranslateExtendedSubset(p, flags); ok {
			e.extendedRegex, e.regexA16, e.regexExtended = true, true, true
		} else {
			e.unsupported(x.Node, "JavaScript regexp /"+p+"/"+flags)
		}
	}
}
