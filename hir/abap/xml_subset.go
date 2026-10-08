package abap

import "strings"

// xmlSubsetRuntime implements only the reviewed abapGit XML domain. XML
// declarations, attributes, namespaced names, entities and whitespace follow
// fast-xml-parser's pinned options; unsupported constructs raise explicitly.
func (e *emitter) xmlSubsetRuntime() {
	id := "runtime.xmlSubset"
	if e.types[id] {
		return
	}
	e.types[id] = true
	e.classvalueRuntime()
	e.dynamicRuntime()
	name, dynamic := e.name(id), e.name("runtime.dynamic")
	exception := e.name("exception.RegistryXMLSubsetError")
	e.files[exception+".clas.abap"] = "CLASS " + exception + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nMETHODS get_text REDEFINITION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + exception + " IMPLEMENTATION.\nMETHOD get_text.\nresult = `input is outside the reviewed abapGit XML subset`.\nENDMETHOD.\nENDCLASS.\n"
	code := `CLASS $P DEFINITION PUBLIC CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS parse IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE REF TO $D.
PROTECTED SECTION.
PRIVATE SECTION.
DATA input TYPE string.
DATA cursor TYPE i.
METHODS read_node IMPORTING closing TYPE string RETURNING VALUE(result) TYPE REF TO $D.
METHODS header RETURNING VALUE(result) TYPE string.
METHODS decode IMPORTING text TYPE string RETURNING VALUE(result) TYPE string.
ENDCLASS.
CLASS $P IMPLEMENTATION.
METHOD parse.
DATA parser TYPE REF TO $P.
CREATE OBJECT parser.
parser->input = p0.
REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>cr_lf IN parser->input WITH cl_abap_char_utilities=>newline.
DATA cr TYPE c LENGTH 1.
cr = cl_abap_conv_in_ce=>uccpi( 13 ).
REPLACE ALL OCCURRENCES OF cr IN parser->input WITH cl_abap_char_utilities=>newline.
result = parser->read_node( '' ).
ENDMETHOD.
METHOD header.
DATA quote TYPE c LENGTH 1.
DATA ch TYPE c LENGTH 1.
DATA start TYPE i.
DATA count TYPE i.
cursor = cursor + 1.
start = cursor.
WHILE cursor < strlen( input ).
ch = input+cursor(1).
IF quote IS NOT INITIAL.
IF ch = quote.
CLEAR quote.
ENDIF.
ELSEIF ch = '"' OR ch = ''''.
quote = ch.
ELSEIF ch = '>'.
count = cursor - start.
result = input+start(count).
cursor = cursor + 1.
RETURN.
ELSEIF ch = '<'.
RAISE EXCEPTION TYPE $E.
ENDIF.
cursor = cursor + 1.
ENDWHILE.
RAISE EXCEPTION TYPE $E.
ENDMETHOD.
METHOD decode.
DATA i TYPE i.
DATA j TYPE i.
DATA count TYPE i.
DATA entity TYPE string.
DATA ch TYPE string.
WHILE i < strlen( text ).
ch = text+i(1).
IF ch <> '&'.
CONCATENATE result ch INTO result RESPECTING BLANKS.
i = i + 1.
CONTINUE.
ENDIF.
j = i + 1.
WHILE j < strlen( text ) AND text+j(1) <> ';'.
j = j + 1.
ENDWHILE.
IF j >= strlen( text ).
RAISE EXCEPTION TYPE $E.
ENDIF.
count = j - i + 1.
entity = text+i(count).
CASE entity.
WHEN '&amp;'.
ch = '&'.
WHEN '&lt;'.
ch = '<'.
WHEN '&gt;'.
ch = '>'.
WHEN '&quot;'.
ch = '"'.
WHEN '&apos;'.
ch = ''''.
WHEN OTHERS.
RAISE EXCEPTION TYPE $E.
ENDCASE.
CONCATENATE result ch INTO result RESPECTING BLANKS.
i = j + 1.
ENDWHILE.
ENDMETHOD.
METHOD read_node.
DATA text TYPE string.
DATA part TYPE string.
DATA tag_header TYPE string.
DATA tag_name TYPE string.
DATA ch TYPE c LENGTH 1.
DATA start TYPE i.
DATA count TYPE i.
DATA name_end TYPE i.
DATA last TYPE i.
DATA self_closing TYPE abap_bool.
DATA closed TYPE abap_bool.
DATA child TYPE REF TO $D.
CREATE OBJECT result.
result->tag = $D=>tag_object.
WHILE cursor < strlen( input ).
start = cursor.
WHILE cursor < strlen( input ) AND input+cursor(1) <> '<'.
cursor = cursor + 1.
ENDWHILE.
IF cursor = strlen( input ) AND closing IS INITIAL.
EXIT.
ENDIF.
count = cursor - start.
part = input+start(count).
CONCATENATE text part INTO text RESPECTING BLANKS.
IF cursor >= strlen( input ).
RAISE EXCEPTION TYPE $E.
ENDIF.
tag_header = header( ).
IF tag_header IS INITIAL.
RAISE EXCEPTION TYPE $E.
ENDIF.
IF tag_header+0(1) = '/'.
tag_name = substring( val = tag_header off = 1 ).
IF closing IS INITIAL OR tag_name <> closing.
RAISE EXCEPTION TYPE $E.
ENDIF.
closed = abap_true.
EXIT.
ENDIF.
IF tag_header+0(1) = '!'.
RAISE EXCEPTION TYPE $E.
ENDIF.
IF tag_header+0(1) = '?'.
IF closing IS NOT INITIAL OR tag_header NP '?xml *?'.
RAISE EXCEPTION TYPE $E.
ENDIF.
IF strlen( text ) > 0.
CREATE OBJECT child.
child->tag = $D=>tag_string.
child->sval = decode( text ).
result->put( p0 = '#text' p1 = child ).
ENDIF.
CLEAR text.
CREATE OBJECT child.
child->tag = $D=>tag_string.
result->add_xml( p0 = '?xml' p1 = child ).
CONTINUE.
ENDIF.
IF closing IS INITIAL.
CLEAR text.
ENDIF.
name_end = 0.
WHILE name_end < strlen( tag_header ).
ch = tag_header+name_end(1).
IF ch = space OR ch = '/' OR ch = cl_abap_char_utilities=>newline OR ch = cl_abap_char_utilities=>horizontal_tab.
EXIT.
ENDIF.
name_end = name_end + 1.
ENDWHILE.
IF name_end = 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
tag_name = tag_header+0(name_end).
FIND REGEX '^(__proto__|constructor|prototype)$' IN tag_name.
IF sy-subrc = 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
FIND REGEX '^[A-Za-z_][A-Za-z0-9_.:-]*$' IN tag_name.
IF sy-subrc <> 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
FIND REGEX ` + "`" + `^[A-Za-z_][A-Za-z0-9_.:-]*(\s+[A-Za-z_][A-Za-z0-9_.:-]*\s*=\s*("[^"]*"|'[^']*'))*\s*/?$` + "`" + ` IN tag_header.
IF sy-subrc <> 0.
RAISE EXCEPTION TYPE $E.
ENDIF.
last = strlen( tag_header ) - 1.
self_closing = xsdbool( tag_header+last(1) = '/' ).
IF self_closing = abap_true.
CREATE OBJECT child.
child->tag = $D=>tag_string.
ELSE.
child = read_node( tag_name ).
ENDIF.
result->add_xml( p0 = tag_name p1 = child ).
ENDWHILE.
IF closing IS NOT INITIAL AND closed = abap_false.
RAISE EXCEPTION TYPE $E.
ENDIF.
IF closing IS NOT INITIAL AND lines( result->entries ) = 0.
result->tag = $D=>tag_string.
result->sval = decode( text ).
ELSEIF strlen( text ) > 0 AND closing IS NOT INITIAL.
CREATE OBJECT child.
child->tag = $D=>tag_string.
child->sval = decode( text ).
result->put( p0 = '#text' p1 = child ).
ENDIF.
ENDMETHOD.
ENDCLASS.
`
	e.files[name+".clas.abap"] = strings.NewReplacer("$P", name, "$D", dynamic, "$E", exception).Replace(code)
}
