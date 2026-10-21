package golang

import (
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Construction shares the compiled program and a /g exec loop shares the
// lastIndex wrapper; before the cache each call re-translated and compiled.
func TestRegExpCacheAllocations(t *testing.T) {
	main := `
 check:=func(ok bool,what string){if !ok {panic(what)}}
 a:=newRegExp(str("\\s+"),str("g"));b:=newRegExp(str("\\s+"),str("g"))
 shared:=a!=b && a.compiled==b.compiled
 a.LastIndex=3;check(b.LastIndex==0,"own lastIndex")
 pattern,flags:=str("\\s+"),str("g")
 construct:=testing.AllocsPerRun(100,func(){newRegExp(pattern,flags)})
 long:=str(strings.Repeat("word ",400))
 loop:=testing.AllocsPerRun(5,func(){r:=newRegExp(pattern,flags);n:=0;for r.exec(long)!=nil {n++};check(n==400,"match count")})
 fault:=func(){defer func(){_,ok:=recover().(trap);check(ok,"trap")}();newRegExp(str("^("),str(""))}
 fault();fault()
 fmt.Println(shared,construct<=4,loop/400<=16,construct,loop/400)`
	got := executeImports(t, &hir.Program{}, []string{"fmt", "strings", "testing"}, main)
	if len(got) < 15 || got[:15] != "true true true " {
		t.Fatal(got)
	}
}
