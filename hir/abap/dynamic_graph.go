package abap

import "strings"

// The graph representation preserves absence (an initial reference), null,
// primitive values, ordered object entries and distinct arrays. It is shared
// by the subset adapters; it does not flatten XML into object descriptions.
func dynamicGraphDefinition(name string) string {
	return strings.ReplaceAll(`
CONSTANTS: tag_object TYPE i VALUE 5, tag_array TYPE i VALUE 6, tag_boolean TYPE i VALUE 7, tag_null TYPE i VALUE 8.
TYPES: BEGIN OF entry, k TYPE string, v TYPE REF TO $D, END OF entry.
TYPES entries_type TYPE STANDARD TABLE OF entry WITH DEFAULT KEY.
TYPES items_type TYPE STANDARD TABLE OF REF TO $D WITH DEFAULT KEY.
DATA entries TYPE entries_type.
DATA items TYPE items_type.
TYPES missing_type TYPE SORTED TABLE OF i WITH UNIQUE KEY table_line.
DATA missing TYPE missing_type.
METHODS array_index IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE i.
DATA bval TYPE abap_bool.
METHODS get IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE REF TO $D.
METHODS put IMPORTING p0 TYPE string p1 TYPE REF TO $D.
METHODS append IMPORTING p0 TYPE REF TO $D.
METHODS add_xml IMPORTING p0 TYPE string p1 TYPE REF TO $D.
METHODS truth RETURNING VALUE(result) TYPE abap_bool.
METHODS as_boolean RETURNING VALUE(result) TYPE abap_bool.
METHODS strict_equals IMPORTING p0 TYPE REF TO $D RETURNING VALUE(result) TYPE abap_bool.
`, "$D", name)
}
func dynamicGraphImplementation(name string) string {
	return strings.ReplaceAll(`
METHOD get.
IF tag = tag_object.
READ TABLE entries WITH KEY k = p0 INTO DATA(row).
IF sy-subrc = 0.
result = row-v.
ENDIF.
ELSEIF tag = tag_array.
IF p0 = 'length'.
CREATE OBJECT result.
result->tag = tag_number.
result->nval = lines( items ).
ELSE.
DATA index TYPE i.
index = array_index( p0 ) + 1.
READ TABLE items INDEX index INTO result.
ENDIF.
ELSEIF tag = tag_string.
IF p0 = 'length'.
CREATE OBJECT result.
result->tag = tag_number.
result->nval = strlen( sval ).
ENDIF.
ELSEIF tag = tag_number OR tag = tag_boolean.
RETURN.
ELSE.
RAISE EXCEPTION TYPE cx_sy_move_cast_error.
ENDIF.
ENDMETHOD.
METHOD array_index.
IF p0 CN '0123456789' OR p0 IS INITIAL OR strlen( p0 ) > 10 OR ( strlen( p0 ) > 1 AND p0+0(1) = '0' ).
RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
DATA number TYPE int8.
number = p0.
IF number >= 2147483647.
RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
result = number.
ENDMETHOD.
METHOD put.
IF tag = tag_array.
DATA index TYPE i.
DATA hole TYPE i.
DATA empty TYPE REF TO $D.
index = array_index( p0 ) + 1.
WHILE lines( items ) < index.
APPEND empty TO items.
hole = lines( items ).
INSERT hole INTO TABLE missing.
ENDWHILE.
MODIFY items FROM p1 INDEX index.
DELETE missing WHERE table_line = index.
RETURN.
ENDIF.
IF tag <> tag_object.
RAISE EXCEPTION TYPE cx_sy_move_cast_error.
ENDIF.
READ TABLE entries WITH KEY k = p0 ASSIGNING FIELD-SYMBOL(<row>).
IF sy-subrc = 0.
<row>-v = p1.
ELSE.
APPEND VALUE #( k = p0 v = p1 ) TO entries.
ENDIF.
ENDMETHOD.
METHOD append.
IF tag <> tag_array.
RAISE EXCEPTION TYPE cx_sy_move_cast_error.
ENDIF.
APPEND p0 TO items.
ENDMETHOD.
METHOD add_xml.
DATA old TYPE REF TO $D.
DATA arr TYPE REF TO $D.
old = get( p0 ).
IF old IS NOT BOUND.
put( p0 = p0 p1 = p1 ).
ELSEIF old->tag = tag_array.
old->append( p1 ).
ELSE.
CREATE OBJECT arr.
arr->tag = tag_array.
arr->append( old ).
arr->append( p1 ).
put( p0 = p0 p1 = arr ).
ENDIF.
ENDMETHOD.
METHOD truth.
CASE tag.
WHEN tag_null.
result = abap_false.
WHEN tag_bigint.
result = xsdbool( sval <> '0' ).
WHEN tag_string.
result = xsdbool( strlen( sval ) > 0 ).
WHEN tag_number.
IF number_special <> 0.
result = xsdbool( number_special <> 1 ).
ELSE.
result = xsdbool( nval <> 0 ).
ENDIF.
WHEN tag_boolean.
result = bval.
WHEN OTHERS.
result = abap_true.
ENDCASE.
ENDMETHOD.
METHOD as_boolean.
IF tag <> tag_boolean.
RAISE EXCEPTION TYPE cx_sy_move_cast_error.
ENDIF.
result = bval.
ENDMETHOD.
METHOD strict_equals.
IF p0 IS NOT BOUND OR tag <> p0->tag.
RETURN.
ENDIF.
CASE tag.
WHEN tag_string OR tag_bigint.
result = xsdbool( sval = p0->sval ).
WHEN tag_number.
IF number_special <> 0 OR p0->number_special <> 0.
result = xsdbool( number_special = p0->number_special AND number_special <> 1 ).
ELSE.
result = xsdbool( nval = p0->nval ).
ENDIF.
WHEN tag_boolean.
result = xsdbool( bval = p0->bval ).
WHEN tag_null.
result = abap_true.
WHEN tag_ref.
result = xsdbool( oval = p0->oval ).
WHEN tag_class.
result = xsdbool( cval = p0->cval ).
WHEN OTHERS.
result = xsdbool( me = p0 ).
ENDCASE.
ENDMETHOD.
`, "$D", name)
}
