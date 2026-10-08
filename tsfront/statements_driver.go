package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type StatementCase struct {
	Name       string `json:"name"`
	Abap       string `json:"abap"`
	Dump       string `json:"dump"`
	Tokens     int    `json:"tokens"`
	Statements int    `json:"statements"`
}

func LoadStatementsCorpus(dir string) ([]StatementCase, error) {
	var cases, oracle []StatementCase
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		return nil, err
	}
	raw, err = os.ReadFile(filepath.Join(dir, "dumps.json"))
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &oracle); err != nil {
		return nil, err
	}
	if len(cases) != len(oracle) {
		return nil, fmt.Errorf("statement corpus/oracle lengths differ")
	}
	for i := range cases {
		if cases[i].Name != oracle[i].Name {
			return nil, fmt.Errorf("statement case %d names differ", i)
		}
		cases[i].Dump = oracle[i].Dump
		cases[i].Tokens = oracle[i].Tokens
		cases[i].Statements = oracle[i].Statements
		if oracle[i].Statements < 0 || strings.Count(oracle[i].Dump, "\n") != oracle[i].Statements {
			return nil, fmt.Errorf("case %s has invalid statement count", cases[i].Name)
		}
	}
	return cases, nil
}

// One test method avoids the runtime's test-method limit. Data methods keep
// large independent oracle dumps out of the executable comparison method.
// Assertions run after all cases, so partial equality and exceptions are counted.
func StatementsTestClass(cases []StatementCase, p DriverParams, tokens, statements string) string {
	var b strings.Builder
	line := func(s string, args ...any) { fmt.Fprintf(&b, s+"\n", args...) }
	line("CLASS ltcl_statements_diff DEFINITION FOR TESTING DURATION LONG RISK LEVEL HARMLESS.")
	line("PRIVATE SECTION.")
	for _, n := range []string{"completed", "equal_cases", "equal_statements", "total_statements", "actual_statements", "equal_tokens", "total_tokens"} {
		line("DATA %s TYPE i.", n)
	}
	line("DATA first_mismatch TYPE string.")
	line("METHODS statements FOR TESTING.")
	line("METHODS compare IMPORTING case_name TYPE string raw TYPE string expected TYPE string tokens TYPE i statements TYPE i.")
	for i := range cases {
		line("METHODS data_%03d EXPORTING raw TYPE string expected TYPE string.", i)
	}
	line("ENDCLASS.")
	line("CLASS ltcl_statements_diff IMPLEMENTATION.")
	line("METHOD statements.")
	line("DATA raw TYPE string.")
	line("DATA expected TYPE string.")
	line("DATA msg TYPE string.")
	for i, c := range cases {
		line("CALL METHOD data_%03d IMPORTING raw = raw expected = expected.", i)
		line("CALL METHOD compare EXPORTING case_name = `%s` raw = raw expected = expected tokens = %d statements = %d.", strings.ReplaceAll(c.Name, "`", "``"), c.Tokens, c.Statements)
	}
	line("cl_abap_unit_assert=>assert_equals( act = completed exp = %d msg = `completion` ).", len(cases))
	line("msg = |cases { equal_cases }/{ completed } statements { equal_statements }/{ total_statements } actual { actual_statements }|.")
	line("msg = msg && | tokens { equal_tokens }/{ completed } total_tokens { total_tokens } first { first_mismatch }|.")
	line("cl_abap_unit_assert=>assert_equals( act = equal_cases exp = %d msg = msg ).", len(cases))
	total := 0
	for _, c := range cases {
		total += c.Statements
	}
	line("cl_abap_unit_assert=>assert_equals( act = equal_statements exp = %d msg = msg ).", total)
	line("cl_abap_unit_assert=>assert_equals( act = equal_tokens exp = %d msg = msg ).", len(cases))
	line("ENDMETHOD.")
	line("METHOD compare.")
	line("DATA actual TYPE string.")
	line("DATA diff TYPE f.")
	line("DATA idx TYPE i.")
	line("DATA actual_lines TYPE STANDARD TABLE OF string WITH DEFAULT KEY.")
	line("DATA expected_lines TYPE STANDARD TABLE OF string WITH DEFAULT KEY.")
	line("DATA actual_line TYPE string.")
	line("DATA expected_line TYPE string.")
	line("DATA newline TYPE string.")
	line("DATA err TYPE REF TO cx_root.")
	line("newline = cl_abap_char_utilities=>newline.")
	line("%s=>%s = -1.", p.Class, tokens)
	line("%s=>%s = -1.", p.Class, statements)
	line("TRY.")
	line("CALL METHOD %s=>%s EXPORTING %s = raw RECEIVING result = actual.", p.Class, p.Dump, p.Raw)
	line("CATCH cx_root INTO err.")
	line("actual = `EXCEPTION|` && err->get_text( ).")
	line("ENDTRY.")
	line("completed = completed + 1.")
	line("total_statements = total_statements + statements.")
	line("total_tokens = total_tokens + tokens.")
	line("IF %s=>%s >= 0.", p.Class, statements)
	line("actual_statements = actual_statements + %s=>%s.", p.Class, statements)
	line("ENDIF.")
	line("IF %s=>%s = tokens.", p.Class, tokens)
	line("equal_tokens = equal_tokens + 1.")
	line("ENDIF.")
	line("CALL METHOD %s=>%s EXPORTING %s = actual %s = expected RECEIVING result = diff.", p.Class, p.Diff, p.A, p.B)
	line("IF diff = -1 AND %s=>%s = statements.", p.Class, statements)
	line("equal_cases = equal_cases + 1.")
	line("ELSEIF first_mismatch IS INITIAL.")
	line("first_mismatch = |{ case_name } diff { diff } tokens { %s=>%s }/{ tokens } stmts { %s=>%s }/{ statements }|.", p.Class, tokens, p.Class, statements)
	line("IF strlen( actual ) > 180.")
	line("actual_line = actual(180).")
	line("ELSE.")
	line("actual_line = actual.")
	line("ENDIF.")
	line("first_mismatch = first_mismatch && ` actual=` && actual_line.")
	line("ENDIF.")
	line("SPLIT actual AT newline INTO TABLE actual_lines.")
	line("SPLIT expected AT newline INTO TABLE expected_lines.")
	line("LOOP AT expected_lines INTO expected_line.")
	line("idx = sy-tabix.")
	line("IF idx = 1.")
	line("CONTINUE.")
	line("ENDIF.")
	line("CLEAR actual_line.")
	line("READ TABLE actual_lines INDEX idx INTO actual_line.")
	line("IF sy-subrc = 0.")
	line("CALL METHOD %s=>%s EXPORTING %s = actual_line %s = expected_line RECEIVING result = diff.", p.Class, p.Diff, p.A, p.B)
	line("IF diff = -1.")
	line("equal_statements = equal_statements + 1.")
	line("ENDIF.")
	line("ENDIF.")
	line("ENDLOOP.")
	line("ENDMETHOD.")
	for i, c := range cases {
		line("METHOD data_%03d.", i)
		line("DATA ch TYPE c LENGTH 1.")
		line("CLEAR raw.")
		line("CLEAR expected.")
		for _, s := range abapStringBuild("raw", "ch", c.Abap) {
			line("%s", s)
		}
		for _, s := range abapStringBuild("expected", "ch", c.Dump) {
			line("%s", s)
		}
		line("ENDMETHOD.")
	}
	line("ENDCLASS.")
	return b.String()
}
