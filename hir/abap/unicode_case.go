package abap

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ECMAScript uses default Unicode full uppercase mappings. TRANSLATE on the
// pinned Go runtime implements simple mappings, which lose expansions such
// as sharp s and ligatures. Apply the full-mapping differences first; their
// outputs contain no lowercase sources, so replacements cannot cascade.
var fullUpperMappings = func() [][2]string {
	upper := cases.Upper(language.Und)
	var out [][2]string
	for r := rune(0); r <= 0xffff; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		source := string(r)
		target := upper.String(source)
		if target != string(unicode.ToUpper(r)) {
			out = append(out, [2]string{source, target})
		}
	}
	return out
}()

// asciiPrintable is the CO set of the toUpperCase fast path: ASCII 32..126 as
// a string literal (backquote doubled).
var asciiPrintable = func() string {
	var b strings.Builder
	b.WriteByte('`')
	for c := byte(32); c <= 126; c++ {
		if c == '`' {
			b.WriteByte('`')
		}
		b.WriteByte(c)
	}
	b.WriteByte('`')
	return b.String()
}()

// upperRuntime emits, once, the class whose full( ) applies JS's full
// upper-case mappings and returns its name.
func (e *emitter) upperRuntime() string {
	name := e.name("runtime.upper")
	if e.types["runtime.upper"] {
		return name
	}
	e.types["runtime.upper"] = true
	var impl strings.Builder
	impl.WriteString("result = p0.\n")
	for _, mapping := range fullUpperMappings {
		impl.WriteString("REPLACE ALL OCCURRENCES OF `" + mapping[0] + "` IN result WITH `" + mapping[1] + "`.\n")
	}
	impl.WriteString("TRANSLATE result TO UPPER CASE.\n")
	e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC FINAL CREATE PUBLIC.\nPUBLIC SECTION.\nCLASS-METHODS full IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE string.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nMETHOD full.\n" + impl.String() + "ENDMETHOD.\nENDCLASS.\n"
	return name
}
