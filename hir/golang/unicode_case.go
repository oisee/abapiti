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
	lower := cases.Lower(language.Und)
	b.WriteString("var lowerExpansion = map[rune]string{\n")
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		target := lower.String(string(r))
		if target != string(unicode.ToLower(r)) {
			fmt.Fprintf(&b, "%d:%q,\n", r, target)
		}
	}
	b.WriteString("}\nvar caseIgnorable=map[rune]bool{\n")
	// Ask the same Unicode casing tables for the final-sigma context property.
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !(unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Lm, r) || unicode.Is(unicode.Sk, r) || unicode.IsPunct(r)) {
			continue
		}
		a, bContext := []rune(lower.String("AΣ"+string(r)+"A")), []rune(lower.String("AΣ"+string(r)+" "))
		if len(a) > 1 && len(bContext) > 1 && a[1] == 'σ' && bContext[1] == 'ς' {
			fmt.Fprintf(&b, "%d:true,\n", r)
		}
	}
	b.WriteString("}\n")
	return b.String()
}()
