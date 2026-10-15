package golang

import (
	"crypto/sha256"
	"fmt"
	"go/scanner"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// internStrLiterals hoists every str("constant") call of the generated
// sources into one package variable, so a hot path (an own-key write, a
// property case) no longer re-encodes a literal to UTF-16 and allocates on
// each call. Calls are found with the Go scanner (identifier str, "(", an
// interpreted string literal, ")"), so text inside raw strings, string
// literals and comments is never rewritten. jsString values are immutable.
func internStrLiterals(files map[string]string, names []string) string {
	vars := map[string]string{}
	for _, n := range names {
		files[n] = rewriteStrCalls(files[n], vars)
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

func rewriteStrCalls(src string, vars map[string]string) string {
	type tok struct {
		off int
		t   token.Token
		lit string
	}
	var toks []tok
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var sc scanner.Scanner
	sc.Init(file, []byte(src), nil, 0)
	for {
		pos, t, lit := sc.Scan()
		if t == token.EOF {
			break
		}
		toks = append(toks, tok{file.Offset(pos), t, lit})
	}
	var b strings.Builder
	last := 0
	for i := 0; i+3 < len(toks); i++ {
		a, open, s, closing := toks[i], toks[i+1], toks[i+2], toks[i+3]
		if a.t != token.IDENT || a.lit != "str" || open.t != token.LPAREN || s.t != token.STRING || !strings.HasPrefix(s.lit, "\"") || closing.t != token.RPAREN {
			continue
		}
		if i > 0 && (toks[i-1].t == token.FUNC || toks[i-1].t == token.PERIOD) {
			continue
		}
		if _, err := strconv.Unquote(s.lit); err != nil {
			continue
		}
		name, ok := vars[s.lit]
		if !ok {
			name = fmt.Sprintf("strLit_%x", sha256.Sum256([]byte(s.lit)))[:23]
			vars[s.lit] = name
		}
		b.WriteString(src[last:a.off])
		b.WriteString(name)
		last = closing.off + 1
		i += 3
	}
	b.WriteString(src[last:])
	return b.String()
}

// internLiteralFiles adds literals.go for hir.go and runtime.go.
func internLiteralFiles(files map[string]string, pkg string) {
	files["literals.go"] = "package " + pkg + "\n" + internStrLiterals(files, []string{"hir.go", "runtime.go"})
}

// discardedResult selects a statement form that skips building a value
// nobody reads: splice(i, n) as a statement keeps the mutation but not the
// removed-elements array. Everything else is returned unchanged.
func discardedResult(x *hir.Expr) *hir.Expr {
	if x != nil && x.Kind == hir.RuntimeOp && x.Op == "array.splice2" {
		drop := *x
		drop.Op = "array.spliceDrop2"
		return &drop
	}
	return x
}
