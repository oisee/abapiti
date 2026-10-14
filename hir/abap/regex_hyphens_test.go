package abap

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestRegexClassHyphensNodeOracle(t *testing.T) {
	patterns := []string{`^[A-Z_-\d<> ]+$`, `^[A-Z_-\d/<> ]+$`, `^[-a-z]+$`, `^[a-z-]+$`, `^[a-z-A]+$`, `^[a-\d]+$`, `^[\d-a]+$`, `^[a\-z]+$`, `^[--a]+$`, `^[-\s]+$`, `^[^a-z-]+$`, `^[\[-\]]+$`, `^[α-ω-a]+$`}
	raw, _ := json.Marshal(patterns)
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(p=>Array.from({length:128},(_,i)=>new RegExp(p).test(String.fromCharCode(i))))))`, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	var wants [][]bool
	if err := json.Unmarshal(out, &wants); err != nil {
		t.Fatal(err)
	}
	receiver := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{local("pattern", str), hir.L(str, "")}}
	m := method("check", boolean, ret(rt("regexp.test", receiver, boolean, local("input", str))))
	m.Static, m.Params = true, []hir.Param{{Name: "pattern", Type: str}, {Name: "input", Type: str}}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "HyphenProbe", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("HyphenProbe")
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA pattern TYPE string.\nDATA input TYPE string.\n")
	for p, pattern := range patterns {
		translated := regexClasses(pattern)
		// PCRE also sees the original syntax on reviewed/dynamic routes.
		pcre := pcreRegexPattern(strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$"))
		if _, err := regexp.Compile(pcre); err != nil {
			t.Fatalf("invalid PCRE class normalization %q: %v", pcre, err)
		}
		re, err := regexp.Compile(translated)
		if err != nil {
			t.Fatalf("%s -> %s: %v", pattern, translated, err)
		}
		if again := regexClasses(translated); again != translated {
			t.Fatalf("not idempotent: %q -> %q", translated, again)
		}
		fmt.Fprintf(&unit, "pattern = %s.\n", regexABAPString(pattern))
		for i, want := range wants[p] {
			if got := re.MatchString(string(rune(i))); got != want {
				t.Errorf("/%s/ char %d: %v, Node %v", pattern, i, got, want)
			}
			// UCCPI(0) is outside the host's checked conversion domain.
			if i == 0 {
				continue
			}
			expected := "abap_false"
			if want {
				expected = "abap_true"
			}
			fmt.Fprintf(&unit, "input = %s.\ncl_abap_unit_assert=>assert_equals( act = %s=>%s( %s = pattern %s = input ) exp = %s msg = 'pattern %d char %d' ).\nFIND PCRE %s IN input.\ncl_abap_unit_assert=>assert_equals( act = xsdbool( sy-subrc = 0 ) exp = %s ).\n", regexABAPString(string(rune(i))), owner, names.Get("member.check"), names.Get("param.pattern"), names.Get("param.input"), expected, p, i, regexABAPString(pcre), expected)
		}
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}
