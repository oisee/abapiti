package golang

import (
	"github.com/oisee/abapiti/hir"
)

// Each accepted language has been reviewed for UTF-16 JavaScript semantics.
// Everything else fails before emission when literal, or at construction when dynamic.
func reviewedRegexp(pattern, flags string) bool {
	switch pattern + "/" + flags {
	case "^Y/", "^Z/", "test$/i", "a.c/i", "x/y/gi":
		return true
	}
	return false
}
func (e *emitter) checkRegexp(x *hir.Expr) {
	if len(x.Args) == 0 {
		return
	}
	pattern := x.Args[0]
	flags := ""
	if pattern.Kind != hir.Lit {
		return
	}
	p, ok := pattern.Value.(string)
	if !ok {
		return
	}
	if len(x.Args) > 1 {
		if x.Args[1].Kind != hir.Lit {
			return
		}
		flags, ok = x.Args[1].Value.(string)
		if !ok {
			return
		}
	}
	if !reviewedRegexp(p, flags) {
		e.unsupported(x.Node, "JavaScript regexp /"+p+"/"+flags)
	}
}
