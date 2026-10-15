package tsfront

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir/abap"
)

// A4HTestSupport is self-contained: the translated closure has no ABAPITI log
// service. Checkpoints commit before entering each test, so a kernel dump cannot
// roll back the next index. No SAP connection is made by the generator.
func A4HTestSupport() (map[string]string, error) {
	files := map[string]string{"zcl_lnt_test_log.clas.abap": a4hTestLog, "zlnt_tests_run.prog.abap": a4hTestReport}
	for file, source := range files {
		formatted, err := abap.Format(source)
		if err != nil {
			return nil, err
		}
		files[file] = formatted
	}
	var fields strings.Builder
	for _, f := range []struct {
		name, typ, length, internal string
		key                         bool
	}{
		{"MANDT", "CLNT", "000003", "000006", true},
		{"RUN_ID", "CHAR", "000032", "000064", true},
		{"SEQ", "INT4", "000010", "000004", true},
		{"CHUNK_NO", "INT4", "000010", "000004", true},
		{"SHARD", "CHAR", "000040", "000080", false},
		{"NEXT_IDX", "INT4", "000010", "000004", false},
		{"USED", "INT4", "000010", "000004", false},
		{"PAYLOAD", "CHAR", "000255", "000510", false},
	} {
		key := ""
		if f.key {
			key = "X"
		}
		inttype := "C"
		if f.typ == "INT4" {
			inttype = "I"
		}
		fmt.Fprintf(&fields, "<DD03P><FIELDNAME>%s</FIELDNAME><KEYFLAG>%s</KEYFLAG><NOTNULL>%s</NOTNULL><ADMINFIELD>0</ADMINFIELD><COMPTYPE>E</COMPTYPE><INTTYPE>%s</INTTYPE><INTLEN>%s</INTLEN><DATATYPE>%s</DATATYPE><LENG>%s</LENG><MASK>  %s</MASK></DD03P>\n", f.name, key, key, inttype, f.internal, f.typ, f.length, f.typ)
	}
	files["zlnt_testrun.tabl.xml"] = `<?xml version="1.0" encoding="utf-8"?>
<abapGit version="v1.0.0" serializer="LCL_OBJECT_TABL" serializer_version="v1.0.0">
<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0"><asx:values>
<DD02V><TABNAME>ZLNT_TESTRUN</TABNAME><DDLANGUAGE>E</DDLANGUAGE><TABCLASS>TRANSP</TABCLASS><CLIDEP>X</CLIDEP><DDTEXT>abaplint test checkpoints and chunked JSON</DDTEXT><MAINFLAG>X</MAINFLAG><CONTFLAG>A</CONTFLAG><EXCLASS>1</EXCLASS></DD02V>
<DD09L><TABNAME>ZLNT_TESTRUN</TABNAME><AS4LOCAL>A</AS4LOCAL><TABKAT>0</TABKAT><TABART>APPL0</TABART><BUFALLOW>N</BUFALLOW></DD09L>
<DD03P_TABLE>` + fields.String() + `</DD03P_TABLE>
</asx:values></asx:abap></abapGit>
`
	return files, nil
}

const a4hTestLog = `CLASS zcl_lnt_test_log DEFINITION PUBLIC FINAL CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS begin IMPORTING run_id TYPE c shard TYPE c start_index TYPE i RETURNING VALUE(next_index) TYPE i.
CLASS-METHODS append IMPORTING value TYPE string.
PROTECTED SECTION.
PRIVATE SECTION.
CLASS-DATA id TYPE c LENGTH 32.
CLASS-DATA selected TYPE c LENGTH 40.
CLASS-DATA seq TYPE i.
CLASS-DATA next TYPE i.
ENDCLASS.
CLASS zcl_lnt_test_log IMPLEMENTATION.
METHOD begin.
DATA row TYPE zlnt_testrun.
id = run_id.
selected = shard.
SELECT MAX( seq ) FROM zlnt_testrun WHERE run_id = @id INTO @seq.
next = 1.
IF seq > 0.
SELECT SINGLE * FROM zlnt_testrun WHERE run_id = @id AND seq = @seq AND chunk_no = 1 INTO @row.
IF row-shard <> selected.
MESSAGE 'Run ID already belongs to another shard' TYPE 'E'.
ENDIF.
next = row-next_idx.
ENDIF.
IF start_index > 0.
next = start_index.
ENDIF.
next_index = next.
TRY.
CALL METHOD ('ZCL_LNT_TEST_LOG_BAL')=>('BEGIN') EXPORTING run_id = id shard = selected next_index = next.
CATCH cx_sy_dyn_call_error.
ENDTRY.
append( '{"kind":"resume","startIndex":' && |{ next }| && '}' ).
ENDMETHOD.
METHOD append.
DATA row TYPE zlnt_testrun.
DATA offset TYPE i.
DATA count TYPE i.
DATA index_text TYPE string.
FIND REGEX '"kind":"test","index":([0-9]+)' IN value SUBMATCHES index_text.
IF sy-subrc = 0.
next = CONV i( index_text ) + 1.
ENDIF.
seq = seq + 1.
row-mandt = sy-mandt.
row-run_id = id.
row-seq = seq.
row-shard = selected.
row-next_idx = next.
WHILE offset < strlen( value ).
count = nmin( val1 = 255 val2 = strlen( value ) - offset ).
row-chunk_no = row-chunk_no + 1.
row-used = count.
row-payload = substring( val = value off = offset len = count ).
INSERT zlnt_testrun FROM @row.
IF sy-subrc <> 0.
MESSAGE 'Test log insert failed; stop rather than lose checkpoints' TYPE 'A'.
ENDIF.
offset = offset + count.
ENDWHILE.
TRY.
CALL METHOD ('ZCL_LNT_TEST_LOG_BAL')=>('EVENT') EXPORTING value = value.
CATCH cx_sy_dyn_call_error.
ENDTRY.
COMMIT WORK AND WAIT.
ENDMETHOD.
ENDCLASS.
`

// A4HTestLogBAL is an optional SAP BAL/SLG1 mirror, shipped only in the
// registry archive. ZLNT_TESTRUN remains the source of truth for checkpoints.
func A4HTestLogBAL() (string, error) {
	return abap.Format(a4hTestLogBAL)
}

const a4hTestLogBAL = `CLASS zcl_lnt_test_log_bal DEFINITION PUBLIC FINAL CREATE PRIVATE.
PUBLIC SECTION.
CLASS-METHODS begin IMPORTING run_id TYPE c shard TYPE c next_index TYPE i.
CLASS-METHODS event IMPORTING value TYPE string.
PRIVATE SECTION.
CLASS-DATA id TYPE c LENGTH 32.
CLASS-DATA selected TYPE c LENGTH 40.
CLASS-DATA handle TYPE balloghndl.
CLASS-DATA started TYPE timestampl.
CLASS-DATA passed TYPE i.
CLASS-DATA failed TYPE i.
CLASS-DATA trapped TYPE i.
CLASS-DATA not_run TYPE i.
CLASS-DATA async TYPE i.
CLASS-METHODS bal IMPORTING type TYPE symsgty text TYPE csequence.
ENDCLASS.
CLASS zcl_lnt_test_log_bal IMPLEMENTATION.
METHOD begin.
id = run_id.
selected = shard.
DATA header TYPE bal_s_log.
header-object = 'ZABAPITI'.
header-subobject = 'CHECK'.
header-extnumber = |TESTS { id } { selected }|.
CALL FUNCTION 'BAL_LOG_CREATE' EXPORTING i_s_log = header IMPORTING e_log_handle = handle EXCEPTIONS OTHERS = 1.
IF sy-subrc <> 0.
CLEAR handle.
ENDIF.
GET TIME STAMP FIELD started.
bal( type = 'I' text = |start run { id } shard { selected } at index { next_index }| ).
ENDMETHOD.
METHOD bal.
DATA free TYPE c LENGTH 200.
DATA handles TYPE bal_t_logh.
IF handle IS INITIAL.
RETURN.
ENDIF.
free = text.
CALL FUNCTION 'BAL_LOG_MSG_ADD_FREE_TEXT' EXPORTING i_log_handle = handle i_msgty = type i_text = free EXCEPTIONS OTHERS = 1.
INSERT handle INTO TABLE handles.
CALL FUNCTION 'BAL_DB_SAVE' EXPORTING i_t_log_handle = handles EXCEPTIONS OTHERS = 1.
ENDMETHOD.
METHOD event.
DATA status TYPE string.
DATA title TYPE string.
DATA message TYPE string.
DATA now TYPE timestampl.
DATA seconds TYPE tzntstmpl.
IF value CP '{"kind":"registry"*'.
bal( type = 'I' text = |registry: { count( val = value sub = '","' ) + 1 } tests| ).
ELSEIF value CP '{"kind":"result"*'.
FIND PCRE '"status":"([a-z-]+)"\}\}$' IN value SUBMATCHES status.
CASE status.
WHEN 'passed'.
passed = passed + 1.
WHEN 'failed'.
failed = failed + 1.
WHEN 'trapped'.
trapped = trapped + 1.
WHEN 'not-run'.
not_run = not_run + 1.
WHEN OTHERS.
async = async + 1.
ENDCASE.
IF status <> 'passed' AND status <> 'not-run'.
FIND PCRE '"title":"((?:[^"\\]|\\.)*)"' IN value SUBMATCHES title.
FIND PCRE '"message":"((?:[^"\\]|\\.)*)"' IN value SUBMATCHES message.
bal( type = 'E' text = |{ status }: { title }: { message }| ).
ENDIF.
ELSEIF value CP '{"kind":"complete"*'.
GET TIME STAMP FIELD now.
seconds = cl_abap_tstmp=>subtract( tstmp1 = now tstmp2 = started ).
bal( type = 'S' text = |done shard { selected } run { id }: passed { passed } failed { failed } trapped { trapped } not-run { not_run } async { async } in { seconds DECIMALS = 0 } s| ).
ENDIF.
ENDMETHOD.
ENDCLASS.
`

const a4hTestReport = `REPORT zlnt_tests_run.
PARAMETERS p_shard TYPE c LENGTH 40 OBLIGATORY LOWER CASE.
PARAMETERS p_start TYPE i DEFAULT 0.
PARAMETERS p_run TYPE c LENGTH 32 OBLIGATORY LOWER CASE.
START-OF-SELECTION.
DATA next_index TYPE i.
DATA result TYPE string.
next_index = zcl_lnt_test_log=>begin( run_id = p_run shard = p_shard start_index = p_start ).
result = zcl_lnt_test_runner=>run( start_index = next_index skip_init = '' shard = CONV string( p_shard ) ).
IF result IS INITIAL.
MESSAGE 'Unknown shard; runner produced no JSON' TYPE 'E'.
ENDIF.
zcl_lnt_test_log=>append( '{"kind":"complete","report":' && result && '}' ).
WRITE: / p_run, p_shard, next_index.
`
