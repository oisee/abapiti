package jsregex

import (
	"bytes"
	"encoding/base64"
	"os/exec"
	"regexp"
	"testing"
)

func TestDelimitedQuoteLanguageExhaustiveNodeOracle(t *testing.T) {
	const pattern = `^'(?:[^'\\]|''|\\'|\\\\|\\(?!'))*'$`
	translated, ok := TranslateSubset(pattern, "")
	if !ok {
		t.Fatal("refused delimited-string language")
	}
	re, err := regexp.Compile(translated)
	if err != nil {
		t.Fatal(err)
	}
	// 349,525 strings: all lengths 0..9 over quote, backslash, ordinary
	// character and newline, in exactly the same traversal order as Node.
	script := `const p=new RegExp(process.argv[1]);const a=["'","\\","a","\n"];const out=[];function walk(s,n){out.push(p.test(s)?1:0);if(n)for(const c of a)walk(s+c,n-1)}walk("",9);console.log(Buffer.from(out).toString("base64"))`
	want, err := exec.Command("node", "-e", script, pattern).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	expected, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(want)))
	if err != nil {
		t.Fatal(err)
	}
	at := 0
	var walk func(string, int)
	walk = func(s string, n int) {
		got := byte(0)
		if re.MatchString(s) {
			got = 1
		}
		if got != expected[at] {
			t.Fatalf("language mismatch for %q: Go %d Node %d", s, got, expected[at])
		}
		at++
		if n > 0 {
			for _, c := range []string{"'", "\\", "a", "\n"} {
				walk(s+c, n-1)
			}
		}
	}
	walk("", 9)
	if at != len(expected) {
		t.Fatal(at, len(expected))
	}
	for _, flags := range []string{"i", "m", "s", "u"} {
		if _, ok := TranslateSubset(pattern, flags); ok {
			t.Fatalf("unreviewed flags %s", flags)
		}
	}
}
