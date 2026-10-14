package abap

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestOptionalParseFloatNodeOracle(t *testing.T) {
	opt, graph := hir.T(hir.Optional, hir.T(hir.Number)), hir.T(hir.Dynamic)
	parse := method("parse", opt, ret(rt("string.parseFloatNumber", local("text", str), hir.T(hir.Number))))
	parse.Static, parse.Params = true, []hir.Param{{Name: "text", Type: str}}
	box := method("box", graph, ret(rt("dynamic.of", local("value", opt), graph)))
	box.Static, box.Params = true, []hir.Param{{Name: "value", Type: opt}}
	truth := method("truth", boolean, ret(&hir.Expr{Kind: hir.ToBoolean, Type: boolean, X: local("value", opt)}))
	truth.Static, truth.Params = true, box.Params
	write := method("write", str, ret(rt("json.stringifySubset", local("value", opt), str)))
	write.Static, write.Params = true, box.Params
	equal := method("equal", boolean, ret(&hir.Expr{Kind: hir.Binary, Op: "==", Type: boolean, X: local("a", opt), Y: local("b", opt)}))
	equal.Static, equal.Params = true, []hir.Param{{Name: "a", Type: opt}, {Name: "b", Type: opt}}
	finite := method("finite", hir.T(hir.Number), ret(&hir.Expr{Kind: hir.Narrow, Type: hir.T(hir.Number), X: local("value", opt)}))
	finite.Static, finite.Params = true, box.Params
	p := &hir.Program{Classes: []*hir.Class{{Name: "OptionalFloatProbe", Methods: []*hir.Method{parse, box, truth, write, equal, finite}}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("OptionalFloatProbe")
	if !strings.Contains(files[owner+".clas.abap"], "=>float_optional(") {
		t.Fatal("optional store must not first evaluate the finite-only operation")
	}
	inputs := []string{"", "NaN", "abc", "Infinity!", "+Infinity", "-Infinity", "1e400", "-1e400", " 10.0tail", "1e+", ".5suffix", "0xFF", "1_000", "-0", "1e-400", "\ufeff\u2007-.125e2x"}
	raw, _ := json.Marshal(inputs)
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(s=>{const n=parseFloat(s);return {text:String(n),truth:!!n,json:JSON.stringify(n)}})))`, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	var wants []struct {
		Text, JSON string
		Truth      bool
	}
	if err := json.Unmarshal(out, &wants); err != nil {
		t.Fatal(err)
	}
	var unit strings.Builder
	fmt.Fprintf(&unit, "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA value TYPE REF TO %s.\nDATA boxed TYPE REF TO %s.\n", names.Get("runtime."+opt.String()), names.Get("runtime.dynamic"))
	unit.WriteString("DATA native TYPE f.\nDATA refused TYPE abap_bool.\n")
	for i, input := range inputs {
		want := wants[i]
		truth := "abap_false"
		if want.Truth {
			truth = "abap_true"
		}
		fmt.Fprintf(&unit, "value = %s=>%s( %s ).\ncl_abap_unit_assert=>assert_true( value->has ).\nboxed = %s=>%s( value ).\n", owner, names.Get("member.parse"), regexABAPString(input), owner, names.Get("member.box"))
		// Fractional dynamic.to_string retains its legacy safe-integer
		// boundary; JSON and truth still observe these values exactly.
		if !strings.Contains(want.Text, ".") {
			fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = boxed->to_string( ) exp = %s ).\n", regexABAPString(want.Text))
		}
		fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = %s=>%s( value ) exp = %s ).\ncl_abap_unit_assert=>assert_equals( act = %s=>%s( value ) exp = %s ).\n", owner, names.Get("member.truth"), truth, owner, names.Get("member.write"), regexABAPString(want.JSON))
		selfEqual := "abap_true"
		if want.Text == "NaN" {
			selfEqual = "abap_false"
		}
		fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = %s=>%s( %s = value %s = value ) exp = %s ).\n", owner, names.Get("member.equal"), names.Get("param.a"), names.Get("param.b"), selfEqual)
		if want.JSON == "null" {
			fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nnative = %s=>%s( value ).\nCATCH cx_sy_range_out_of_bounds.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member.finite"))
		}
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}
