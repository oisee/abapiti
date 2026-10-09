package abap

import (
	"fmt"
	"os"
	"strings"
)

// castTrace (ABAPITI_CAST_TRACE=1) is a debugging emission: every statement
// line carrying a checked cast is wrapped so that a CX_SY_MOVE_CAST_ERROR
// becomes the unexecuted-trap exception naming the TypeScript location of
// the statement. Off by default; ordinary output is unchanged.
var castTrace = os.Getenv("ABAPITI_CAST_TRACE") != ""

func (e *emitter) traceCasts() {
	trap := e.name("exception.unexecuted")
	e.files[trap+".clas.abap"] = "CLASS " + trap + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nDATA source_location TYPE string.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + trap + " IMPLEMENTATION.\nENDCLASS.\n"
	// The dynamic box's get reports the tag and key it cannot read.
	if dyn := e.name("runtime.dynamic") + ".clas.abap"; e.files[dyn] != "" {
		e.files[dyn] = strings.Replace(e.files[dyn], "ELSEIF tag = tag_number OR tag = tag_boolean.\nRETURN.\nELSE.\nRAISE EXCEPTION TYPE cx_sy_move_cast_error.", "ELSEIF tag = tag_number OR tag = tag_boolean.\nRETURN.\nELSE.\nDATA(getx) = NEW "+trap+"( ).\ngetx->source_location = |dynamic.get tag={ tag } key={ p0 }|.\nRAISE EXCEPTION getx.", 1)
	}
	serial := 0
	for file, src := range e.files {
		var out strings.Builder
		where, method := "", ""
		for _, line := range strings.Split(src, "\n") {
			if rest, ok := strings.CutPrefix(line, "*@src "); ok {
				where = rest
				continue
			}
			if strings.HasPrefix(line, "METHOD ") {
				where, method = "", strings.TrimSuffix(strings.TrimPrefix(line, "METHOD "), ".")
			}
			at := where
			if at == "" {
				at = file + " " + method
			}
			if method != "" && !strings.HasPrefix(line, "METHOD ") && (strings.Contains(line, " ?= ") || strings.Contains(line, "= CAST ") || strings.Contains(line, "->as_")) {
				serial++
				out.WriteString("TRY.\n" + line + "\nCATCH cx_sy_move_cast_error.\n")
				fmt.Fprintf(&out, "DATA(castx%d) = NEW %s( ).\ncastx%d->source_location = `%s`.\nRAISE EXCEPTION castx%d.\nENDTRY.\n", serial, trap, serial, strings.ReplaceAll(at, "`", "``"), serial)
				continue
			}
			out.WriteString(line + "\n")
		}
		e.files[file] = strings.TrimSuffix(out.String(), "\n")
	}
}
