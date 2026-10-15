package tsfront

import (
	"strings"
	"testing"
)

func TestA4HTestSupport(t *testing.T) {
	files, err := A4HTestSupport()
	if err != nil {
		t.Fatal(err)
	}
	for file, source := range files {
		if !strings.HasSuffix(file, ".abap") {
			continue
		}
		for i, line := range strings.Split(source, "\n") {
			if len(line) > 255 {
				t.Fatalf("%s:%d overlong", file, i+1)
			}
		}
	}
	for file, source := range files {
		for _, forbidden := range []string{"bal_", "balloghndl", "bal_s_log", "bal_t_logh", "symsgty"} {
			if strings.Contains(strings.ToLower(source), forbidden) {
				t.Errorf("%s contains SAP BAL dependency %s", file, forbidden)
			}
		}
	}
	if _, exists := files["zcl_lnt_test_log_bal.clas.abap"]; exists {
		t.Fatal("BAL adapter in test support")
	}
	log := files["zcl_lnt_test_log.clas.abap"]
	for _, method := range []string{"BEGIN", "EVENT"} {
		if !strings.Contains(log, "CALL METHOD ('ZCL_LNT_TEST_LOG_BAL')=>('"+method+"')") {
			t.Errorf("missing optional %s hook", method)
		}
	}
	if strings.Count(log, "CATCH cx_sy_dyn_call_error.") != 2 {
		t.Fatal("optional hooks must tolerate an absent adapter")
	}
	for _, required := range []string{"COMMIT WORK AND WAIT", "row-used = count", "next = CONV i( index_text ) + 1", "row-shard <> selected"} {
		if !strings.Contains(log, required) {
			t.Fatal("missing durable log contract:", required)
		}
	}
	report := files["zlnt_tests_run.prog.abap"]
	if !strings.Contains(report, "p_start") || !strings.Contains(report, "p_shard") || !strings.Contains(report, "zcl_lnt_test_runner=>run") {
		t.Fatal(report)
	}
}

func TestA4HTestLogBAL(t *testing.T) {
	source, err := A4HTestLogBAL()
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"CLASS-METHODS begin", "CLASS-METHODS event", "BAL_LOG_CREATE", "BAL_LOG_MSG_ADD_FREE_TEXT", "BAL_DB_SAVE", "WHEN 'passed'.", "WHEN 'failed'.", "WHEN 'trapped'.", "WHEN 'not-run'."} {
		if !strings.Contains(source, required) {
			t.Errorf("missing BAL behavior %s", required)
		}
	}
	if strings.Contains(strings.ToLower(source), "zlnt_testrun") {
		t.Fatal("BAL adapter must not write checkpoint rows")
	}
	for i, line := range strings.Split(source, "\n") {
		if len(line) > 255 {
			t.Fatalf("BAL adapter:%d overlong", i+1)
		}
	}
}
