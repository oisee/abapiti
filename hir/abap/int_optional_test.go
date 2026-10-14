package abap

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestOptionalParseIntNodeOracle(t *testing.T) {
	opt := hir.T(hir.Optional, hir.T(hir.Number))
	parse := method("parse", opt, ret(rt("string.parseIntNumber", local("text", str), hir.T(hir.Number))))
	parse.Static, parse.Params = true, []hir.Param{{Name: "text", Type: str}}
	write := method("write", str, ret(rt("json.stringifySubset", local("value", opt), str)))
	write.Static, write.Params = true, []hir.Param{{Name: "value", Type: opt}}
	truth := method("truth", boolean, ret(&hir.Expr{Kind: hir.ToBoolean, Type: boolean, X: local("value", opt)}))
	truth.Static, truth.Params = true, write.Params
	p := &hir.Program{Classes: []*hir.Class{{Name: "OptionalIntProbe", Methods: []*hir.Method{parse, write, truth}}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("OptionalIntProbe")
	if !strings.Contains(files[owner+".clas.abap"], "=>int_optional(") {
		t.Fatal("optional parse must bypass the finite-only operation")
	}
	inputs := []string{"undefined", "", "NaN", "abc", "+", "-", " 42tail", "-7.9", "+3e4", "0xFF", "\ufeff\u2007+12suffix", "\u008542", "9007199254740991", "9007199254740992", "-0", strings.Repeat("9", 400), "-" + strings.Repeat("9", 400)}
	raw, _ := json.Marshal(inputs)
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(s=>{const n=parseInt(s,10);return {json:JSON.stringify(n),truth:!!n}})))`, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	var wants []struct {
		JSON  string
		Truth bool
	}
	if err := json.Unmarshal(out, &wants); err != nil {
		t.Fatal(err)
	}
	var unit strings.Builder
	fmt.Fprintf(&unit, "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA value TYPE REF TO %s.\n", names.Get("runtime."+opt.String()))
	for i, input := range inputs {
		truth := "abap_false"
		if wants[i].Truth {
			truth = "abap_true"
		}
		fmt.Fprintf(&unit, "value = %s=>%s( %s ).\ncl_abap_unit_assert=>assert_true( value->has ).\ncl_abap_unit_assert=>assert_equals( act = %s=>%s( value ) exp = %s ).\ncl_abap_unit_assert=>assert_equals( act = %s=>%s( value ) exp = %s ).\n", owner, names.Get("member.parse"), regexABAPString(input), owner, names.Get("member.write"), regexABAPString(wants[i].JSON), owner, names.Get("member.truth"), truth)
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}
