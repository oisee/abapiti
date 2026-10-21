package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

// Clone the already reviewed compiled representation, including engine kind.
// Recompiling source could change exclusions or reject a checked literal.
func (e *emitter) regexpCloneRuntime() {
	if _, ok := hir.UsedRuntimeOps(e.p)["regexp.clone"]; !ok {
		return
	}
	base := e.name("runtime.regexp")
	for _, js := range []bool{false, true} {
		if js && !e.jsRegexpUsed {
			continue
		}
		id := "runtime.regexp"
		if js {
			id = "runtime.regexp_js"
		}
		name := e.name(id)
		file := name + ".clas.abap"
		code := e.files[file]
		def := "METHODS clone RETURNING VALUE(result) TYPE REF TO " + base + "."
		target := "result"
		impl := "METHOD clone.\n"
		if js {
			def = "METHODS clone REDEFINITION."
			target = "copy"
			impl += "DATA copy TYPE REF TO " + name + ".\n"
		}
		impl += "CREATE OBJECT " + target + " EXPORTING pattern = `` flags = ``.\n"
		for _, field := range []string{"source", "flags", "posix", "excluded", "ignore_case", "global", "group_names", "last_index"} {
			impl += target + "->" + field + " = me->" + field + ".\n"
		}
		impl += target + "->last_index = nmax( val1 = 0 val2 = last_index ).\n"
		if js {
			impl += "copy->sticky = sticky.\ncopy->exec_pattern = exec_pattern.\nresult = copy.\n"
		}
		impl += "ENDMETHOD.\n"
		code = strings.Replace(code, "PUBLIC SECTION.", "PUBLIC SECTION.\n"+def, 1)
		e.files[file] = strings.TrimSuffix(code, "ENDCLASS.\n") + impl + "ENDCLASS.\n"
	}
}
