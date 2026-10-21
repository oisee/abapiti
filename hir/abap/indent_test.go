package abap

import (
	"strings"
	"testing"
)

func TestIndentBlocks(t *testing.T) {
	want := `CLASS zcl_test DEFINITION.
  PUBLIC SECTION.
    METHODS run.
  PROTECTED SECTION.
    DATA: a TYPE i,
    b TYPE i.
  PRIVATE SECTION.
ENDCLASS.
CLASS zcl_test IMPLEMENTATION.
  METHOD run.
    IF a = 1.
      LOOP AT tab INTO row.
        TRY.
            CASE a.
              WHEN 1.
                WRITE a.
              WHEN OTHERS.
                WRITE b.
            ENDCASE.
          CATCH cx_root.
            WRITE a.
          CLEANUP.
            WRITE b.
        ENDTRY.
      ENDLOOP.
    ELSEIF a = 2.
      WHILE a > 0.
        a = a - 1.
      ENDWHILE.
    ELSE.
      DO 2 TIMES.
* column-zero comment
        WRITE 'ENDIF. ''TRY.'''. " IF ignored
      ENDDO.
    ENDIF.
  ENDMETHOD.
ENDCLASS.
INTERFACE zif_test.
  METHODS run.
ENDINTERFACE.
START-OF-SELECTION.
  WRITE 'start'.
END-OF-SELECTION.
  WRITE 'end'.
FORM run.
  WRITE 'form'.
ENDFORM.
`
	var input strings.Builder
	for _, line := range strings.Split(want, "\n") {
		input.WriteString(strings.TrimLeft(line, " "))
		input.WriteByte('\n')
	}
	got := indent(strings.TrimSuffix(input.String(), "\n"))
	if got != want {
		t.Fatalf("unexpected indentation:\n%s", got)
	}
	if indent(got) != got {
		t.Fatal("indent is not idempotent")
	}
}

func TestFormatWrappedNestedStatement(t *testing.T) {
	src := "CLASS zcl IMPLEMENTATION.\nMETHOD run.\nIF a = 1.\nLOOP AT tab INTO row.\nresult = call( " + strings.Repeat("argument = value ", 80) + ").\nENDLOOP.\nENDIF.\nENDMETHOD.\nENDCLASS.\n"
	got, err := Format(src)
	if err != nil {
		t.Fatal(err)
	}
	inside := false
	continuations := 0
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 255 {
			t.Fatalf("line has %d bytes", len(line))
		}
		if strings.Contains(line, "result =") {
			inside = true
		}
		if inside {
			if !strings.HasPrefix(line, "        ") {
				t.Fatalf("continuation lost indentation: %q", line)
			}
			continuations++
			if strings.HasSuffix(line, ").") {
				inside = false
			}
		}
	}
	if continuations < 2 {
		t.Fatal("statement was not wrapped")
	}
	again, err := Format(got)
	if err != nil || again != got {
		t.Fatal("Format is not idempotent")
	}
}

func TestIndentStatementBoundaries(t *testing.T) {
	src := "CLASS zcl DEFINITION DEFERRED.\nFORM run.\nGET RUN TIME FIELD reading.\nDATA: a TYPE i, b TYPE i.\nIF a = 1. WRITE `a.b`. ENDIF.\nWRITE |escaped \\| period. template|.\nENDFORM.\n"
	want := "CLASS zcl DEFINITION DEFERRED.\nFORM run.\n  GET RUN TIME FIELD reading.\n  DATA: a TYPE i, b TYPE i.\n  IF a = 1. WRITE `a.b`. ENDIF.\n  WRITE |escaped \\| period. template|.\nENDFORM.\n"
	if got := indent(src); got != want {
		t.Fatalf("got:\n%s", got)
	}
}

// Deep nesting may leave no room for a long literal; the line keeps its
// tokens and drops only the cosmetic indentation.
func TestWrapDropsIndentationForLongLiteral(t *testing.T) {
	literal := "RAISE EXCEPTION TYPE zcx_refusal EXPORTING reason = `" + strings.Repeat("x", 120) + "`."
	got, err := wrap(strings.Repeat(" ", 180) + literal + "\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if len(line) > 255 {
			t.Fatalf("long line %q", line)
		}
	}
	if strings.Join(strings.Fields(got), " ") != strings.Join(strings.Fields(literal), " ") {
		t.Fatalf("tokens changed: %q", got)
	}
	if _, err := wrap("`" + strings.Repeat("x", 300) + "`\n"); err == nil {
		t.Fatal("unbreakable literal accepted")
	}
}
