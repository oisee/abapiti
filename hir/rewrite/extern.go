package rewrite

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
)

func (x *extractor) externCall(e *hir.Expr, path string) {
	x.add("unknown_effect", x.method)
	x.add("raises", x.method, path)
	x.add("runtime_op", path, "extern.call")
	x.add("site_method", path, x.method)
	x.add("op_reads", "extern.call", "Args")
	x.add("op_reads", "extern.call", "Global")
	x.add("op_allocates", "extern.call", "true")
	x.add("op_may_raise", "extern.call", "Catchable")
	x.add("op_writes", "extern.call", "Global")
	x.add("op_aliases", "extern.call", "None")
	x.add("op_conservative", "extern.call", "Host module effects are unknown.")
	for i := range e.Args {
		x.add("sink", x.method, fmt.Sprintf("%s/arg%d", path, i))
	}
}
