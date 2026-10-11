package abap

import "testing"

func TestPCREReviewedPatternSyntax(t *testing.T) {
	cases := map[string]string{
		`^a$`:                                 `(?<![\s\S])a(?![\s\S])`,
		`[^^$]\^\$`:                           `[^^$]\^\$`,
		`[\x{0009}-\x{000D}\x{00A0}\x{2028}]`: "[\\x09-\\x0D\\xA0\u2028]",
		`\b[dD][eE][fF][iI][nN][eE]`:          `(?<![A-Za-z0-9_])[dD][eE][fF][iI][nN][eE]`,
		`\\x{2028}`:                           `\\x{2028}`,
		`a b#c[ #]`:                           `a\x20b\x23c[ #]`,
	}
	for in, want := range cases {
		if got := pcreRegexPattern(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
