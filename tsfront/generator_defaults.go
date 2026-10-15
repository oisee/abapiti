package tsfront

import (
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Generator<T> defaults its completion and input parameters to any. Consumers
// still use the void completion ABI: sent values and return/throw are refused.
func generatorSyntaxDefaults(a []*ast.Node) bool {
	return len(a) == 1 || len(a) == 3 && a[1].Kind == ast.KindVoidKeyword && a[2].Kind == ast.KindUndefinedKeyword
}
func generatorCheckerDefaults(a []*checker.Type) bool {
	return len(a) == 3 && (a[1].Flags()&checker.TypeFlagsVoid != 0 && a[2].Flags()&checker.TypeFlagsUndefined != 0 || a[1].Flags()&checker.TypeFlagsAny != 0 && a[2].Flags()&checker.TypeFlagsAny != 0)
}
