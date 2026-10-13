package abap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	debtfixtures "github.com/oisee/abapiti/hir/fixtures"
)

func TestRuntimeDebtFixtures(t *testing.T) {
	properties, count := debtfixtures.Properties()
	for _, tc := range []struct {
		name  string
		p     *hir.Program
		count int
	}{{"xml_validation_fixture", debtfixtures.XMLValidation(), 444}, {"property_fixture", properties, count}, {"seq_property_fixture", debtfixtures.PropertiesInSeq(), 1}} {
		t.Run(tc.name, func(t *testing.T) {
			files, names, err := emitABAPNamed(t, tc.p)
			if err != nil {
				t.Fatal(err)
			}
			var c *hir.Class
			for _, candidate := range tc.p.Classes {
				if candidate.Name == tc.name {
					c = candidate
				}
			}
			var declarations, implementations strings.Builder
			for _, m := range c.Methods {
				if m.Name == "run" {
					continue
				}
				fmt.Fprintf(&declarations, "METHODS %s FOR TESTING.\n", m.Name)
				fmt.Fprintf(&implementations, "METHOD %s.\nDATA actual TYPE i.\nactual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = 1 msg = '%s' ).\nENDMETHOD.\n", m.Name, names.Get(c.Name), names.Get("member."+m.Name), m.Name)
			}
			files[names.Get(c.Name)+".clas.testclasses.abap"] = "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\n" + declarations.String() + "ENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\n" + implementations.String() + "ENDCLASS.\n"

			out := t.TempDir()
			if base := fixtureOutputDir(t); base != "" {
				out = filepath.Join(base, "TestRuntimeDebtFixtures")
			}
			if err := os.MkdirAll(out, 0755); err != nil {
				t.Fatal(err)
			}
			for name, src := range files {
				for _, line := range strings.Split(src, "\n") {
					if len(line) > 255 {
						t.Fatalf("%s: long line", name)
					}
				}
				if err := os.WriteFile(filepath.Join(out, name), []byte(src), 0644); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("%d parity cases", tc.count)
		})
	}
}
