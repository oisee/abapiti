package golang

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestStringRepeatAndMatchRuntime(t *testing.T) {
	main := `
 check:=func(ok bool){if !ok{panic("string semantics")}}
 fault:=func(f func()){defer func(){check(recover()!=nil)}();f()}
 for _,c:=range []struct{s string;n float64;want string}{{"ab",0,""},{"ab",1,"ab"},{"ab",3,"ababab"},{"",9,""},{"x",2.9,"xx"},{"x",-0.9,""},{"😀",2,"😀😀"}} {check(str(c.s).repeat(c.n)==str(c.want))}
 zero:=float64(0)
 check(str("x").repeat(zero/zero)==str(""))
 fault(func(){str("x").repeat(-1)});fault(func(){str("").repeat(1/zero)});fault(func(){str("x").repeat(-1/zero)})
 r:=newRegExp(str("(a)(b)"),str(""));r.LastIndex=99
 check(str("xyz").match(r)==nil)
 m:=str("😀ab").match(r);check(m.MatchIndex==2 && len(m.Items)==3 && m.Items[0]==str("ab") && m.Items[1]==str("a") && m.Items[2]==str("b") && r.LastIndex==99)
 g:=newRegExp(str("&"),str("g"));g.LastIndex=99
 all:=str("&&&").match(g);check(len(all.Items)==3 && all.Items[0]==str("&") && g.LastIndex==0)
 check(str("xyz").match(g)==nil && g.LastIndex==0)
 fmt.Println("ok")`
	if got := execute(t, &hir.Program{}, main); got != "ok\n" {
		t.Fatal(got)
	}
}
