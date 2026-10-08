#!/usr/bin/env node
// Builds the phase-2 differential corpus: the 44 phase-1 lexer cases plus 20
// statement-heavy snippets, into tsfront/testdata/stmtscorpus/cases.json.
// The statement dumps (dumps.json) are produced by tools/statements-oracle.mjs
// from the original abaplint build and are what the lowered ABAP driver must
// reproduce exactly.
import {readFileSync, writeFileSync, mkdirSync} from "node:fs";
import {dirname, resolve} from "node:path";
import {fileURLToPath} from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, "../tsfront/testdata/stmtscorpus");

const lexerCases = JSON.parse(readFileSync(resolve(here, "../tsfront/testdata/lexercorpus/cases.json"), "utf8"));

const snippets = [
  {name: "stmt_data", abap: `
DATA lv_foo TYPE i.
DATA lv_bar TYPE string VALUE 'bar'.
DATA: BEGIN OF ls_str,
  name TYPE string,
  count TYPE i,
END OF ls_str.
`},
  {name: "stmt_write", abap: `
WRITE: / 'hello' INTENSIFIED OFF NO-GAP,
       / lv_foo LEFT-JUSTIFIED NO-ZERO NO-SIGN
       USING NO EDIT MASK
       TO lv_target
       AS LINE
       DD/MM/YY
       EXPONENT lv_e
       COLOR COL_HEADING INVERSE ON.
`},
  {name: "stmt_move", abap: `
lv_x = lv_a + lv_b.
lv_x = lv_a * 2 - lv_b.
lv_x += 1.
lv_x /= 2.
lv_y &&= lv_z.
MOVE EXACT lv_a TO lv_b.
MOVE lv_a TO lv_b PERCENTAGE 50.
lv_obj ?= lv_super.
`},
  {name: "stmt_if", abap: `
IF lv_x = 1 AND lv_y IS NOT INITIAL.
  WRITE 'one'.
ELSEIF lv_x BETWEEN 1 AND 5 OR NOT ( lv_y IS BOUND ).
  WRITE 'range'.
ELSE.
  WRITE 'other'.
ENDIF.
`},
  {name: "stmt_class_def", abap: `
CLASS lcl_base DEFINITION ABSTRACT.
ENDCLASS.
CLASS lcl_test DEFINITION
  INHERITING FROM lcl_base
  FINAL
  CREATE PUBLIC
  FOR TESTING
  RISK LEVEL HARMLESS
  DURATION SHORT
  SHARED MEMORY ENABLED.
  PUBLIC SECTION.
    CLASS-DATA gv_x TYPE i.
    METHODS meth IMPORTING iv_i TYPE i OPTIONAL EXPORTING ev_o TYPE string.
ENDCLASS.
`},
  {name: "stmt_colon_chain", abap: `
DATA: a TYPE i,
      b TYPE string,
      c TYPE i VALUE 3.
WRITE: / a, / b, / c.
CLEAR: a, b.
`},
  {name: "stmt_define_macro", abap: `
DEFINE write_line.
  WRITE: / &1.
END-OF-DEFINITION.
write_line 'x'.
write_line 'y'.
`},
  {name: "stmt_pragma", abap: `
DATA lv_i TYPE i ##NEEDED.
WRITE lv_i ##SUBSTITUTE_OK##.
`},
  {name: "stmt_exec_sql", abap: `
EXEC SQL.
  SELECT name INTO :lv_name FROM users WHERE id = :lv_id
ENDEXEC.
`},
  {name: "stmt_amdp", abap: `
CLASS lcl_db DEFINITION.
  PUBLIC SECTION.
    METHODS get_data
      BY DATABASE PROCEDURE FOR HDB LANGUAGE SQLSCRIPT
      OPTIONS READ-ONLY
      USING t1.
ENDCLASS.
`},
  {name: "stmt_select_single", abap: `
DATA ls_user TYPE usr02.
SELECT SINGLE * FROM usr02 INTO ls_user WHERE bname = 'X'.
`},
  {name: "stmt_select_loop", abap: `
DATA lt TYPE TABLE OF usr02.
DATA ls LIKE LINE OF lt.
SELECT * FROM usr02 INTO TABLE lt UP TO 10 ROWS.
SELECT * FROM usr02 INTO ls WHERE mandt = 100.
  WRITE ls-bname.
ENDSELECT.
SELECT COUNT( * ) FROM usr02.
`},
  {name: "stmt_select_count_agg", abap: `
DATA lv_cnt TYPE i.
SELECT COUNT( * ) INTO lv_cnt FROM mara.
SELECT bname COUNT( * ) FROM usr02 GROUP BY bname.
`},
  {name: "stmt_with_cte", abap: `
WITH +cte AS ( SELECT * FROM usr02 )
SELECT * FROM +cte INTO TABLE @DATA(lt_users).
`},
  {name: "stmt_lazy_unknown", abap: `
WRITE 'no period'
WRITE 'next statement'.
lv_missing = 1 + 2.
`},
  {name: "stmt_empty_punct", abap: `
..
WRITE 'x'..
.
`},
  {name: "stmt_comments", abap: `
* full line comment
WRITE 'x'. " trailing comment
" another
WRITE 'y'.
`},
  {name: "stmt_types_fieldsymbols", abap: `
TYPES: BEGIN OF ty_s,
  a TYPE i,
  b TYPE string,
END OF ty_s.
TYPES ty_t TYPE STANDARD TABLE OF ty_s WITH DEFAULT KEY.
FIELD-SYMBOLS <fs> TYPE ty_s.
ASSIGN ls_str TO <fs>.
UNASSIGN <fs>.
`},
  {name: "stmt_raise_call", abap: `
RAISE EXCEPTION TYPE cx_static_check EXPORTING textid = 'X'.
CALL METHOD lo_obj->( 'DYN_METH' ).
CALL FUNCTION 'RFC_PING'.
PERFORM do_something USING lv_x.
`},
  {name: "stmt_misc", abap: `
CONCATENATE lv_a lv_b INTO lv_c SEPARATED BY space RESPECTING BLANKS.
SPLIT lv_c AT ':' INTO TABLE lt_parts.
CONDENSE lv_c.
SORT lt BY a DESCENDING AS TEXT.
LOOP AT lt INTO ls WHERE a > 1.
  CONTINUE.
ENDLOOP.
RETURN.
`},
];

const cases = [...lexerCases, ...snippets];
mkdirSync(outDir, {recursive: true});
writeFileSync(resolve(outDir, "cases.json"), JSON.stringify(cases, null, 1));
console.error(`wrote ${cases.length} cases to ${outDir}/cases.json`);
