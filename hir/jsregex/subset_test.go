package jsregex

import "testing"

// a{2} was G's old refusal; F's Node-certified bounds extend the domain.
func TestSubsetCertifiedBoundedRepeat(t *testing.T) {
	if got, ok := TranslateSubset("a{2}", ""); !ok || got != "a{2}" {
		t.Fatalf("bounded repeat: %q, %v", got, ok)
	}
}

func TestSubsetRefusesUnmodeledSyntax(t *testing.T) {
	for _, pattern := range []string{"", `(a)*`, `(?:a(b)?)+`, `(a){2}`, `(a){1,}`, `a{2,1}`, `a{1001}`, `a{01}`, `\b*`, `\B?`, `\1`, `\p{Letter}`, `(?=a)`, `(?!a)`, `a**`, `a*??`, `a{2}??`, `(a)*?`, `(?:a(b)?)+?`, `a$b`, `^é`, `a|`, `|a`, `()`, `[a-\w]`, `[\W_]`, `[\s-a]`, `[a-\s]`, `\b+`} {
		flags := ""
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
