package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// parseWithErrors follows the Go parser's deliberately narrow fast-xml-parser
// error boundary. Coverage faults stay distinct from translated Error payloads.
// The strict parseSubset class and its default emitted bytes are unchanged.
func (e *emitter) xmlWithErrorsRuntime() string {
	id := "runtime.xmlWithErrors"
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	e.xmlSubsetRuntime()
	strict := e.name("runtime.xmlSubset")
	refusal := e.name("exception.RegistryXMLSubsetError")
	if e.classBy("builtin.Error") == nil {
		e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\nCLASS-METHODS parse IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE " + e.typ(hir.T(hir.Dynamic)) + ".\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nMETHOD parse.\nRAISE EXCEPTION TYPE " + refusal + ".\nENDMETHOD.\nENDCLASS.\n"
		return name
	}
	errorType := hir.Ref("builtin.Error")
	errorABAP := e.typ(errorType)
	exception := e.exception(errorType)
	optional := e.typ(hir.T(hir.Optional, hir.T(hir.String)))
	code := strings.ReplaceAll(e.files[strict+".clas.abap"], strict, name)
	code = strings.Replace(code, "DATA count TYPE i.\ncursor = cursor + 1.", `DATA count TYPE i.
DATA position TYPE i.
DATA lo TYPE i.
DATA hi TYPE i.
DATA context TYPE string.
DATA message TYPE string.
DATA native TYPE `+errorABAP+`.
DATA opt TYPE `+optional+`.
DATA fault TYPE REF TO `+exception+`.
cursor = cursor + 1.`, 1)
	syntax := `IF start > 0.
IF start = strlen( input ).
ch = 'a'.
ELSE.
ch = input+start(1).
ENDIF.
IF ch CO 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_'.
position = start - 1.
lo = nmax( val1 = 0 val2 = position - 50 ).
hi = nmin( val1 = strlen( input ) val2 = position + 50 ).
count = hi - lo.
context = input+lo(count).
message = |readTagExp returned undefined at position { position }. Context: "|.
CONCATENATE message context '"' INTO message RESPECTING BLANKS.
CREATE OBJECT opt.
opt->has = abap_true.
opt->value = message.
CREATE OBJECT native EXPORTING ` + e.param("message") + ` = opt.
CREATE OBJECT fault.
fault->payload = native.
`
	if e.promiseUsed {
		e.dynamicRuntime()
		syntax += "CREATE OBJECT fault->source_value.\nfault->source_value->tag = " + e.name("runtime.dynamic") + "=>tag_ref.\nfault->source_value->oval = native.\n"
	}
	syntax += "RAISE EXCEPTION fault.\nENDIF.\nENDIF.\n"
	code = strings.Replace(code, "ENDWHILE.\nRAISE EXCEPTION TYPE "+refusal+".\nENDMETHOD.\nMETHOD decode.", "ENDWHILE.\n"+syntax+"RAISE EXCEPTION TYPE "+refusal+".\nENDMETHOD.\nMETHOD decode.", 1)
	// The external parser accepts mismatched closing names and unfinished nodes;
	// EOF text is not committed. These quirks are also in the unchanged Go oracle.
	code = strings.Replace(code, "count = cursor - start.\npart = input+start(count).", "IF cursor = strlen( input ).\nclosed = abap_true.\nEXIT.\nENDIF.\ncount = cursor - start.\npart = input+start(count).", 1)
	code = strings.Replace(code, "IF closing IS INITIAL OR tag_name <> closing.", "IF closing IS INITIAL.", 1)
	code = strings.Replace(code, "IF closing IS NOT INITIAL AND closed = abap_false.\nRAISE EXCEPTION TYPE "+refusal+".\nENDIF.\n", "", 1)
	e.files[name+".clas.abap"] = code
	return name
}
