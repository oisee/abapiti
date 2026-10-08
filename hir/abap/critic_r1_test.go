package abap

import (
	"strings"
	"testing"
)

func TestLiteralChunkUnicodeMode(t *testing.T) {
	for _, phrase := range literalModePhrases {
		remaining := []rune("€€€€€€ " + phrase + " end")
		for len(remaining) > 0 {
			n := literalChunkLimit(remaining, len(remaining))
			if strings.Contains(string(remaining[:n]), phrase) {
				t.Fatalf("unsplit phrase: %q", remaining[:n])
			}
			remaining = remaining[n:]
		}
	}
}
