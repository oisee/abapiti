package abap

import "strings"

// Explicit ICU-root order for the source-pinned ASCII key/name domains.
// No ABAP locale or database collation is allowed to select the ordering.
func (e *emitter) orderingSubsetRuntime() {
	id := "runtime.orderingSubset"
	if e.types[id] {
		return
	}
	e.types[id] = true
	name, exception := e.name(id), e.name("exception.RegistryOrderingSubsetError")
	e.files[exception+".clas.abap"] = "CLASS " + exception + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nMETHODS get_text REDEFINITION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + exception + " IMPLEMENTATION.\nMETHOD get_text.\nresult = `ordering input is outside the reviewed ASCII rule-key/object-name domains`.\nENDMETHOD.\nENDCLASS.\n"
	code := `CLASS $C DEFINITION PUBLIC CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS rule_key IMPORTING p0 TYPE string p1 TYPE string RETURNING VALUE(result) TYPE i.
CLASS-METHODS object_name IMPORTING p0 TYPE string p1 TYPE string RETURNING VALUE(result) TYPE i.
PROTECTED SECTION.
PRIVATE SECTION.
CLASS-METHODS compare IMPORTING p0 TYPE string p1 TYPE string alphabet TYPE string pattern TYPE string RETURNING VALUE(result) TYPE i.
ENDCLASS.
CLASS $C IMPLEMENTATION.
METHOD rule_key.
result = compare( p0 = p0 p1 = p1 alphabet = $Q_0123456789abcdefghijklmnopqrstuvwxyz$Q pattern = $Q[^a-z0-9_]$Q ).
ENDMETHOD.
METHOD object_name.
result = compare( p0 = p0 p1 = p1 alphabet = $Q_/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ$Q pattern = $Q[^A-Z0-9_/]$Q ).
ENDMETHOD.
METHOD compare.
DATA index TYPE i.
DATA left_rank TYPE i.
DATA right_rank TYPE i.
DATA left_char TYPE string.
DATA right_char TYPE string.
FIND FIRST OCCURRENCE OF REGEX pattern IN p0.
IF sy-subrc = 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
FIND FIRST OCCURRENCE OF REGEX pattern IN p1.
IF sy-subrc = 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
WHILE index < strlen( p0 ) AND index < strlen( p1 ).
left_char = p0+index(1).
right_char = p1+index(1).
FIND FIRST OCCURRENCE OF left_char IN alphabet MATCH OFFSET left_rank.
FIND FIRST OCCURRENCE OF right_char IN alphabet MATCH OFFSET right_rank.
IF left_rank < right_rank.
result = -1.
RETURN.
ELSEIF left_rank > right_rank.
result = 1.
RETURN.
ENDIF.
index = index + 1.
ENDWHILE.
IF strlen( p0 ) < strlen( p1 ).
result = -1.
ELSEIF strlen( p0 ) > strlen( p1 ).
result = 1.
ENDIF.
ENDMETHOD.
ENDCLASS.
`
	e.files[name+".clas.abap"] = strings.NewReplacer("$C", name, "$E", exception, "$Q", "`").Replace(code)
}
