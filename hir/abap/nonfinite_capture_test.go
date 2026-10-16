package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestNonfiniteParserCaptureStorage(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "widened"
		if existing {
			name = "existing optional"
		}
		t.Run(name, func(t *testing.T) { nonfiniteParserCaptureFixture(t, existing) })
	}
}

func nonfiniteParserCaptureFixture(t *testing.T, existing bool) {
	num, row := hir.T(hir.Number), hir.Ref("Row")
	storage := num
	if existing {
		storage = hir.T(hir.Optional, num)
	}
	ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign,
		X: &hir.Expr{Kind: hir.FieldGet, Type: storage, X: &hir.Expr{Kind: hir.This, Type: row}, Name: "value"}, Y: local("value", storage)}))
	ctor.Params = []hir.Param{{Name: "value", Type: storage}}
	makeRow := func(value *hir.Expr) *hir.Expr {
		if existing {
			value = &hir.Expr{Kind: hir.Conditional, Type: storage,
				X: hir.L(hir.T(hir.Bool), true), Y: value,
				Z: &hir.Expr{Kind: hir.Lit, Type: storage}}
		}
		return &hir.Expr{Kind: hir.New, Type: row, Args: []*hir.Expr{value}}
	}
	seed := method("missing", row, ret(makeRow(rt("string.parseFloatNumber", hir.L(str, "NaN"), num))))
	seed.Static = true
	methods := []*hir.Method{}
	if !existing {
		methods = append(methods, seed)
	}
	var originalCapture *hir.Stmt
	for _, parser := range []string{"Int", "Float"} {
		// Frontend coercions can put a parser behind a sequence before the
		// evaluation-order temporary is introduced.
		parsed := &hir.Expr{Kind: hir.Seq, Type: num,
			Stmt: hir.B(decl("input", str, local("text", str))),
			Y:    rt("string.parse"+parser+"Number", local("input", str), num)}
		m := method("parse"+parser, row, hir.B(
			decl("parsed", num, parsed),
			decl("captured", num, local("parsed", num)), ret(makeRow(local("captured", num)))))
		originalCapture = m.Body.List[0]
		m.Static = true
		m.Params = []hir.Param{{Name: "text", Type: str}}
		methods = append(methods, m)
	}
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "Row", Fields: []hir.Field{{Name: "value", Type: storage}}, Ctor: ctor},
		{Name: "Probe", Methods: methods},
	}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	if !ctor.Params[0].Type.Equal(storage) || !originalCapture.Type.Equal(num) {
		t.Fatal("ABAP storage lowering mutated source HIR")
	}
	owner := names.Get("Probe")
	text := files[owner+".clas.abap"]
	for _, parser := range []string{"int_optional", "float_optional"} {
		if !strings.Contains(text, "=>"+parser+"(") {
			t.Errorf("captured parser lost its tagged destination: %s", parser)
		}
	}
	if strings.Contains(text, "range_parse_safe_prefix") {
		t.Error("captured parseInt still executes the finite-only guard")
	}
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\n")
	fmt.Fprintf(&unit, "DATA row TYPE REF TO %s.\n", names.Get("Row"))
	for _, parser := range []string{"Int", "Float"} {
		for _, input := range []string{"undefined", "42tail"} {
			want := "0"
			if input == "undefined" {
				want = "1"
			}
			fmt.Fprintf(&unit, "row = %s=>%s( `%s` ).\ncl_abap_unit_assert=>assert_true( row->%s->has ).\ncl_abap_unit_assert=>assert_equals( act = row->%s->number_special exp = %s ).\n", owner, names.Get("member.parse"+parser), input, names.Get("member.value"), names.Get("member.value"), want)
			if input == "42tail" {
				fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = row->%s->value exp = 42 ).\n", names.Get("member.value"))
			}
		}
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}

// Sibling blocks and evaluation-order sequences may reuse a source name.
// Each captured read must follow its own declaration's storage, while an
// unrelated outer local with that name remains a finite scalar.
func TestNonfiniteParserCaptureLexicalStorage(t *testing.T) {
	num, opt, row := hir.T(hir.Number), hir.T(hir.Optional, hir.T(hir.Number)), hir.Ref("Row")
	ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign,
		X: &hir.Expr{Kind: hir.FieldGet, Type: opt, X: &hir.Expr{Kind: hir.This, Type: row}, Name: "value"}, Y: local("value", opt)}))
	ctor.Params = []hir.Param{{Name: "value", Type: opt}}
	makeRow := func(value *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.New, Type: row, Args: []*hir.Expr{value}}
	}
	body := []*hir.Stmt{decl("column", num, hir.L(num, 99))}
	for _, value := range []int{1, 2} {
		body = append(body, hir.B(decl("column", num, hir.L(num, value)),
			decl("row", row, makeRow(local("column", num)))))
	}
	body = append(body, decl("row", row, makeRow(&hir.Expr{Kind: hir.Seq, Type: num,
		Stmt: hir.B(decl("captured", num, rt("string.parseIntNumber", hir.L(str, "42tail"), num))),
		Y:    local("captured", num)})), ret(local("column", num)))
	check := method("check", num, hir.B(body...))
	check.Static = true
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "Row", Fields: []hir.Field{{Name: "value", Type: opt}}, Ctor: ctor},
		{Name: "Probe", Methods: []*hir.Method{check}},
	}}
	q, _, _ := retainNonfiniteStorage(p)
	lowered := q.Classes[1].Methods[0].Body.List
	for i := 1; i <= 2; i++ {
		block := lowered[i].List
		if !block[0].Type.Equal(opt) || !block[1].X.Args[0].Type.Equal(opt) {
			t.Fatalf("block %d: capture declaration %s and read %s must both use %s", i, block[0].Type, block[1].X.Args[0].Type, opt)
		}
	}
	seq := lowered[3].X.Args[0]
	if !seq.Stmt.List[0].Type.Equal(opt) || !seq.Y.Type.Equal(opt) {
		t.Fatal("sequence result lost its capture's tagged storage")
	}
	if !lowered[0].Type.Equal(num) || !lowered[4].X.Type.Equal(num) || !q.Classes[1].Methods[0].Result.Equal(num) {
		t.Fatal("shadowed captures widened the unrelated outer local")
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("Probe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_equals( act = %s=>%s( ) exp = 99 ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.check"))
	storageHostFixture(t, files, owner)
}
