package golang

import "testing"

func TestLiveClassViewNullishAndPrimitiveDomains(t *testing.T) {
	p := typedStructuralUnionProgram()
	main := `if ` + entry("probe.ts", "view") + `(dynNull())!=nil {panic("null view must be nil")};if ` + entry("probe.ts", "view") + `(nil)!=nil {panic("undefined view must be nil")};for _,d:=range []*dynamic{box(true),box(str("x")),box(float64(42))} {func(){defer func(){if x:=recover();x!=nil {if _,ok:=x.(rangeFault);!ok {panic(x)}} else {panic("primitive admitted")}}();` + entry("probe.ts", "view") + `(d)}()};fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
