package jsregex

import "testing"

func TestSubsetRefusesUnmodeledSyntax(t *testing.T) {
	for _, pattern := range []string{`(a)`, `a|b`, `a{2}`, `\1`, `\w`, `\p{Letter}`, `[a-z]`, `(?=a)`, `a*?`, `a**`, `a$b`, `^é`} {
		flags := ""
		if pattern == `[a-z]` {
			flags = "i"
		}
		if translated, ok := TranslateSubset(pattern, flags); ok {
			t.Errorf("accepted %q/%s as %q", pattern, flags, translated)
		}
	}
	for _, flags := range []string{"ii", "u", "s", "m", "d", "v", "z"} {
		if _, ok := TranslateSubset("a", flags); ok {
			t.Errorf("accepted flags %q", flags)
		}
	}
}
