package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func lowerProbe(t *testing.T, dir string) (*hir.Program, []LowerDiagnostic) {
	t.Helper()
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.Lower([]string{"input.ts"})
	if err != nil {
		t.Fatal(err)
	}
	return prog, diags
}

// These sources are the critic's original probes, retained for independent
// regression coverage rather than rebuilt from the implementation.
func TestCriticR1Diagnostics(t *testing.T) {
	for name, category := range map[string]string{
		"default": "unsupported-param-default", "default_required": "unsupported-param-default", "before_super_this": "unsupported-before-super", "radix": "unsupported-call",
		"regex_anchor": "unsupported-regex", "regex_replacement": "unsupported-regex",
		"regex_sideeffect": "unsupported-regex", "top_level": "unsupported-top-level",
		"unproven_as": "unsupported-assertion", "super_reorder": "unsupported-top-level",
		"constructor_timing": "unsupported-static-init",
	} {
		t.Run(name, func(t *testing.T) {
			_, diags := lowerProbe(t, filepath.Join("testdata", "critic-r1", name))
			for _, d := range diags {
				if d.Category == category {
					if !strings.Contains(d.Loc, "input.ts:") {
						t.Fatalf("missing source location: %v", d)
					}
					return
				}
			}
			t.Fatalf("missing %s: %v", category, diags)
		})
	}
}

func TestCriticR1Accepted(t *testing.T) {
	for _, name := range []string{"implicit_init", "static_init", "super_preserved", "optional_downcast", "unicode_mode", "overflow", "inherited_init", "static_order", "number_semantics", "proven_as", "regex_literals", "unicode_upper", "supplementary", "unicode_codes", "unicode_trim"} {
		t.Run(name, func(t *testing.T) {
			prog, diags := lowerProbe(t, filepath.Join("testdata", "critic-r1", name))
			for _, d := range diags {
				if !strings.HasPrefix(d.Category, "note-") {
					t.Fatal(d)
				}
			}
			if errs := hir.Verify(prog); len(errs) > 0 {
				t.Fatal(errs)
			}
			files, names, err := abap.EmitNamed(prog)
			if err != nil {
				t.Fatal(err)
			}
			var probe *hir.Class
			for _, c := range prog.Classes {
				if strings.HasSuffix(c.Name, ".Probe") {
					probe = c
				}
			}
			if probe == nil {
				t.Fatal("no Probe class")
			}
			source := files[names.Get(probe.Name)+".clas.abap"]
			// Each emitted probe carries a runtime oracle independent of HIR shape.
			var check string
			switch name {
			case "implicit_init":
				check = "actual = obj->" + names.Get("member.run") + "( ).\nexpected = 7."
			case "static_init":
				check = "actual = " + names.Get(probe.Name) + "=>" + names.Get("member.run") + "( ).\nexpected = 7."
			case "super_preserved":
				check = "actual = obj->" + names.Get("member.run") + "( ).\nexpected = 1."
			case "overflow":
				check = "actual = obj->" + names.Get("member.run") + "( ).\nexpected = '2147483648'."
			case "optional_downcast", "proven_as":
				var sub *hir.Class
				for _, c := range prog.Classes {
					if strings.HasSuffix(c.Name, ".Sub") {
						sub = c
					}
				}
				check = "DATA sub TYPE REF TO " + names.Get(sub.Name) + ".\nCREATE OBJECT sub.\nactual = obj->" + names.Get("member.run") + "( " + names.Get("param.p") + " = sub ).\nexpected = 2."
			case "inherited_init":
				check = "actual = obj->" + names.Get("member.run") + "( ).\nexpected = 578."
			case "static_order":
				check = "actual = " + names.Get(probe.Name) + "=>" + names.Get("member.run") + "( ).\nexpected = 78."
			case "regex_literals":
				check = "text = obj->" + names.Get("member.run") + "( ).\ncl_abap_unit_assert=>assert_equals( act = text exp = `XcXc` )."
			case "unicode_upper":
				check = "text = obj->" + names.Get("member.run") + "( ).\ncl_abap_unit_assert=>assert_equals( act = text exp = `ÄÖÜSSFFIΪ́` )."
			case "unicode_codes":
				check = "actual = obj->" + names.Get("member.run") + "( ).\nexpected = 8364."
			case "unicode_trim":
				check = "text = obj->" + names.Get("member.run") + "( ).\ncl_abap_unit_assert=>assert_equals( act = text exp = `abc` )."
			case "supplementary":
				check = "DATA conv TYPE REF TO cl_abap_conv_out_ce.\nDATA bytes TYPE xstring.\nDATA expected_bytes TYPE xstring.\nactual = obj->" + names.Get("member.length") + "( ).\nexpected = 2.\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\ntext = obj->" + names.Get("member.run") + "( ).\nconv = cl_abap_conv_out_ce=>create( encoding = '4103' ).\nCALL METHOD conv->convert EXPORTING data = text IMPORTING buffer = bytes.\nexpected_bytes = '3DD8'.\ncl_abap_unit_assert=>assert_equals( act = bytes exp = expected_bytes )."
			case "number_semantics":
				check = "actual = obj->" + names.Get("member.run") + "( ).\nexpected = '2147483648'.\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\ntext = obj->" + names.Get("member.indices") + "( ).\ncl_abap_unit_assert=>assert_equals( act = text exp = `bc` ).\nactual = obj->" + names.Get("member.remainder") + "( ).\nexpected = '-1.5'."
			case "unicode_mode":
				check = "text = obj->" + names.Get("member.run") + "( ).\ncl_abap_unit_assert=>assert_equals( act = text exp = `€€€€€€ IN BYTE MODE end` )."
			}
			files[names.Get(probe.Name)+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS regression FOR TESTING.
ENDCLASS.
CLASS ltcl_probe IMPLEMENTATION.
METHOD regression.
DATA obj TYPE REF TO %s.
DATA actual TYPE f.
DATA expected TYPE f.
DATA text TYPE string.
%s
%s
cl_abap_unit_assert=>assert_equals( act = actual exp = expected ).
ENDMETHOD.
ENDCLASS.
`, names.Get(probe.Name), func() string {
				if name == "inherited_init" {
					return "CREATE OBJECT obj EXPORTING " + names.Get("param.n") + " = 5."
				}
				return "CREATE OBJECT obj."
			}(), check)

			switch name {
			case "implicit_init":
				if probe.Ctor == nil || !strings.Contains(hir.Dump(prog), "7") {
					t.Fatal("initializer lost")
				}
			case "static_init":
				found := false
				for _, m := range probe.Methods {
					if m.Name == "class_constructor" {
						found = true
					}
				}
				if !found {
					t.Fatal("static initializer lost")
				}
			case "optional_downcast":
				if !strings.Contains(source, " ?= ") {
					t.Fatal("optional downcast needs checked assignment")
				}
			case "unicode_mode":
				if !strings.Contains(source, "IN BYTE MODE") {
					t.Fatal("mode phrase should remain in the literal")
				}
			case "super_preserved":
				if probe.Ctor.Body.List[0].Kind != hir.Assign || probe.Ctor.Body.List[1].X.Kind != hir.SuperCall {
					t.Fatal("pre-super statement reordered")
				}
			case "overflow":
				if !strings.Contains(source, "TYPE f") {
					t.Fatal("number must use binary64")
				}
			}
			if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
				dir := filepath.Join(out, "critic-r1", name)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for file, src := range files {
					if err := os.WriteFile(filepath.Join(dir, file), []byte(src), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestStringBuildModeLiteral(t *testing.T) {
	for _, phrase := range []string{"IN BYTE MODE", "IN CHARACTER MODE"} {
		value := "€€€€€€ " + phrase + " end"
		lines := abapStringBuild("raw", "ch", value)
		if len(lines) != 1 || !strings.Contains(lines[0], "`"+value+"`") {
			t.Fatalf("literal was split: %v", lines)
		}
	}
}

func TestCriticR1RejectVariants(t *testing.T) {
	for name, body := range map[string]string{
		"anchor_end":          `return "abc".replace(/$/g, "X");`,
		"replacement_capture": `return "abc".replace(/a/g, "$1");`,
		"replacement_dynamic": `const replacement = "X"; return "abc".replace(/a/g, replacement);`,
		"radix_side_effect":   `let n = 0; return (16).toString(n = n + 1);`,
		"regex_octal":         `return "abc".replace(/\01/g, "X");`,
		"number_remainder":    `return 5 % 3;`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			config, err := os.ReadFile(filepath.Join("testdata", "critic-r1", "implicit_init", "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), config, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "input.ts"), []byte("export class Probe { run() { "+body+" } }"), 0644); err != nil {
				t.Fatal(err)
			}
			_, diags := lowerProbe(t, dir)
			for _, d := range diags {
				if strings.HasPrefix(d.Category, "unsupported-") {
					return
				}
			}
			t.Fatalf("silently accepted: %v", diags)
		})
	}
}
