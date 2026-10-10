package main

import (
	"reflect"
	"testing"
)

func TestSplitABAPLiteralsCommentsAndDepth(t *testing.T) {
	src := `* comment.
METHOD m.
DATA(t1) = CONV string( |a.b| ). " trailing comment.
LOOP AT tab INTO row.
IF row = 'it''s.a'.
x = 1.25.
ENDIF.
ENDLOOP.
ENDMETHOD.
`
	ms := parseMethods(src)
	if len(ms) != 1 {
		t.Fatal(ms)
	}
	m := ms[0]
	if len(m.Statements) != 6 {
		t.Fatalf("statements: %#v", m.Statements)
	}
	var depths []int
	for _, s := range m.Statements {
		depths = append(depths, s.Depth)
	}
	if !reflect.DeepEqual(depths, []int{0, 0, 1, 1, 1, 0}) {
		t.Fatal(depths)
	}
	if m.Statements[0].Line != 3 {
		t.Fatal(m.Statements[0])
	}
}
func TestCandidateSafetyBoundaries(t *testing.T) {
	cases := []struct {
		name, body string
		want       map[string]int
	}{
		{"literal is not use", `DATA(t1) = CAST cls( obj ). x = t1. text = 't1'.`, map[string]int{"cast_temp": 1, "temp_assign": 1}},
		{"multiple use", `DATA(t1) = CAST cls( obj ). x = t1. y = t1.`, map[string]int{}},
		{"overwrite before read", `DATA(t1) = VALUE i( ). DATA(t2) = CONV i( '3' ). t1 = t2. result = t1.`, map[string]int{"value_init_overwritten": 1, "temp_assign": 2, "return_copy": 1, "literal_scalar": 1}},
		{"branch stops init", `DATA(t1) = VALUE i( ). IF cond = abap_true. t1 = 1. ENDIF. result = t1.`, map[string]int{}},
		{"self read", `DATA(t1) = VALUE i( ). t1 = t1 + 1.`, map[string]int{}},
		{"bool assignment", `DATA(t1) = VALUE abap_bool( ). t1 = xsdbool( x = y ). IF t1 = abap_true. ENDIF.`, map[string]int{"value_init_overwritten": 1, "bool_if": 1}},
		{"clear overwrite", `DATA t1 TYPE REF TO cls. CLEAR t1. t1 = NEW cls( ). result = t1.`, map[string]int{"clear_before_write": 1, "temp_assign": 1, "return_copy": 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := parseMethods("METHOD m. " + c.body + " ENDMETHOD.")[0]
			m.Patterns = map[string][2]int{}
			detect(m)
			got := map[string]int{}
			for k, v := range m.Patterns {
				if v[0]+v[1] > 0 {
					got[k] = v[0] + v[1]
				}
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
func TestNormalizeDoesNotInventLiteralUses(t *testing.T) {
	if uses(`text = 't1''t1'.`, "t1") != 0 {
		t.Fatal("literal counted as use")
	}
	a := normalize(`DATA(t1) = CAST z_foo_ab123( t2 ).`)
	b := normalize(`DATA(t45) = CAST z_bar_cd456( t67 ).`)
	if a != b {
		t.Fatalf("%s != %s", a, b)
	}
}

func TestTemplateExpressionUses(t *testing.T) {
	if uses(`text = |literal t1 { t1 } { 't1' }|.`, "t1") != 1 {
		t.Fatal("template expression not counted correctly")
	}
}
