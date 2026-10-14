package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Structural shapes/unions can be shared even when first discovered at a test
// site. Executable test declarations and test lambdas must never enter core.
func testDeclaration(c *hir.Class) bool {
	source := relSource(c.Source)
	return strings.HasPrefix(c.Name, "test/") || strings.HasPrefix(c.Name, "runtime/mocha.ts") || strings.HasPrefix(c.Name, "runtime/chai.ts") ||
		strings.HasPrefix(c.Name, "closure.") && (strings.HasPrefix(source, "test/") || strings.HasPrefix(source, "runtime/"))
}

func (e *emitter) testPackageExtension() {
	cv := e.name("runtime.classvalue")
	args := e.typ(hir.T(hir.Array, hir.T(hir.Dynamic)))
	deletion := e.nativeDeletePart(true)
	e.files["zif_lnt_runtime_extension.intf.abap"] = "INTERFACE zif_lnt_runtime_extension PUBLIC.\nMETHODS new IMPORTING p0 TYPE REF TO " + cv + " RETURNING VALUE(result) TYPE REF TO object.\nMETHODS new_args IMPORTING p0 TYPE REF TO " + cv + " p1 TYPE " + args + " RETURNING VALUE(result) TYPE REF TO object.\nMETHODS delete IMPORTING p0 TYPE REF TO object key TYPE string EXPORTING handled TYPE abap_bool result TYPE abap_bool.\nENDINTERFACE.\n"
	e.files["zcl_lnt_test_extension.clas.abap"] = "CLASS zcl_lnt_test_extension DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\nINTERFACES zif_lnt_runtime_extension.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS zcl_lnt_test_extension IMPLEMENTATION.\nMETHOD zif_lnt_runtime_extension~new.\nresult = zcl_lnt_test_factory=>new( p0 ).\nENDMETHOD.\nMETHOD zif_lnt_runtime_extension~new_args.\nresult = zcl_lnt_test_factory=>new_args( p0 = p0 p1 = p1 ).\nENDMETHOD.\nMETHOD zif_lnt_runtime_extension~delete.\n" + deletion + "=>run( EXPORTING p0 = p0 key = key IMPORTING handled = handled result = result ).\nENDMETHOD.\nENDCLASS.\n"
	// Registration belongs to the runner, never to a class constructor: the
	// zabapgit client must work with only the $ZLNT package installed.
}
