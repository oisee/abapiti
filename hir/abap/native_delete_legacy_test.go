package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) legacyNativeDeletePart(testPart bool) string {
	name := e.name("runtime.legacyNativeDelete")
	if testPart {
		name = "zcl_legacy_test_delete"
	}
	if e.types[name] {
		return name
	}
	e.types[name] = true
	var code strings.Builder
	fmt.Fprintf(&code, "CLASS %s DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\nCLASS-METHODS run IMPORTING p0 TYPE REF TO object key TYPE string RETURNING VALUE(result) TYPE abap_bool.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS %s IMPLEMENTATION.\nMETHOD run.\nIF p0 IS NOT BOUND.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `record_storage.go: nativeDelete: receiver is undefined`.\nENDIF.\n", name, name, e.name("view.refusal"))
	if e.testPackages && !testPart {
		factory := e.name("runtime.classvalue.factory")
		fmt.Fprintf(&code, "IF %s=>extension IS BOUND.\nDATA handled TYPE abap_bool.\n%s=>extension->delete( EXPORTING p0 = p0 key = key IMPORTING handled = handled result = result ).\nIF handled = abap_true.\nRETURN.\nENDIF.\nENDIF.\n", factory, factory)
	}
	for i, c := range e.classesByDepth() {
		if e.testPackages && testDeclaration(c) != testPart {
			continue
		}
		fmt.Fprintf(&code, "IF p0 IS INSTANCE OF %s.\nDATA native%d TYPE REF TO %s.\nnative%d ?= p0.\n", e.name(c.Name), i, e.name(c.Name), i)
		seen := map[string]bool{}
		caseOpened := false
		for base := c; base != nil; base = e.classBy(base.Super) {
			for _, f := range base.Fields {
				if f.Static || seen[f.Name] {
					continue
				}
				seen[f.Name] = true
				if !caseOpened {
					code.WriteString("CASE key.\n")
					caseOpened = true
				}
				fmt.Fprintf(&code, "WHEN `%s`.\n", strings.ReplaceAll(f.Name, "`", "``"))
				typ := e.semanticFieldType(base, f)
				if prototypeProperty(f.Name) || typ.Kind != hir.Optional && typ.Kind != hir.Dynamic {
					fmt.Fprintf(&code, "RAISE EXCEPTION TYPE %s EXPORTING reason = `record_storage.go: nativeDelete: required or prototype slot cannot retain absence`.\n", e.name("view.refusal"))
				} else {
					fmt.Fprintf(&code, "CLEAR native%d->%s.\nINSERT key INTO TABLE native%d->%s.\n", i, e.member(f.Name), i, e.name("record.deleted"))
				}
			}
		}
		if caseOpened {
			code.WriteString("ENDCASE.\n")
		}
		code.WriteString("result = abap_true.\nRETURN.\nENDIF.\n")
	}
	if testPart {
		text := code.String()
		text = strings.Replace(text, " RETURNING VALUE(result) TYPE abap_bool.", " EXPORTING handled TYPE abap_bool result TYPE abap_bool.", 1)
		text = strings.ReplaceAll(text, "result = abap_true.\nRETURN.", "handled = abap_true.\nresult = abap_true.\nRETURN.")
		code.Reset()
		code.WriteString(text)
		code.WriteString("ENDMETHOD.\nENDCLASS.\n")
	} else {
		fmt.Fprintf(&code, "RAISE EXCEPTION TYPE %s EXPORTING reason = `record_storage.go: nativeDelete: native class layout unknown`.\nENDMETHOD.\nENDCLASS.\n", e.name("view.refusal"))
	}
	e.files[name+".clas.abap"] = code.String()
	return name
}
