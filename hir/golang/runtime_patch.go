package golang

import (
	"strconv"
	"strings"
)

// Runtime source is patched by needle. A needle that no longer matches must
// fail loudly: a silently skipped patch corrupts emitted programs (the join
// rewrite in 486f6cc7 left [1,,3].join(",") producing "1,0,3").
func mustReplace(source, old, replacement, what string) string {
	if n := strings.Count(source, old); n != 1 {
		panic("runtime patch target moved: " + what + " occurs " + strconv.Itoa(n) + " times, want 1")
	}
	return strings.Replace(source, old, replacement, 1)
}

// optionalReplace patches a needle that documented configurations may omit,
// for example a method another runtime appends only when its feature is used.
// Absence is a no-op; more than one occurrence is still ambiguous and panics.
func optionalReplace(source, old, replacement, what string) string {
	if n := strings.Count(source, old); n > 1 {
		panic("runtime patch target ambiguous: " + what + " occurs " + strconv.Itoa(n) + " times, want at most 1")
	}
	return strings.Replace(source, old, replacement, 1)
}

// mustContain guards ReplaceAll sites where several matches are expected.
func mustContain(source, needle, what string) string {
	if !strings.Contains(source, needle) {
		panic("runtime patch target moved: " + what)
	}
	return source
}

// mustIndex locates a splice boundary; a -1 would silently corrupt the text.
func mustIndex(source, needle, what string) int {
	i := strings.Index(source, needle)
	if i < 0 {
		panic("runtime patch target moved: " + what)
	}
	return i
}
