package hir

import (
	"strings"
	"testing"
)

func TestReadableNamesMode(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"", true}, {"readable", true}, {"hashed", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("ABAPITI_NAMES", tc.value)
			got, err := ReadableNames()
			if err != nil || got != tc.want {
				t.Fatalf("ReadableNames() = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	for _, value := range []string{"typo", "READABLE", " hashed "} {
		t.Setenv("ABAPITI_NAMES", value)
		if _, err := ReadableNames(); err == nil || !strings.Contains(err.Error(), "ABAPITI_NAMES") || !strings.Contains(err.Error(), "readable or hashed") {
			t.Fatalf("unclear error for %q: %v", value, err)
		}
	}
}
