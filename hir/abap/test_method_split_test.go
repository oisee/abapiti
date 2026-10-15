package abap

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestGeneratedDispatchSplitEquivalence(t *testing.T) {
	for _, method := range []string{"run", "entries", "invoke"} {
		var branches strings.Builder
		for i := 0; i < 700; i++ {
			fmt.Fprintf(&branches, "IF p0 IS INSTANCE OF zcl_%d.\nIF key = `a.b`.\nresult = abap_true.\nENDIF.\nRETURN.\nENDIF.\n", i)
		}
		old := "CLASS zcl_a DEFINITION.\nPUBLIC SECTION.\nENDCLASS.\nCLASS zcl_a IMPLEMENTATION.\nMETHOD " + method + ".\n" + branches.String() + "RAISE EXCEPTION TYPE cx_a.\nENDMETHOD.\nENDCLASS.\n"
		split := splitTestDispatch(old, method, "IF p0 IS INSTANCE OF ", "p0 TYPE REF TO object key TYPE string", " result TYPE abap_bool")
		var flat strings.Builder
		for _, m := range regexp.MustCompile(`(?ms)^METHOD `+method+`_part_\d+\.\n(.*?)^ENDMETHOD\.`).FindAllStringSubmatch(split, -1) {
			body := strings.TrimPrefix(m[1], "CLEAR handled.\n")
			body = strings.ReplaceAll(body, "handled = abap_true.\n", "")
			flat.WriteString(body)
			if strings.Count(m[1], "\n") > 250 {
				t.Fatal("unbounded dispatch helper")
			}
		}
		if flat.String() != branches.String() {
			t.Fatalf("%s: first-match branch order/control flow changed", method)
		}
		if !strings.Contains(split, "IMPORTING handled = matched") || !strings.Contains(split, "IF matched = abap_true.") {
			t.Fatal("lost early return propagation")
		}
	}
}

func TestRegistrationInitializerFrame(t *testing.T) {
	c := &hir.Class{Name: "test/large.ts.module"}
	m := &hir.Method{Name: "test.initialize.1", Static: true, Result: hir.T(hir.Void)}
	var stmts []*hir.Stmt
	// A single local remains live across thousands of sequential operations.
	stmts = append(stmts, &hir.Stmt{Kind: hir.VarDecl, Name: "value", Type: hir.T(hir.I32), X: hir.L(hir.T(hir.I32), 0)})
	for i := 0; i < 1800; i++ {
		stmts = append(stmts, &hir.Stmt{Kind: hir.Assign, X: hir.V("value", hir.T(hir.I32)), Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: hir.T(hir.I32), X: hir.V("value", hir.T(hir.I32)), Y: hir.L(hir.T(hir.I32), 1)}})
	}
	m.Body = hir.B(stmts...)
	c.Methods = []*hir.Method{m}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{c}})
	if err != nil {
		t.Fatal(err)
	}
	source := files[names.Get(c.Name)+".clas.abap"]
	state := names.Get(c.Name + "." + m.Name + ".state")
	if strings.Count(source, "CLEAR "+state+".") != 1 {
		t.Fatal("frame must reset once at action entry")
	}
	if strings.Contains(source, "DATA("+state+"-") || strings.Contains(source, "DATA "+state+"-") {
		t.Fatal("frame field emitted as a local declaration")
	}
	for _, m := range regexp.MustCompile(`(?ms)^\s*METHOD (\w+)\.\n(.*?)^\s*ENDMETHOD\.`).FindAllStringSubmatch(source, -1) {
		if strings.Count(m[2], "\n") > 5000 {
			t.Fatal("unbounded initializer", m[1])
		}
	}
}
