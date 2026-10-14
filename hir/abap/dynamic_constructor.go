package abap

import (
	"fmt"
	"strings"
)

// Constructor reflection uses the nominal descriptor ABI, not a fabricated
// structural view. Record-backed objects keep their original property reader.
func (e *emitter) dynamicConstructorRuntime() {
	e.liveRuntime()
	d, cv, protocol, storage := e.name("runtime.dynamic"), e.name("runtime.classvalue"), e.name("runtime.described"), e.name("view.storage")
	helper, trap := e.name("runtime.dynamicConstructor"), e.classRefusal()
	var code strings.Builder
	fmt.Fprintf(&code, "CLASS %s DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\nCLASS-METHODS get IMPORTING p0 TYPE REF TO %s RETURNING VALUE(result) TYPE REF TO %s.\nCLASS-METHODS name IMPORTING p0 TYPE REF TO %s RETURNING VALUE(result) TYPE string.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS %s IMPLEMENTATION.\nMETHOD get.\nDATA adapter TYPE REF TO %s.\nDATA native TYPE REF TO %s.\nDATA descriptor TYPE REF TO %s.\nIF p0->oval IS NOT BOUND.\nRAISE EXCEPTION TYPE %s.\nENDIF.\nTRY.\nadapter ?= p0->oval.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nIF adapter IS BOUND.\nIF adapter->structural( ) = abap_true.\nresult = adapter->get( `constructor` ).\nRETURN.\nENDIF.\nENDIF.\nTRY.\nnative ?= p0->oval.\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE %s.\nENDTRY.\ndescriptor = native->%s( ).\n", helper, d, d, cv, helper, storage, protocol, cv, trap, trap, e.name("builtin.nativeConstructor"))
	for _, c := range e.p.Classes {
		shadow, unsafe := false, c.CoercionUnsafe
		for parent := c; parent != nil; parent = e.classBy(parent.Super) {
			unsafe = unsafe || parent.CoercionUnsafe
			for _, f := range parent.Fields {
				shadow = shadow || !f.Static && f.Name == "constructor"
			}
			for _, m := range parent.Methods {
				unsafe = unsafe || !m.Static && (m.Name == "get.constructor" || m.Name == "set.constructor")
			}
		}
		if unsafe {
			fmt.Fprintf(&code, "IF descriptor->emitted = `%s`.\nRAISE EXCEPTION TYPE %s.\nENDIF.\n", e.name(c.Name), trap)
		} else if shadow {
			fmt.Fprintf(&code, "IF descriptor->emitted = `%s`.\nIF adapter IS NOT BOUND.\nRAISE EXCEPTION TYPE %s.\nENDIF.\nresult = adapter->get( `constructor` ).\nRETURN.\nENDIF.\n", e.name(c.Name), trap)
		}
	}
	fmt.Fprintf(&code, "CREATE OBJECT result.\nresult->tag = %s=>tag_class.\nresult->cval = descriptor.\nENDMETHOD.\nMETHOD name.\nIF p0 IS NOT BOUND.\nRAISE EXCEPTION TYPE %s.\nENDIF.\nIF p0->has( `name` ) = abap_true OR p0->has( `get.name` ) = abap_true.\nRAISE EXCEPTION TYPE %s.\nENDIF.\n", d, trap, trap)
	for _, c := range e.p.Classes {
		if c.CoercionUnsafe {
			fmt.Fprintf(&code, "IF p0->emitted = `%s`.\nRAISE EXCEPTION TYPE %s.\nENDIF.\n", e.name(c.Name), trap)
		}
	}
	code.WriteString("result = p0->name.\nENDMETHOD.\nENDCLASS.\n")
	e.files[helper+".clas.abap"] = code.String()
	file := d + ".clas.abap"
	prefix := fmt.Sprintf("METHOD get.\nIF tag = tag_ref AND p0 = `constructor`.\nresult = %s=>get( me ).\nRETURN.\nENDIF.\nIF tag = tag_class AND p0 = `name`.\nCREATE OBJECT result.\nresult->tag = tag_string.\nresult->sval = %s=>name( cval ).\nRETURN.\nENDIF.\n", helper, helper)
	e.files[file] = strings.Replace(e.files[file], "METHOD get.\n", prefix, 1)
}
