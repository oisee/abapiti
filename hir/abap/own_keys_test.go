package abap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestOwnKeysOrderAndRefusals(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	keys := method("keys", str, ret(rt("array.join", rt("dynamic.keys", local("input", dyn), hir.T(hir.Array, str)), str, hir.L(str, ","))))
	keys.Static = true
	keys.Params = []hir.Param{{Name: "input", Type: dyn}}
	put := method("put", hir.T(hir.Void), run(rt("dynamic.put", local("input", dyn), hir.T(hir.Void), hir.L(str, "extra"), rt("dynamic.of", hir.L(boolean, true), dyn))))
	put.Static = true
	put.Params = keys.Params
	makeClass := func(name string, order []string) *hir.Class {
		c := &hir.Class{Name: name}
		body := hir.B()
		for _, key := range []string{"10", "2", "01", "4294967295", "a"} {
			c.Fields = append(c.Fields, hir.Field{Name: key, Type: boolean})
		}
		for _, key := range order {
			body.List = append(body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}, Name: key, Type: boolean}, Y: hir.L(boolean, true)})
		}
		c.Ctor = method("constructor", hir.T(hir.Void), body)
		return c
	}
	good := makeClass("GoodKeys", []string{"10", "2", "01", "4294967295", "a"})
	bad := makeClass("BadKeys", []string{"2", "10", "01", "4294967295", "a"})
	unknown := &hir.Class{Name: "UnknownKeys", Structural: true, Fields: []hir.Field{{Name: "value", Type: boolean}, {Name: "maybe", Type: hir.T(hir.Optional, boolean)}}}
	unsafe := &hir.Class{Name: "BehaviorKeys", Super: "GoodKeys", Fields: []hir.Field{{Name: "extra", Type: boolean}}}
	unsafe.Methods = []*hir.Method{method("mutate", boolean, hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("BehaviorKeys")}, Name: "extra", Type: boolean}, Y: hir.L(boolean, true)}, ret(hir.L(boolean, true))))}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{good, bad, unknown, unsafe, {Name: "MissingKeys", Fields: []hir.Field{{Name: "flag", Type: boolean}}, Ctor: method("constructor", hir.T(hir.Void), hir.B())}, {Name: "KeyProbe", Methods: []*hir.Method{keys, put}}}})
	if err != nil {
		t.Fatal(err)
	}
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_keys DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_keys IMPLEMENTATION.\nMETHOD check.\n")
	fmt.Fprintf(&unit, "DATA input TYPE REF TO %s.\nDATA raw TYPE REF TO object.\nDATA actual TYPE string.\nDATA refused TYPE abap_bool.\nCREATE OBJECT input.\ninput->tag = %s=>tag_ref.\nCREATE OBJECT raw TYPE %s.\ninput->oval = raw.\nactual = %s=>%s( input ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = '2,10,01,4294967295,a' ).\n", names.Get("runtime.dynamic"), names.Get("runtime.dynamic"), names.Get("GoodKeys"), names.Get("KeyProbe"), names.Get("member.keys"))
	fmt.Fprintf(&unit, "TRY.\nDATA prototype TYPE REF TO %s.\nprototype = input->get( 'toString' ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nCLEAR refused.\n", names.Get("runtime.dynamic"), names.Get("view.refusal"))
	fmt.Fprintf(&unit, "TRY.\n%s=>%s( input ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", names.Get("KeyProbe"), names.Get("member.put"), names.Get("view.refusal"))
	for _, name := range []string{"BadKeys", "BehaviorKeys", "UnknownKeys", "MissingKeys"} {
		fmt.Fprintf(&unit, "CLEAR refused.\nCREATE OBJECT raw TYPE %s.\ninput->oval = raw.\nTRY.\nactual = %s=>%s( input ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", names.Get(name), names.Get("KeyProbe"), names.Get("member.keys"), names.Get("view.refusal"))
		if name == "UnknownKeys" {
			fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nprototype = input->get( 'value' ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", names.Get("view.refusal"))
		}

	}
	fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nprototype = input->get( 'flag' ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", names.Get("view.refusal"))
	for _, tag := range []string{"tag_array", "tag_number", "tag_null"} {
		fmt.Fprintf(&unit, "CLEAR refused.\ninput->tag = %s=>%s.\nTRY.\nactual = %s=>%s( input ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", names.Get("runtime.dynamic"), tag, names.Get("KeyProbe"), names.Get("member.keys"), names.Get("view.refusal"))
	}
	// Install a known per-instance presence graph, then exercise the original
	// structural object's computed writes, extras and present-undefined slot.
	fmt.Fprintf(&unit, `DATA shape TYPE REF TO %s.
DATA metadata TYPE REF TO %s.
DATA value TYPE REF TO %s.
CREATE OBJECT shape.
CREATE OBJECT metadata.
metadata->tag = %s=>tag_object.
metadata->put( p0 = 'value' p1 = VALUE #( ) ).
shape->%s = metadata.
input->tag = %s=>tag_ref.
input->oval = shape.
actual = %s=>%s( input ).
cl_abap_unit_assert=>assert_equals( act = actual exp = 'value' ).
value = input->get( 'maybe' ).
cl_abap_unit_assert=>assert_true( xsdbool( value IS NOT BOUND ) ).
CREATE OBJECT value.
value->tag = %s=>tag_boolean.
value->bval = abap_true.
input->put( p0 = 'maybe' p1 = value ).
value = input->get( 'maybe' ).
cl_abap_unit_assert=>assert_true( xsdbool( value IS BOUND ) ).
cl_abap_unit_assert=>assert_true( value->bval ).
input->put( p0 = 'maybe' p1 = VALUE #( ) ).
value = input->get( 'maybe' ).
cl_abap_unit_assert=>assert_true( xsdbool( value IS NOT BOUND ) ).
%s=>%s( input ).
actual = %s=>%s( input ).
cl_abap_unit_assert=>assert_equals( act = actual exp = 'value,maybe,extra' ).
value = input->get( 'extra' ).
cl_abap_unit_assert=>assert_true( value->bval ).
`, names.Get("UnknownKeys"), names.Get("runtime.dynamic"), names.Get("runtime.dynamic"), names.Get("runtime.dynamic"), names.Get("own.keys"), names.Get("runtime.dynamic"), names.Get("KeyProbe"), names.Get("member.keys"), names.Get("runtime.dynamic"), names.Get("KeyProbe"), names.Get("member.put"), names.Get("KeyProbe"), names.Get("member.keys"))
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[names.Get("KeyProbe")+".clas.testclasses.abap"] = unit.String()
	if out := fixtureOutputDir(t); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			source, err = Format(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
