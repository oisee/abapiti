package abap

import "github.com/oisee/abapiti/hir"

// The telemetry mapping uses elapsed microseconds; it cannot implement the
// epoch clock. Until a timestamp ABI exists ABAP refuses at the call site.
func init() {
	for _, op := range []string{"clock.epochMilliseconds", "weak.lookupKey", "weak.writeKey"} {
		unsupportedOps[op] = true
	}
}

func (b *body) mapInsertions(x *hir.Expr, n string) bool {
	if x.Op != "map.insertions" {
		return false
	}
	b.line(n + " = " + b.expr(x.X) + "->insertions( ).")
	return true
}
