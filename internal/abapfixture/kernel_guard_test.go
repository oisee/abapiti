package abapfixture

import (
	"strings"
	"testing"
)

func TestKernelViolations(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"METHOD constructor. super->constructor( ). IF source IS BOUND. RETURN. ENDIF. ENDMETHOD.", ""},
		{"METHOD constructor. IF x = 1. super->\nconstructor( ). ENDIF. ENDMETHOD.", "inside if"},
		{"METHOD constructor. CASE x. WHEN 1. super->constructor( ). ENDCASE. ENDMETHOD.", "inside case"},
		{"METHOD constructor. TRY. super->constructor( ). CATCH cx_root. ENDTRY. ENDMETHOD.", "inside try"},
		{"METHOD constructor. LOOP AT xs INTO x. super->constructor( ). ENDLOOP. ENDMETHOD.", "inside loop"},
		{"METHOD constructor. RETURN. super->constructor( ). ENDMETHOD.", "RETURN before"},
		{"METHOD constructor. super->constructor( ). super->constructor( ). ENDMETHOD.", "calls super 2"},
		{"METHOD run. CASE key. WHEN OTHERS. ENDCASE. ENDMETHOD.", ""},
		{"METHOD constructor. super->constructor( ). x = `IF. CASE. super->constructor( ).`. \" IF.\n ENDMETHOD.", ""},
	} {
		got := strings.Join(KernelViolations(map[string]string{"probe.clas.abap": tc.source}), "\n")
		if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q, want %q", tc.source, got, tc.want)
		}
	}
}
