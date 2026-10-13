package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func regexNew(p *hir.Expr, flags string) *hir.Expr {
	return &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{p, hir.L(str, flags)}}
}

func TestLegacyDynamicRegexDoesNotEnablePCRE(t *testing.T) {
	m := method("test", hir.T(hir.Bool), ret(rt("regexp.test", regexNew(hir.V("pattern", str), "i"), hir.T(hir.Bool), hir.L(str, "abc"))))
	m.Params = []hir.Param{{Name: "pattern", Type: str}}
	files, err := emitABAP(t, &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	for file, text := range files {
		if strings.Contains(text, "PCRE") || strings.Contains(text, "checked =") || strings.Contains(text, "exec_legacy") {
			t.Fatalf("legacy dynamic regex upgraded in %s", file)
		}
	}
}

func TestMixedRegexRuntimes(t *testing.T) {
	result := hir.T(hir.Optional, hir.T(hir.Array, str))
	legacy := regexNew(hir.L(str, "^abc$"), "i")
	exec := regexNew(hir.L(str, "(a)(b)"), "g")
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{
		method("plain", hir.T(hir.Bool), ret(rt("regexp.test", legacy, hir.T(hir.Bool), hir.L(str, "abc")))),
		method("exec", result, ret(rt("regexp.exec", exec, result, hir.L(str, "zab")))),
	}}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	base := names.Get("runtime.regexp")
	js := names.Get("runtime.regexp_js")
	if strings.Contains(files[base+".clas.abap"], "PCRE") {
		t.Fatal("PCRE leaked into the legacy runtime")
	}
	for file, text := range files {
		if strings.Contains(text, "PCRE") && file != js+".clas.abap" {
			t.Errorf("PCRE leaked into %s", file)
		}
	}
	text := files[names.Get("Probe")+".clas.abap"]
	for _, want := range []string{"TYPE " + base + " EXPORTING", "TYPE " + js + " EXPORTING", js + "=>exec_legacy"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s", want)
		}
	}
	if !strings.Contains(files[js+".clas.abap"], "INHERITING FROM "+base) {
		t.Fatal("JS regex must be assignable to the existing RegExp ABI")
	}
}

func TestRegexJSRequirements(t *testing.T) {
	for _, c := range []struct {
		p, f string
		js   bool
	}{
		{"^abc$", "i", false},
		{`^(?!(?:IF|ELSE)$)[A-Z]+$`, "", false},
		{`[?*+]`, "", false},
		{`[(?<]`, "", false},
		{`\(\?<x`, "", false},
		{`a\*\?`, "", false},
		{`(?<=a)b`, "", true},
		{`(?!a)\w+`, "", true},
		{`a+?`, "", true},
		{`a{1,3}?`, "", true},
		{"a", "y", true},
		{"a", "m", true},
	} {
		if got := regexNeedsJS(regexNew(hir.L(str, c.p), c.f)); got != c.js {
			t.Errorf("/%s/%s: JS = %v, want %v", c.p, c.f, got, c.js)
		}
	}
}

func TestLazyMatchEnablesOnlyJSRegexRuntime(t *testing.T) {
	pattern := regexNew(hir.L(str, `\bdefine\s+(?:\w+\s+)*?behavior\s+for\s+([\w/]+)(?:\s+alias\s+(\w+))?`), "gi")
	result := hir.T(hir.Optional, hir.T(hir.Array, str))
	m := method("match", result, ret(rt("string.match", hir.L(str, "DEFINE behavior for ZBOOK"), result, pattern)))
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	jsFile := names.Get("runtime.regexp_js") + ".clas.abap"
	if !strings.Contains(files[jsFile], "FIND FIRST OCCURRENCE OF PCRE search") {
		t.Fatal("lazy match requires the JS runtime even without an exec call")
	}
	for file, text := range files {
		if strings.Contains(text, "PCRE") && file != jsFile {
			t.Errorf("PCRE leaked into %s", file)
		}
	}
}

func TestPrunedRegexDoesNotEnablePCRE(t *testing.T) {
	// An unreachable declaration/body is already replaced with Trap before
	// emission. RegExp declarations alone must not activate the JS runtime.
	m := method("unexecuted", hir.T(hir.RegExp), &hir.Stmt{Kind: hir.Trap, Name: "not executed"})
	files, err := emitABAP(t, &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	for file, text := range files {
		if strings.Contains(text, "PCRE") {
			t.Fatalf("pruned body enabled PCRE in %s", file)
		}
	}
}
