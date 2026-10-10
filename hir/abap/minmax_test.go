package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestMinMaxNumberAndEmptyTraps(t *testing.T) {
	number := hir.T(hir.Number)
	arr := hir.T(hir.Array, number)
	c := &hir.Class{Name: "Extrema"}
	for _, op := range []string{"min", "max"} {
		m := method(op, number, hir.B(ret(&hir.Expr{Kind: hir.NumericMinMax, Op: op, Type: number, X: hir.V("values", arr)})))
		m.Static = true
		m.Params = []hir.Param{{Name: "values", Type: arr}}
		c.Methods = append(c.Methods, m)
		pair := method(op+"Pair", number, hir.B(ret(&hir.Expr{Kind: hir.NumericMinMax, Op: op, Type: number, X: hir.V("a", number), Y: hir.V("b", number)})))
		pair.Static = true
		pair.Params = []hir.Param{{Name: "a", Type: number}, {Name: "b", Type: number}}
		c.Methods = append(c.Methods, pair)
	}
	files, err := Emit(&hir.Program{Classes: []*hir.Class{c}})
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, s := range files {
		text.WriteString(s)
	}
	for _, want := range []string{"empty input yields Infinity", "RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.", "signed-zero tie", " FROM 2.", "READ TABLE"} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("missing %q", want)
		}
	}
}
