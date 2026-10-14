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
	log := files["zcl_lnt_test_log.clas.abap"]
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
