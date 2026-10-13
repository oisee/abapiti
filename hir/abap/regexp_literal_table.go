package abap

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/jsregex"
)

// Go's checked runtime admits TranslateSubset in addition to Reviewed. Certify
// the program's literal constructors with that same parser, not a second regex
// grammar. Unseen dynamic patterns still refuse; no arbitrary RE2 is admitted.
// Do not mutate Reviewed: it is also used by default Go emission.
func (e *emitter) regexpLiteralTable() map[string][2]string {
	table := make(map[string][2]string, len(jsregex.Reviewed))
	for key, spec := range jsregex.Reviewed {
		table[key] = spec
	}
	seen := map[*hir.Expr]bool{}
	var visit func(*hir.Expr)
	visit = func(x *hir.Expr) {
		if x == nil || seen[x] {
			return
		}
		seen[x] = true
		if x.Kind == hir.New && x.Type.Kind == hir.RegExp && len(x.Args) >= 1 && len(x.Args) <= 2 && x.Args[0].Kind == hir.Lit {
			pattern, ok := x.Args[0].Value.(string)
			flags, flagsOK := "", true
			if len(x.Args) == 2 {
				flags, flagsOK = x.Args[1].Value.(string)
				flagsOK = flagsOK && x.Args[1].Kind == hir.Lit
			}
			key := pattern + "/" + flags
			if _, exists := table[key]; ok && flagsOK && !exists {
				if translated, accepted := jsregex.TranslateSubset(pattern, flags); accepted {
					table[key] = [2]string{translated, ""}
				}
			}
		}
		e.walkStmt(x.Stmt, visit)
		visit(x.X)
		visit(x.Y)
		visit(x.Z)
		for _, arg := range x.Args {
			visit(arg)
		}
	}
	for _, class := range e.p.Classes {
		if class.Ctor != nil {
			e.walkStmt(class.Ctor.Body, visit)
		}
		for _, method := range class.Methods {
			e.walkStmt(method.Body, visit)
		}
	}
	return table
}
