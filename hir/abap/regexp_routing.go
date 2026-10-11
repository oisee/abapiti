package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Existing dynamic configuration patterns with fixed i/g flags retain their
// legacy POSIX contract. Unknown flags, advanced literal syntax and observed
// RegExp state require the checked JS runtime. Merely using String.match does
// not upgrade every regex in the program.
func regexNeedsJS(x *hir.Expr) bool {
	if len(x.Args) > 1 {
		if x.Args[1].Kind != hir.Lit {
			return true
		}
		flags, _ := x.Args[1].Value.(string)
		if strings.ContainsAny(flags, "my") {
			return true
		}
	}
	if len(x.Args) == 0 || x.Args[0].Kind != hir.Lit {
		return false
	}
	p, _ := x.Args[0].Value.(string)
	// Scan what remains after the legacy translator removes its bounded
	// negative assertions. Shorthand expansion alone is not evidence that
	// an arbitrary lookahead was translated.
	p, _ = regexPattern(p)
	inClass := false
	for i := 0; i < len(p); i++ {
		if p[i] == '\\' {
			i++
			continue
		}
		if p[i] == '[' {
			inClass = true
		} else if p[i] == ']' {
			inClass = false
		} else if !inClass {
			if strings.HasPrefix(p[i:], "(?<") || strings.HasPrefix(p[i:], "(?=") || strings.HasPrefix(p[i:], "(?!") {
				return true
			}
			if i+1 < len(p) && strings.ContainsRune("*+?}", rune(p[i])) && p[i+1] == '?' {
				return true
			}
		}
	}
	return false
}

func regexMayHaveState(x *hir.Expr) bool {
	if len(x.Args) < 2 {
		return false
	}
	if x.Args[1].Kind != hir.Lit {
		return true
	}
	flags, _ := x.Args[1].Value.(string)
	return strings.ContainsAny(flags, "gy")
}

// Restore the pre-exec runtime for legacy users, retaining inline FIND types
// without opting into a JS engine which the pinned OSGO cannot compile.
func (e *emitter) regexpLegacyRuntime(name string) {
	file := name + ".clas.abap"
	text := e.files[file]
	if e.jsRegexpUsed {
		// Mixed programs share the feature array ABI, but never the PCRE
		// engine in this base class.
		text = strings.Replace(text, "DATA flags TYPE string.", "DATA flags TYPE string.\nDATA last_index TYPE i.", 1)
		e.files[file] = text
		return
	}
	text = strings.Replace(text, "IF p1 CS '$'.\n  RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\n", "", 1)
	text = strings.Replace(text, "DATA idx TYPE i.\n", "", 1)
	text = strings.ReplaceAll(text, "FIRST OCCURRENCE OF REGEX posix IN p0 IGNORING CASE RESULTS hit", "FIRST OCCURRENCE OF REGEX posix IN p0 IGNORING CASE RESULTS matches")
	text = strings.ReplaceAll(text, "FIRST OCCURRENCE OF REGEX posix IN p0 RESULTS hit", "FIRST OCCURRENCE OF REGEX posix IN p0 RESULTS matches")
	text = strings.Replace(text, "  IF sy-subrc = 0.\n    APPEND hit TO matches.\n  ENDIF.\n", "", 1)
	text = strings.Replace(text, "    groups = hit-submatches.\n    LOOP AT groups INTO group.", "    LOOP AT hit-submatches INTO group.", 1)
	text = strings.Replace(text, "        idx = lines( result->items ).\n        INSERT idx INTO TABLE result->match_missing.\n        CLEAR part.\n      ELSE.\n        part = p0+group-offset(group-length).\n      ENDIF.", "        RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\n      ENDIF.\n      part = p0+group-offset(group-length).", 1)
	text = strings.Replace(text, "DATA flags TYPE string.\n", "", 1)
	text = strings.Replace(text, "me->flags = flags.\n", "", 1)
	e.files[file] = text
}

// Only this subclass contains PCRE. Plain test/replace/match constructors and
// their translation remain legacy even in a mixed program. Materialized exec
// converts a legacy receiver on demand, preserving the receiver's lastIndex.
func (e *emitter) regexpJSSubclass() {
	name := e.name("runtime.regexp_js")
	base := e.name("runtime.regexp")
	file := name + ".clas.abap"
	text := e.files[file]
	text = strings.Replace(text, "DEFINITION PUBLIC CREATE PUBLIC.", "DEFINITION PUBLIC INHERITING FROM "+base+" CREATE PUBLIC.", 1)
	for _, field := range []string{"source", "flags", "posix", "excluded"} {
		text = strings.Replace(text, "DATA "+field+" TYPE string.\n", "", 1)
	}
	for _, field := range []string{"ignore_case", "global"} {
		text = strings.Replace(text, "DATA "+field+" TYPE abap_bool.\n", "", 1)
	}
	text = strings.Replace(text, "DATA last_index TYPE i.\n", "", 1)
	for _, method := range []string{"test", "match_test", "match", "replace"} {
		start := strings.Index(text, "METHODS "+method+" IMPORTING")
		end := strings.Index(text[start:], ".\n") + start
		text = text[:start] + "METHODS " + method + " REDEFINITION" + text[end:]
	}
	text = strings.Replace(text, "METHOD constructor.\n", "METHOD constructor.\nsuper->constructor( pattern = pattern flags = flags excluded_pattern = excluded_pattern ).\nCLEAR: source, posix, excluded, ignore_case, global.\n", 1)
	arr := e.typ(hir.T(hir.Array, hir.T(hir.String)))
	text = strings.Replace(text, "PUBLIC SECTION.", "PUBLIC SECTION.\nCLASS-METHODS exec_legacy IMPORTING p0 TYPE REF TO "+base+" p1 TYPE string RETURNING VALUE(result) TYPE "+arr+".\n", 1)
	text = strings.TrimSuffix(text, "ENDCLASS.\n") + `METHOD exec_legacy.
DATA js TYPE REF TO ` + name + `.
TRY.
  js ?= p0.
CATCH cx_sy_move_cast_error.
ENDTRY.
IF js IS NOT BOUND.
  CREATE OBJECT js EXPORTING pattern = p0->source flags = p0->flags checked = abap_true.
ENDIF.
js->last_index = p0->last_index.
result = js->exec( p1 ).
p0->last_index = js->last_index.
ENDMETHOD.
ENDCLASS.
`
	e.files[file] = text
}
