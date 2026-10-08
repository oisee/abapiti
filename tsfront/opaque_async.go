package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Check declaration provenance: user classes named Promise/Map/Set must
// retain their own semantics.
func (l *lowerer) librarySymbol(sym *ast.Symbol) bool {
	if sym == nil {
		return false
	}
	f := l.fileOfSymbol(sym)
	return f != nil && strings.Contains(f.FileName(), "/lib.") && strings.HasSuffix(f.FileName(), ".d.ts")
}

func (l *lowerer) opaquePromise() string {
	const name = "builtin.OpaquePromise"
	for _, c := range l.out.Classes {
		if c.Name == name {
			return name
		}
	}
	// Abstract with no API or constructor: a trapped body may mention this
	// result type but cannot produce a successfully callable Promise.
	l.out.Classes = append(l.out.Classes, &hir.Class{Name: name, Abstract: true})
	return name
}
