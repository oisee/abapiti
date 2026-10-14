package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// OSG admits unanchored PCRE on multiline input. Unicode normalization must
// use the ordinary checked-regex path, not a newline-guard special case.
// Execution coverage lives in TestAbaplintTestHostOSGEmission.
func TestUnicodeRegexReplacementPCRE(t *testing.T) {
	for _, pattern := range []string{`\u00a0`, `\u000b`, `\s+`, `[\w\d]+`} {
		t.Run(pattern, func(t *testing.T) {
			r := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, pattern), hir.L(str, "g")}}
			x := rt("string.replaceRegex", hir.L(str, "a\n\u00a0b\vc😀\u2028\ufeff"), str, r, hir.L(str, " "))
			// Observed RegExp state selects the checked JS/PCRE subclass.
			result := hir.T(hir.Optional, hir.T(hir.Array, str))
			exec := method("exec", result, ret(rt("regexp.exec", r, result, hir.L(str, "a"))))
			p := &hir.Program{Classes: []*hir.Class{{Name: "UnicodeReplace", Methods: []*hir.Method{method("run", str, ret(x)), exec}}}}
			files, names, err := emitABAPNamed(t, p)
			if err != nil {
				t.Fatal(err)
			}
			caller := files[names.Get("UnicodeReplace")+".clas.abap"]
			if strings.Contains(caller, "REPLACE ALL OCCURRENCES OF ") || !strings.Contains(caller, "->replace( p0") {
				t.Fatal("checked regex must not use the legacy substring specialization")
			}
			runtime := files[names.Get("runtime.regexp_js")+".clas.abap"]
			if !strings.Contains(runtime, "FIND ALL OCCURRENCES OF PCRE posix IN text RESULTS matches.") {
				t.Fatal("replacement must use checked PCRE match offsets")
			}
			if pattern == `\s+` {
				for _, spelling := range []string{"\u00a0", "cl_abap_conv_in_ce=>uccpi( 8232 )", "cl_abap_conv_in_ce=>uccpi( 65279 )"} {
					if !strings.Contains(runtime, spelling) {
						t.Fatalf("JS whitespace spelling %q missing from explicit class", spelling)
					}
				}
			}
			if pattern == `[\w\d]+` && !strings.Contains(runtime, "[A-Za-z0-9_0-9]+") {
				t.Fatal("JS word and digit shorthands must expand to ASCII classes")
			}
		})
	}
}
