package abap

import "strings"

// jsonSubsetRuntime accepts strict JSON, retaining the recursively tagged graph.
// It deliberately does not implement JSON5 comments, names, or trailing commas.
func (e *emitter) jsonSubsetRuntime() {
	id := "runtime.jsonSubset"
	if e.types[id] {
		return
	}
	e.types[id] = true
	e.classvalueRuntime()
	e.dynamicRuntime()
	name, dynamic := e.name(id), e.name("runtime.dynamic")
	exception := e.name("exception.RegistryJSONSubsetError")
	e.files[exception+".clas.abap"] = "CLASS " + exception + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + exception + " IMPLEMENTATION.\nENDCLASS.\n"
	code := `CLASS $P DEFINITION PUBLIC CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS parse IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE REF TO $D.
PROTECTED SECTION.
PRIVATE SECTION.
DATA input TYPE string.
DATA cursor TYPE i.
METHODS whitespace.
METHODS value RETURNING VALUE(result) TYPE REF TO $D.
METHODS text RETURNING VALUE(result) TYPE string.
METHODS hex RETURNING VALUE(result) TYPE i.
ENDCLASS.
CLASS $P IMPLEMENTATION.
METHOD parse.
DATA parser TYPE REF TO $P.
CREATE OBJECT parser.
parser->input = p0.
result = parser->value( ).
parser->whitespace( ).
IF parser->cursor <> strlen( p0 ).
RAISE EXCEPTION TYPE $E.
ENDIF.
ENDMETHOD.
METHOD whitespace.
DATA ch TYPE c LENGTH 1.
DATA cr TYPE c LENGTH 1.
cr = cl_abap_conv_in_ce=>uccpi( 13 ).
WHILE cursor < strlen( input ).
ch = input+cursor(1).
IF ch <> space AND ch <> cr AND ch <> cl_abap_char_utilities=>newline AND ch <> cl_abap_char_utilities=>horizontal_tab.
EXIT.
ENDIF.
cursor = cursor + 1.
ENDWHILE.
ENDMETHOD.
METHOD hex.
DATA ch TYPE c LENGTH 1.
DATA digit TYPE i.
DATA alphabet TYPE string VALUE '0123456789abcdef'.
DO 4 TIMES.
IF cursor >= strlen( input ).
RAISE EXCEPTION TYPE $E.
ENDIF.
ch = to_lower( input+cursor(1) ).
FIND ch IN alphabet MATCH OFFSET digit.
IF sy-subrc <> 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
result = result * 16 + digit.
cursor = cursor + 1.
ENDDO.
ENDMETHOD.
METHOD text.
DATA ch TYPE string.
DATA code TYPE i.
DATA low TYPE i.
DATA pair TYPE string.
DATA bytes TYPE xstring.
DATA unit TYPE x LENGTH 2.
DATA converter TYPE REF TO cl_abap_conv_in_ce.
cursor = cursor + 1.
WHILE cursor < strlen( input ).
ch = input+cursor(1).
cursor = cursor + 1.
IF ch = '"'.
RETURN.
ENDIF.
IF ch = '\'.
IF cursor >= strlen( input ).
RAISE EXCEPTION TYPE $E.
ENDIF.
ch = input+cursor(1).
cursor = cursor + 1.
CASE ch.
WHEN '"' OR '\' OR '/'.
WHEN 'b'.
ch = cl_abap_conv_in_ce=>uccpi( 8 ).
WHEN 'f'.
ch = cl_abap_conv_in_ce=>uccpi( 12 ).
WHEN 'n'.
ch = cl_abap_char_utilities=>newline.
WHEN 'r'.
ch = cl_abap_conv_in_ce=>uccpi( 13 ).
WHEN 't'.
ch = cl_abap_char_utilities=>horizontal_tab.
WHEN 'u'.
code = hex( ).
IF code >= 55296 AND code <= 56319 AND cursor + 6 <= strlen( input ).
pair = input+cursor(2).
IF pair = '\u'.
cursor = cursor + 2.
low = hex( ).
IF low >= 56320 AND low <= 57343.
unit = code.
CONCATENATE unit+1(1) unit+0(1) INTO bytes IN BYTE MODE.
unit = low.
CONCATENATE bytes unit+1(1) unit+0(1) INTO bytes IN BYTE MODE.
converter = cl_abap_conv_in_ce=>create( encoding = '4103' ).
converter->convert( EXPORTING input = bytes IMPORTING data = ch ).
ELSE.
RAISE EXCEPTION TYPE $E.
ENDIF.
ELSE.
unit = code.
ch = cl_abap_conv_in_ce=>uccp( unit ).
ENDIF.
ELSEIF code >= 55296 AND code <= 57343.
unit = code.
ch = cl_abap_conv_in_ce=>uccp( unit ).
ELSE.
ch = cl_abap_conv_in_ce=>uccpi( code ).
ENDIF.
WHEN OTHERS.
RAISE EXCEPTION TYPE $E.
ENDCASE.
ELSE.
code = cl_abap_conv_out_ce=>uccpi( ch ).
IF code < 32.
RAISE EXCEPTION TYPE $E.
ENDIF.
ENDIF.
CONCATENATE result ch INTO result RESPECTING BLANKS.
ENDWHILE.
RAISE EXCEPTION TYPE $E.
ENDMETHOD.
METHOD value.
DATA ch TYPE string.
DATA key TYPE string.
DATA child TYPE REF TO $D.
DATA start TYPE i.
DATA count TYPE i.
DATA token TYPE string.
whitespace( ).
IF cursor >= strlen( input ).
RAISE EXCEPTION TYPE $E.
ENDIF.
ch = input+cursor(1).
CREATE OBJECT result.
CASE ch.
WHEN '"'.
result->tag = $D=>tag_string.
result->sval = text( ).
WHEN '{'.
result->tag = $D=>tag_object.
cursor = cursor + 1.
whitespace( ).
IF cursor < strlen( input ) AND input+cursor(1) = '}'.
cursor = cursor + 1.
RETURN.
ENDIF.
WHILE cursor < strlen( input ).
IF input+cursor(1) <> '"'.
RAISE EXCEPTION TYPE $E.
ENDIF.
key = text( ).
whitespace( ).
IF cursor >= strlen( input ) OR input+cursor(1) <> ':'.
RAISE EXCEPTION TYPE $E.
ENDIF.
cursor = cursor + 1.
child = value( ).
result->put( p0 = key p1 = child ).
whitespace( ).
IF cursor >= strlen( input ).
RAISE EXCEPTION TYPE $E.
ENDIF.
ch = input+cursor(1).
cursor = cursor + 1.
IF ch = '}'.
RETURN.
ELSEIF ch <> ','.
RAISE EXCEPTION TYPE $E.
ENDIF.
whitespace( ).
ENDWHILE.
RAISE EXCEPTION TYPE $E.
WHEN '['.
result->tag = $D=>tag_array.
cursor = cursor + 1.
whitespace( ).
IF cursor < strlen( input ) AND input+cursor(1) = ']'.
cursor = cursor + 1.
RETURN.
ENDIF.
WHILE cursor < strlen( input ).
child = value( ).
result->append( child ).
whitespace( ).
IF cursor >= strlen( input ).
RAISE EXCEPTION TYPE $E.
ENDIF.
ch = input+cursor(1).
cursor = cursor + 1.
IF ch = ']'.
RETURN.
ELSEIF ch <> ','.
RAISE EXCEPTION TYPE $E.
ENDIF.
whitespace( ).
ENDWHILE.
RAISE EXCEPTION TYPE $E.
WHEN OTHERS.
start = cursor.
WHILE cursor < strlen( input ).
ch = input+cursor(1).
IF ch = ',' OR ch = '}' OR ch = ']' OR ch = space OR ch = cl_abap_char_utilities=>newline OR ch = cl_abap_char_utilities=>horizontal_tab OR ch = cl_abap_conv_in_ce=>uccpi( 13 ).
EXIT.
ENDIF.
cursor = cursor + 1.
ENDWHILE.
count = cursor - start.
token = input+start(count).
CASE token.
WHEN 'null'.
result->tag = $D=>tag_null.
WHEN 'true' OR 'false'.
result->tag = $D=>tag_boolean.
result->bval = xsdbool( token = 'true' ).
WHEN OTHERS.
FIND REGEX '^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$' IN token.
IF sy-subrc <> 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
result->tag = $D=>tag_number.
TRY.
result->nval = token.
CATCH cx_sy_conversion_error cx_sy_arithmetic_error.
RAISE EXCEPTION TYPE $E.
ENDTRY.
ENDCASE.
ENDCASE.
ENDMETHOD.
ENDCLASS.
`
	e.files[name+".clas.abap"] = strings.NewReplacer("$P", name, "$D", dynamic, "$E", exception).Replace(code)
}
