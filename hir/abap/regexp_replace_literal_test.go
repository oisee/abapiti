package abap

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestFreshCharacterRegexReplacement(t *testing.T) {
	for _, pattern := range []string{`\u00a0`, `\u000b`} {
		t.Run(pattern, func(t *testing.T) {
			r := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, pattern), hir.L(str, "g")}}
			x := rt("string.replaceRegex", hir.L(str, "a\n\u00a0b\vc😀\u00a0\v"), str, r, hir.L(str, " "))
			p := &hir.Program{Classes: []*hir.Class{{Name: "CharacterReplace", Methods: []*hir.Method{method("run", str, ret(x))}}}}
			if errs := hir.Verify(p); len(errs) != 0 {
				t.Fatal(errs)
			}
			files, err := emitABAP(t, p)
			if err != nil {
				t.Fatal(err)
			}
			text := ""
			for _, file := range files {
				text += file
			}
			if !strings.Contains(text, "REPLACE ALL OCCURRENCES OF ") || strings.Contains(text, "->replace( p0") || strings.Contains(text, "checked = abap_true") {
				t.Fatal("fresh singleton regex must use exact substring replacement")
			}
		})
	}
}

func TestFreshCharacterRegexReplacementNodeOracle(t *testing.T) {
	inputs := []string{"", "\r\n", "\u00a0\v\u00a0\v", " a\n\u00a0b\vc😀\u00a0\v ", "\u0085\u180e\ufeff"}
	raw, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(s=>s.replace(/\u00a0/g," ").replace(/\u000b/g," "))))`, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v\n%s", err, out)
	}
	var want []string
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatal(err)
	}
	for i, input := range inputs {
		got := strings.ReplaceAll(strings.ReplaceAll(input, "\u00a0", " "), "\v", " ")
		if got != want[i] {
			t.Fatalf("substring normalization %q != Node %q", got, want[i])
		}
	}
}

func TestFreshCharacterRegexReplacementBoundary(t *testing.T) {
	for _, pattern := range []string{`\u00a0`, `\u000b`, "a", `\s+`} {
		for _, flags := range []string{"g", "", "gi"} {
			for _, with := range []string{" ", "$&"} {
				r := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, pattern), hir.L(str, flags)}}
				x := rt("string.replaceRegex", hir.L(str, "a"), str, r, hir.L(str, with))
				// Only negative cases: do not bypass the fixture-collecting emitter
				// for a successful emission, even when checking an optimization.
				if flags == "g" && with == " " && (pattern == `\u00a0` || pattern == `\u000b`) {
					continue
				}
				if (*body)(nil).replaceFreshCharacterRegex(x, "result") {
					t.Fatal("observed/unsupported regex or dollar substitution specialized")
				}
			}
		}
	}
	// An existing RegExp receiver may carry observable lastIndex.
	x := rt("string.replaceRegex", hir.L(str, "a"), str, hir.V("regex", hir.T(hir.RegExp)), hir.L(str, " "))
	if (*body)(nil).replaceFreshCharacterRegex(x, "result") {
		t.Fatal("existing receiver specialized")
	}
}
