package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

func replacementNeedsSubstitution(x *hir.Expr) bool {
	if x == nil || x.Kind != hir.Lit {
		return true
	}
	s, ok := x.Value.(string)
	return !ok || strings.Contains(s, "$")
}

// Empty needles need the UTF-16 insertion path for replaceAll even when
// the replacement itself does not need GetSubstitution.
func literalReplacementNeedsRuntime(x *hir.Expr) bool {
	if replacementNeedsSubstitution(x.Args[1]) {
		return true
	}
	needle, ok := x.Args[0].Value.(string)
	return x.Op == "string.replaceAll" && (x.Args[0].Kind != hir.Lit || !ok || needle == "")
}

// Shared GetSubstitution for string and checked-RegExp replacement. All
// offsets and slices are UTF-16 units; captures include the whole match.
func (e *emitter) replacementRuntime() string {
	id := "runtime.replacement"
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	arr := e.typ(hir.T(hir.Array, hir.T(hir.String)))
	base := e.name("runtime.regexp")
	js := e.name("runtime.regexp_js")
	code := `CLASS $P DEFINITION PUBLIC CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS substitute IMPORTING text TYPE string replacement TYPE string start TYPE i finish TYPE i captures TYPE $A OPTIONAL source TYPE string OPTIONAL RETURNING VALUE(result) TYPE string.
CLASS-METHODS literal IMPORTING text TYPE string needle TYPE string replacement TYPE string all_matches TYPE abap_bool RETURNING VALUE(result) TYPE string.
CLASS-METHODS regex IMPORTING text TYPE string pattern TYPE REF TO $R replacement TYPE string RETURNING VALUE(result) TYPE string.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS $P IMPLEMENTATION.
METHOD substitute.
TYPES: BEGIN OF named_capture, name TYPE string, idx TYPE i, END OF named_capture.
DATA names TYPE STANDARD TABLE OF named_capture WITH DEFAULT KEY.
DATA entry TYPE named_capture.
DATA i TYPE i.
DATA j TYPE i.
DATA k TYPE i.
DATA n TYPE i.
DATA d TYPE i.
DATA count TYPE i.
DATA total TYPE i.
DATA c TYPE string.
DATA next TYPE string.
DATA part TYPE string.
DATA escaped TYPE abap_bool.
DATA in_class TYPE abap_bool.
DATA named TYPE abap_bool.
IF captures IS BOUND.
 total = lines( captures->items ) - 1.
ENDIF.
WHILE i < strlen( source ).
 c = source+i(1).
 IF escaped = abap_true.
  escaped = abap_false.
 ELSEIF c = '\'.
  escaped = abap_true.
 ELSEIF c = '['.
  in_class = abap_true.
 ELSEIF c = ']'.
  in_class = abap_false.
 ELSEIF c = '(' AND in_class = abap_false.
  j = i + 1.
  IF j >= strlen( source ) OR source+j(1) <> '?'.
   count = count + 1.
  ELSE.
   j = j + 1.
   IF j < strlen( source ) AND source+j(1) = '<'.
    count = count + 1.
    named = abap_true.
    j = j + 1.
    k = j.
    WHILE j < strlen( source ) AND source+j(1) <> '>'.
     j = j + 1.
    ENDWHILE.
    n = j - k.
    entry-name = source+k(n).
    entry-idx = count + 1.
    APPEND entry TO names.
   ENDIF.
  ENDIF.
 ENDIF.
 i = i + 1.
ENDWHILE.
i = 0.
WHILE i < strlen( replacement ).
 c = replacement+i(1).
 j = i + 1.
 IF c <> '$' OR j >= strlen( replacement ).
  result = result && c.
 ELSE.
  next = replacement+j(1).
  CASE next.
  WHEN '$'.
   result = result && '$'.
   i = i + 1.
  WHEN '&'.
   n = finish - start.
   result = result && text+start(n).
   i = i + 1.
  WHEN '$BACKTICK'.
   result = result && text(start).
   i = i + 1.
  WHEN ''''.
   n = strlen( text ) - finish.
   result = result && text+finish(n).
   i = i + 1.
  WHEN '<'.
   k = j + 1.
   j = k.
   WHILE j < strlen( replacement ) AND replacement+j(1) <> '>'.
    j = j + 1.
   ENDWHILE.
   IF named = abap_true AND j < strlen( replacement ).
    n = j - k.
    part = replacement+k(n).
    READ TABLE names WITH KEY name = part INTO entry.
    IF sy-subrc = 0.
     READ TABLE captures->items INDEX entry-idx INTO part.
     result = result && part.
    ENDIF.
    i = j.
   ELSE.
    result = result && c.
   ENDIF.
  WHEN OTHERS.
   IF next CO '0123456789'.
    d = next.
    count = 1.
    j = i + 2.
    IF j < strlen( replacement ).
     next = replacement+j(1).
     IF next CO '0123456789'.
      n = next.
      n = d * 10 + n.
      IF n <= total.
       d = n.
       count = 2.
      ENDIF.
     ENDIF.
    ENDIF.
    IF d > 0 AND d <= total.
     d = d + 1.
     READ TABLE captures->items INDEX d INTO part.
     result = result && part.
     i = i + count.
    ELSE.
     result = result && c.
    ENDIF.
   ELSE.
    result = result && c.
   ENDIF.
  ENDCASE.
 ENDIF.
 i = i + 1.
ENDWHILE.
ENDMETHOD.
METHOD literal.
DATA at TYPE i.
DATA last TYPE i.
DATA rel TYPE i.
DATA start TYPE i.
DATA finish TYPE i.
DATA n TYPE i.
DATA rest TYPE string.
WHILE at <= strlen( text ).
 IF needle IS INITIAL.
  rel = 0.
 ELSE.
  n = strlen( text ) - at.
  rest = text+at(n).
  rel = find( val = rest sub = needle ).
  IF rel < 0.
   EXIT.
  ENDIF.
 ENDIF.
 start = at + rel.
 finish = start + strlen( needle ).
 n = start - last.
 result = result && text+last(n) && substitute( text = text replacement = replacement start = start finish = finish ).
 last = finish.
 IF all_matches = abap_false.
  EXIT.
 ENDIF.
 at = finish.
 IF needle IS INITIAL.
  at = at + 1.
 ENDIF.
ENDWHILE.
n = strlen( text ) - last.
result = result && text+last(n).
ENDMETHOD.
METHOD regex.
DATA hit TYPE $A.
DATA start TYPE i.
DATA finish TYPE i.
DATA last TYPE i.
DATA n TYPE i.
IF pattern->global = abap_true.
 pattern->last_index = 0.
ENDIF.
DO.
 hit = $J=>exec_legacy( p0 = pattern p1 = text ).
 IF hit IS NOT BOUND.
  EXIT.
 ENDIF.
 start = hit->match_index.
 READ TABLE hit->items INDEX 1 INTO DATA(matched).
 finish = start + strlen( matched ).
 n = start - last.
 result = result && text+last(n) && substitute( text = text replacement = replacement start = start finish = finish captures = hit source = pattern->source ).
 last = finish.
 IF pattern->global = abap_false.
  EXIT.
 ENDIF.
 IF finish = start.
  pattern->last_index = pattern->last_index + 1.
 ENDIF.
ENDDO.
n = strlen( text ) - last.
result = result && text+last(n).
ENDMETHOD.
ENDCLASS.
`
	code = strings.NewReplacer("$BACKTICK", "`", "$P", name, "$A", arr, "$R", base, "$J", js).Replace(code)
	e.files[name+".clas.abap"] = code
	return name
}
