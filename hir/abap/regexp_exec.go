package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

// Added only for materialized exec results, keeping unrelated output stable.
func (e *emitter) regexpExecRuntime(name string) {
	arr := e.typ(hir.T(hir.Array, hir.T(hir.String)))
	dynamic := e.name("runtime.dynamic")
	file := name + ".clas.abap"
	e.files[file] = strings.Replace(e.files[file], "DATA global TYPE abap_bool.", "DATA global TYPE abap_bool.\nDATA sticky TYPE abap_bool.\nDATA last_index TYPE i.\nDATA exec_pattern TYPE string.", 1)
	e.files[file] = strings.Replace(e.files[file], "ELSEIF c = 'g'.", "ELSEIF c = 'y'.\n    sticky = abap_true.\n  ELSEIF c = 'g'.", 1)
	e.files[file] = strings.Replace(e.files[file], "METHODS test IMPORTING", "METHODS exec IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE "+arr+".\nMETHODS test IMPORTING", 1)
	e.files[file] = strings.Replace(e.files[file], "ENDMETHOD.\nMETHOD test.", "exec_pattern = posix.\nENDMETHOD.\nMETHOD test.", 1)
	e.files[file] = strings.TrimSuffix(e.files[file], "ENDCLASS.\n") + `METHOD exec.
DATA start TYPE i.
DATA cursor_base TYPE i.
DATA text TYPE string.
DATA part TYPE string.
DATA idx TYPE i.
DATA search TYPE string.
DATA hit TYPE match_result.
DATA groups TYPE submatch_result_tab.
DATA group TYPE submatch_result.
DATA named_row TYPE group_name_type.
DATA named_value TYPE REF TO ` + dynamic + `.
IF global = abap_true OR sticky = abap_true.
  start = nmax( val1 = 0 val2 = last_index ).
ENDIF.
IF start > strlen( p0 ).
  last_index = 0.
  RETURN.
ENDIF.
text = me->unit_text( p0 ).
IF excluded IS NOT INITIAL.
  FIND REGEX excluded IN text.
  IF sy-subrc = 0.
    IF global = abap_true OR sticky = abap_true.
      last_index = 0.
    ENDIF.
    RETURN.
  ENDIF.
ENDIF.
IF global = abap_true OR sticky = abap_true.
  search = '(?<![\s\S])'.
  IF start > 0.
    cursor_base = start - 1.
    text = text+cursor_base.
    search = search && '[\s\S]'.
  ENDIF.
  IF sticky = abap_false.
    search = search && '[\s\S]*?'.
  ENDIF.
  search = search && '(' && exec_pattern && ')'.
ELSE.
  search = exec_pattern.
ENDIF.
IF ignore_case = abap_true.
  FIND FIRST OCCURRENCE OF REGEX search IN text IGNORING CASE RESULTS hit.
ELSE.
  FIND FIRST OCCURRENCE OF REGEX search IN text RESULTS hit.
ENDIF.
IF sy-subrc <> 0.
  IF global = abap_true OR sticky = abap_true.
    last_index = 0.
  ENDIF.
  RETURN.
ENDIF.
groups = hit-submatches.
IF global = abap_true OR sticky = abap_true.
  READ TABLE groups INDEX 1 INTO group.
  hit-offset = hit-offset + hit-length - group-length.
  hit-length = group-length.
  DELETE groups INDEX 1.
ENDIF.
IF cursor_base > 0.
  hit-offset = hit-offset + cursor_base.
  LOOP AT groups INTO group.
    IF group-offset >= 0.
      group-offset = group-offset + cursor_base.
      MODIFY groups FROM group INDEX sy-tabix.
    ENDIF.
  ENDLOOP.
ENDIF.
IF source = '^(-?)0+(?=\d)'.
 hit-length = hit-length - 1.
 DELETE groups INDEX lines( groups ).
ENDIF.
IF global = abap_true OR sticky = abap_true.
  last_index = hit-offset + hit-length.
ENDIF.
CREATE OBJECT result.
result->match_index = hit-offset.
result->match_has_index = abap_true.
part = p0+hit-offset(hit-length).
APPEND part TO result->items.
LOOP AT groups INTO group.
  IF group-offset < 0.
    idx = lines( result->items ).
    INSERT idx INTO TABLE result->match_missing.
    CLEAR part.
  ELSE.
    part = p0+group-offset(group-length).
  ENDIF.
  APPEND part TO result->items.
ENDLOOP.
IF group_names IS NOT INITIAL.
  CREATE OBJECT result->match_groups.
  result->match_groups->tag = ` + dynamic + `=>tag_object.
  LOOP AT group_names INTO named_row.
    READ TABLE groups INDEX named_row-index INTO group.
    IF sy-subrc = 0 AND group-offset >= 0.
      CREATE OBJECT named_value.
      named_value->tag = ` + dynamic + `=>tag_string.
      named_value->sval = p0+group-offset(group-length).
    ENDIF.
    APPEND VALUE #( k = named_row-name v = named_value ) TO result->match_groups->entries.
    CLEAR named_value.
  ENDLOOP.
ENDIF.
ENDMETHOD.
ENDCLASS.
`
	text := e.files[file]
	start := strings.Index(text, "METHOD test.\n")
	end := strings.Index(text, "METHOD match_test.\n")
	text = text[:start] + "METHOD test.\nDATA hit TYPE " + arr + ".\nhit = me->exec( p0 ).\nIF hit IS BOUND.\nresult = abap_true.\nENDIF.\nENDMETHOD.\n" + text[end:]
	start = strings.Index(text, "METHOD match_test.\n")
	end = strings.Index(text, "METHOD replace.\n")
	text = text[:start] + "METHOD match_test.\nIF global = abap_true.\nlast_index = 0.\nENDIF.\nresult = me->test( p0 ).\nIF global = abap_true.\nlast_index = 0.\nENDIF.\nENDMETHOD.\n" + text[end:]
	text = strings.Replace(text, "METHOD match.\n", "METHOD match.\nIF global = abap_true.\nlast_index = 0.\nENDIF.\n", 1)
	e.files[file] = text
}
