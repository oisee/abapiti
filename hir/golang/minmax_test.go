package golang

import (
	"fmt"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestMinMaxExceptionalNumbers(t *testing.T) {
	number := hir.T(hir.Number)
	array := hir.T(hir.Array, number)
	c := &hir.Class{Name: "Extrema"}
	for _, op := range []string{"min", "max"} {
		m := method(op, number, ret(&hir.Expr{Kind: hir.NumericMinMax, Op: op, Type: number, X: hir.V("values", array)}))
		m.Static = true
		m.Params = []hir.Param{{Name: "values", Type: array}}
		c.Methods = append(c.Methods, m)
	}
	main := fmt.Sprintf(`
 check:=func(ok bool){if !ok{panic("exceptional minmax")}}
 min,max:=%s,%s
 inf:=numberMinArray(nil);ninf:=numberMaxArray(nil)
 z:=float64(0);nz:=negativeZero();nan:=z/z
 check(min(&array[float64]{})==inf && max(&array[float64]{})==ninf)
 for _,xs:=range [][]float64{{nan,inf},{ninf,nan},{inf,nan,ninf},{nan,1,2}} {
  a:=&array[float64]{Items:xs}
  check(min(a)!=min(a) && max(a)!=max(a))
 }
 for _,xs:=range [][]float64{{z,nz},{nz,z},{nz,z,nz}} {
  a:=&array[float64]{Items:xs}
  check(1/min(a)==ninf && 1/max(a)==inf)
 }
 check(min(&array[float64]{Items:[]float64{1.5,-2.75,3}})==-2.75)
 fmt.Println("ok")`, entry("Extrema", "min"), entry("Extrema", "max"))
	if got := execute(t, &hir.Program{Classes: []*hir.Class{c}}, main); got != "ok\n" {
		t.Fatal(got)
	}
}
