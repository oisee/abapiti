package abap

import "strings"

// These are host diagnostics, not JavaScript own properties. Keeping the CX
// reference preserves identity and the complete previous chain across JS
// catch/rethrow, including after the JS message is changed by user code.
func (e *emitter) abapErrorDiagnostics() (string, string) {
	return `DATA abap_origin TYPE REF TO cx_root.
DATA abap_class TYPE string.
DATA abap_text TYPE string.
DATA abap_previous TYPE REF TO cx_root.
DATA abap_program TYPE syrepid.
DATA abap_include TYPE syrepid.
DATA abap_line TYPE i.
DATA abap_position_status TYPE string.
METHODS capture_abap IMPORTING origin TYPE REF TO cx_root.
METHODS capture_abap_position.
`, strings.ReplaceAll(`METHOD capture_abap.
abap_origin = origin.
abap_class = cl_abap_classdescr=>get_class_name( origin ).
REPLACE FIRST OCCURRENCE OF '\CLASS=' IN abap_class WITH ''.
abap_text = origin->get_text( ).
abap_previous = origin->previous.
capture_abap_position( ).
$MESSAGE = abap_class && |: | && abap_text.
IF abap_program IS NOT INITIAL OR abap_include IS NOT INITIAL OR abap_line > 0.
$MESSAGE = $MESSAGE && | @ { abap_program }/{ abap_include }:{ abap_line }|.
ENDIF.
ENDMETHOD.
METHOD capture_abap_position.
CLEAR: abap_program, abap_include, abap_line.
abap_position_status = 'unavailable'.
TRY.
abap_origin->get_source_position( IMPORTING program_name = abap_program include_name = abap_include source_line = abap_line ).
IF abap_program IS NOT INITIAL OR abap_include IS NOT INITIAL OR abap_line > 0.
abap_position_status = 'available'.
ENDIF.
CATCH cx_root INTO DATA(position_error).
CLEAR: abap_program, abap_include, abap_line.
abap_position_status = 'unavailable: ' && position_error->get_text( ).
ENDTRY.
ENDMETHOD.
`, "$MESSAGE", e.member("message"))
}
