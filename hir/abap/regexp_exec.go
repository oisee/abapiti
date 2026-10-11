package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

// Added only for materialized exec results, keeping unrelated output stable.
func (e *emitter) regexpExecRuntime(name string) {
	arr := e.typ(hir.T(hir.Array, hir.T(hir.String)))
	file := name + ".clas.abap"
	e.files[file] = strings.Replace(e.files[file], "DATA global TYPE abap_bool.", "DATA global TYPE abap_bool.\nDATA sticky TYPE abap_bool.\nDATA last_index TYPE i.\nDATA exec_pattern TYPE string.", 1)
	e.files[file] = strings.Replace(e.files[file], "ELSEIF c = 'g'.", "ELSEIF c = 'y'.\n    sticky = abap_true.\n  ELSEIF c = 'g'.", 1)
	e.files[file] = strings.Replace(e.files[file], "METHODS test IMPORTING", "METHODS exec IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE "+arr+".\nMETHODS test IMPORTING", 1)
	e.files[file] = strings.Replace(e.files[file], "ENDMETHOD.\nMETHOD test.", "exec_pattern = posix.\nENDMETHOD.\nMETHOD test.", 1)
	e.files[file] = strings.TrimSuffix(e.files[file], "ENDCLASS.\n") + `METHOD exec.
DATA start TYPE i.
DATA hit TYPE match_result.
DATA group TYPE submatch_result.
DATA groups TYPE submatch_result_tab.
DATA text TYPE string.
DATA part TYPE string.
DATA idx TYPE i.
DATA search TYPE string.
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
search = '(?<![\s\S])[\s\S]{' && |{ start }| && '}'.
IF sticky = abap_false.
  search = search && '[\s\S]*?'.
ENDIF.
search = search && '(' && exec_pattern && ')'.
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
READ TABLE groups INDEX 1 INTO group.
hit-offset = hit-offset + hit-length - group-length.
hit-length = group-length.
DELETE groups INDEX 1.
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
