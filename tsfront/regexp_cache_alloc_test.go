package tsfront

import (
	"strings"
	"testing"
)

// The A16 exec used a cursor-sized counted prefix, so each /g step compiled a
// new wrapper; construction re-ran the iterator translator every time.
func TestGogenA16RegExpCacheAllocations(t *testing.T) {
	source := `class Probe {static run():string {return String(/a/u.test("a"));}}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	probe := `package main
import "testing"
func regexpCacheProbe() (bool, float64, float64) {
 check:=func(ok bool,what string){if !ok {panic(what)}}
 pattern,flags:=str("q[0-9]+z"),str("g")
 a,b:=newRegExp(pattern,flags),newRegExp(pattern,flags)
 shared:=a!=b && a.compiled==b.compiled
 construct:=testing.AllocsPerRun(100,func(){newRegExp(pattern,flags)})
 long:=str(strings.Repeat("q12z ",300)+"😀q3z")
 loop:=testing.AllocsPerRun(3,func(){r:=newRegExp(pattern,flags);n:=0;last:=int32(0);for m:=r.exec(long);m!=nil;m=r.exec(long) {n++;last=m.matchIndex()};check(n==301 && last==1502,"matches")})
 return shared,construct,loop/301
}
`
	got := runGoHIR(t, p, `shared,construct,loop:=regexpCacheProbe();fmt.Println(shared,construct<=4,loop<=12,construct,loop)`, func(files map[string]string) {
		files["regexp_cache_probe.go"] = strings.Replace(probe, `import "testing"`, `import ("strings";"testing")`, 1)
	})
	fields := strings.Fields(got)
	if len(fields) != 5 || fields[0] != "true" || fields[1] != "true" || fields[2] != "true" {
		t.Fatal(got)
	}
	t.Log(got)
}
