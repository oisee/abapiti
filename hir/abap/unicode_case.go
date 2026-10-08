package abap

import (
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
