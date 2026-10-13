package tsfront

import (
	"strings"
	"testing"
)

func TestRegistryRunTestAssertsParity(t *testing.T) {
	for _, trap := range []string{"", "zcx_coverage"} {
		text := RegistryRunTest("zcl_abapiti_registry_run", trap)
		if !strings.Contains(text, "assert_equals( act = ok exp = abap_true msg = report )") || strings.Contains(text, "=>fail(") {
			t.Fatal("registry driver must assert parity, not fail unconditionally", text)
		}
		statements := strings.Join(strings.Fields(text), " ")
		if !strings.Contains(statements, "ENDTRY. WRITE / report. cl_abap_unit_assert=>assert_equals(") {
			t.Fatal("registry timing report must print on success before asserting parity", text)
		}
		if !strings.Contains(statements, "CATCH cx_root INTO error. CLEAR ok.") {
			t.Fatal("caught exceptions must not retain a successful verdict", text)
		}
		if trap != "" && !strings.Contains(statements, "CATCH "+trap+" INTO DATA(trapped). CLEAR ok.") {
			t.Fatal("coverage refusals must not retain a successful verdict", text)
		}
	}
}
