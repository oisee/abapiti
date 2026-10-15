package golang

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// strLiteralCall matches str("…") with a Go-quoted constant argument.
var strLiteralCall = regexp.MustCompile(`\bstr\(("(?:[^"\\\n]|\\.)*")\)`)

// internStrLiterals hoists every str("constant") of the generated sources
// into one package variable, so a hot path (an own-key write, a property
// case) no longer re-encodes a literal to UTF-16 and allocates on each call.
// The variable is initialised once; jsString values are immutable.
func internStrLiterals(files map[string]string, names []string) string {
	vars := map[string]string{}
	for _, n := range names {
		files[n] = strLiteralCall.ReplaceAllStringFunc(files[n], func(call string) string {
			quoted := strLiteralCall.FindStringSubmatch(call)[1]
			if _, err := strconv.Unquote(quoted); err != nil {
				return call
			}
			name, ok := vars[quoted]
			if !ok {
				name = fmt.Sprintf("strLit_%x", sha256.Sum256([]byte(quoted)))[:23]
				vars[quoted] = name
			}
			return name
		})
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "var %s = str(%s)\n", vars[k], k)
	}
	return b.String()
}

// internLiteralFiles adds literals.go for hir.go and runtime.go.
func internLiteralFiles(files map[string]string, pkg string) {
	files["literals.go"] = "package " + pkg + "\n" + internStrLiterals(files, []string{"hir.go", "runtime.go"})
}
