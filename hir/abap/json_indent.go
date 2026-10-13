package abap

import (
	_ "embed"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Native field layout does not prove JavaScript own-key presence/order.
// Dynamic references are checked by the writer at execution, never guessed.
func (e *emitter) jsonSubsetType(t hir.Type) bool {
	switch t.Kind {
	case hir.String, hir.Bool, hir.I32, hir.I64, hir.Number, hir.Dynamic, hir.ClassValue:
		return true
	case hir.Optional, hir.Array:
		return e.jsonSubsetType(t.Args[0])
	case hir.ClassRef:
		c := e.classBy(t.Name)
		return e.materialized[t.Name] || e.liveClasses[t.Name] || e.jsonOwnKeys && c != nil && c.Structural && c.Super == "" && jsonKeyNamesSafe(c)
	}
	return false
}

// Mirror indentJSON's UTF-16 scanner. It consumes the compact writer output,
// preserving key order, escaped strings and shortest binary64 spellings.
func (e *emitter) jsonIndentRuntime() {
	name := e.name("runtime.jsonIndent")
	if e.types["runtime.jsonIndent"] {
		return
	}
	e.types["runtime.jsonIndent"] = true
	e.files[name+".clas.abap"] = strings.ReplaceAll(jsonIndentSource, "$I", name)
}

//go:embed json_indent_runtime.abap.tmpl
var jsonIndentSource string
