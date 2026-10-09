package golang

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Emit full Unicode expansion mappings once; generated programs use only stdlib.
var unicodeUpperSource = func() string {
	upper := cases.Upper(language.Und)
	var b strings.Builder
	b.WriteString("\nvar upperExpansion = map[rune]string{\n")
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		s := upper.String(string(r))
		if s != string(unicode.ToUpper(r)) {
			fmt.Fprintf(&b, "%d:%q,\n", r, s)
		}
	}
	b.WriteString("}\n")
	return b.String()
}()
