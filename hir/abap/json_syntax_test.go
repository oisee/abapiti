package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestJSONSyntaxErrorsUseSourceCatchABI(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(fmt.Sprint(async), func(t *testing.T) {
			d := hir.T(hir.Dynamic)
			catchType := hir.Ref("builtin.Error")
			caught := local("error", catchType)
			if async {
				catchType = d
				caught = rt("dynamic.asRef", local("error", d), hir.Ref(hir.RootObject))
			}
			attempt := &hir.Stmt{Kind: hir.Try, Name: "error", Type: catchType,
				Body: hir.B(run(rt("json.parseSubset", local("text", str), d)), ret(hir.L(boolean, false))),
				Else: hir.B(ret(&hir.Expr{Kind: hir.InstanceOf, Type: boolean, Owner: "builtin.SyntaxError", X: caught}))}
			m := method("caught", boolean, hir.B(attempt))
			m.Static, m.Params = true, []hir.Param{{Name: "text", Type: str}}
			methods := []*hir.Method{m}
			if async {
				settled := hir.T(hir.Promise, hir.T(hir.Void))
				p := method("settled", settled, ret(&hir.Expr{Kind: hir.PromiseFulfill, Type: settled}))
				p.Static = true
				methods = append(methods, p)
			}
			files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{
				{Name: "JSONCatchProbe", Methods: methods},
				{Name: "builtin.Error", Fields: []hir.Field{{Name: "message", Type: str}}},
				{Name: "builtin.SyntaxError", Super: "builtin.Error"},
			}})
			if err != nil {
				t.Fatal(err)
			}
			owner := names.Get("JSONCatchProbe")
			text := files[owner+".clas.abap"]
			if !strings.Contains(text, "CATCH "+names.Get("exception.JSONSyntaxError")) || strings.Contains(text, "CATCH "+names.Get("exception.RegistryJSONSubsetError")) {
				t.Fatal("only syntax errors may cross the source catch boundary")
			}
			var unit strings.Builder
			unit.WriteString("CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA caught TYPE abap_bool.\nDATA refused TYPE abap_bool.\n")
			for _, input := range []string{"sdfsdf", "", "{", "[1,]", `{"x":}`, `"bad\q"`, "01", "true false"} {
				fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_true( %s=>%s( %s ) ).\n", owner, names.Get("member.caught"), regexABAPString(input))
			}
			fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_false( %s=>%s( `null` ) ).\nTRY.\ncaught = %s=>%s( `1e9999` ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.caught"), owner, names.Get("member.caught"), names.Get("exception.RegistryJSONSubsetError"))
			files[owner+".clas.testclasses.abap"] = unit.String()
			storageHostFixture(t, files, owner)
		})
	}
}
