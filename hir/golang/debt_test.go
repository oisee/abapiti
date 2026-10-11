package golang

import (
	"fmt"
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
			main := ""
			for _, c := range tc.p.Classes {
				if c.Name == tc.name {
					for _, m := range c.Methods {
						if m.Name != "run" {
							main += fmt.Sprintf("if %s()!=1 {panic(%q)};", entry(c.Name, m.Name), m.Name)
						}
					}
				}
			}
			main += "fmt.Print(\"ok\")"
			if got := execute(t, tc.p, main); got != "ok" {
				t.Fatal(got)
			}
			t.Logf("%d parity cases", tc.count)
		})
	}
}
