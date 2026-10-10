package golang

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestStringRepeatJSCounts(t *testing.T) {
	main := `
 check:=func(ok bool){if !ok{panic("repeat semantics")}}
 fault:=func(f func()){defer func(){check(recover()!=nil)}();f()}
 var zero float64; nan:=zero/zero; inf:=1/zero
 check(str("ab").repeat(2.9)==str("abab"))
 check(str("a").repeat(-0.9)==str(""))
 check(str("a").repeat(nan)==str(""))
 check(str("").repeat(1e30)==str(""))
 check(unit(0xd800).repeat(2)==unit(0xd800)+unit(0xd800))
 fault(func(){str("a").repeat(-1)})
 fault(func(){str("a").repeat(inf)})
 fault(func(){str("").repeat(-inf)})
 fault(func(){str("ab").repeat(2147483647)})
 fmt.Println("ok")`
	if got := execute(t, &hir.Program{}, main); got != "ok\n" {
		t.Fatal(got)
	}
}
