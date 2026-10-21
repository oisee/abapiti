package golang

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestRegExpExecEdges(t *testing.T) {
	main := `
 check:=func(ok bool){if !ok {panic("exec edge")}}
 r:=newRegExp(str("(a)(b)"),str("g"));r.LastIndex=1
 m:=r.exec(str("😀ab"));check(m.matchIndex()==2 && r.LastIndex==4 && m.Items[2]==str("b"))
 check(r.exec(str("😀ab"))==nil && r.LastIndex==0)
 r.LastIndex=-10;check(r.exec(str("ab")).matchIndex()==0 && r.LastIndex==2)
 anchored:=newRegExp(str("^a"),str("g"));anchored.LastIndex=1;check(anchored.exec(str("aa"))==nil && anchored.LastIndex==0)
 sticky:=newRegExp(str("(a)(b)"),str("y"));check(sticky.exec(str("zab"))==nil && sticky.LastIndex==0);sticky.LastIndex=1;check(sticky.exec(str("zab")).matchIndex()==1 && sticky.LastIndex==3)
 empty:=newRegExp(str("a*"),str("g"));check(empty.exec(str("z")).Items[0]==str("") && empty.LastIndex==0);check(empty.exec(str("z")).matchIndex()==0)
 fault:=func(p,f string){defer func(){v:=recover();check(v!=nil);_,ok:=v.(trap);check(ok)}();newRegExp(str(p),str(f))}
 fault("(?=a)a","");fault("^(","");fault("a","u");fault("a","gg")
 faultIndex:=func(n float64){defer func(){v:=recover();check(v!=nil);_,ok:=v.(trap);check(ok)}();r.setLastIndex(n)}
 faultIndex(0.5);faultIndex(2147483648);zero:=float64(0);faultIndex(1/zero);faultIndex(zero/zero)
 name:=newRegExp(str("^(ABC|DEF)$"),str("i"));check(name.test(str("def")))
 fmt.Println("ok")`
	if got := execute(t, &hir.Program{}, main); got != "ok\n" {
		t.Fatal(got)
	}
}
