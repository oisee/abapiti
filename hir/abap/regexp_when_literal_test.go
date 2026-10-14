package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/jsregex"
)

// A real kernel refuses an expression after WHEN; only & may join literals.
func TestRegExpDynamicWhenKeysAreLiterals(t *testing.T) {
	for k := range jsregex.Reviewed {
		if _, ok := regexABAPLiteral(k); !ok {
			t.Fatalf("reviewed regex key is not a compile-time literal and would be refused: %q", k)
		}
	}
	long := strings.Repeat("a", 100)
	got, ok := regexABAPLiteral(long)
	if !ok || strings.Contains(got, "&&") || !strings.Contains(got, " &\n") {
		t.Fatalf("long key must be split with the literal operator &: %q", got)
	}
	if _, ok := regexABAPLiteral(strings.Repeat("a", 256)); ok {
		t.Fatal("keys over 255 characters are not literals")
	}
	if _, ok := regexABAPLiteral("a\nb"); ok {
		t.Fatal("control characters cannot be part of a literal")
	}
}
