package golang

import (
	"regexp"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// arrayExtraProgram enables every optional array runtime: live views (parsed
// XML), sparse holes and regexp match groups, so arrayExtra has all fields.
func arrayExtraProgram() *hir.Program {
	strs := hir.T(hir.Array, str)
	parse := &hir.Method{Name: "parse", Static: true, Result: hir.T(hir.Dynamic), Params: []hir.Param{{Name: "text", Type: str}},
		Body: hir.B(ret(rt("xml.parseSubset", hir.V("text", str), hir.T(hir.Dynamic))))}
	groups := &hir.Method{Name: "groups", Static: true, Result: hir.T(hir.Dynamic), Params: []hir.Param{{Name: "m", Type: strs}},
		Body: hir.B(ret(rt("array.matchGroups", hir.V("m", strs), hir.T(hir.Dynamic))))}
	dyn := hir.T(hir.Dynamic)
	values := &hir.Method{Name: "values", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "d", Type: dyn}},
		Body: hir.B(run(rt("dynamic.values", hir.V("d", dyn), hir.T(hir.Array, dyn))))}
	return &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{parse, groups, values}}}}
}

// The header of every array is Items plus one pointer; views, holes and exec
// metadata must not widen it, even with all optional runtimes enabled.
func TestArrayHeaderSize(t *testing.T) {
	files, err := Emit(arrayExtraProgram())
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`live\s+liveArrayStorage\n`, `holes\s+map\[int\]bool\n`, `MatchGroups\s+\*dynamic\n`, `Missing\s+map\[int32\]bool\n`} {
		if !regexp.MustCompile(`\n\t` + field).MatchString(files["runtime.go"]) {
			t.Fatalf("full runtime lacks array field %q", field)
		}
	}
	main := `fmt.Println(unsafe.Sizeof(array[any]{}),unsafe.Sizeof(array[jsString]{}))`
	for name, p := range map[string]*hir.Program{"plain": {}, "all": arrayExtraProgram()} {
		if got := executeImports(t, p, []string{"fmt", "unsafe"}, main); got != "32 32\n" {
			t.Fatalf("%s: array header sizes %q, want 32 bytes", name, got)
		}
	}
}

func TestArrayExtraDenseStaysLean(t *testing.T) {
	main := `check:=func(ok bool,what string){if !ok {panic(what)}}
 a:=&array[any]{};a.push(box(int32(1)));a.put(0,box(int32(2)));a.reserve(8)
 check(a.extra==nil && a.missing()==nil && !a.get(1).Has && a.get(0).Has,"dense")
 r:=newRegExp(str("(a)(b)"),str(""));m:=r.exec(str("zab"))
 check(m.matchIndex()==1 && len(m.Items)==3 && m.missing()==nil && m.get(2).Value==str("b"),"exec")
 x:=matchArray(5);x.Items=[]jsString{str("a"),str("")};x.extra.Missing=map[int32]bool{1:true}
 check(x.matchIndex()==5 && !x.get(1).Has && x.get(0).Has,"missing")
 x.put(1,str("y"));check(x.get(1).Has && !x.missing()[1],"put clears missing")
 func(){defer func(){_,ok:=recover().(trap);check(ok,"matchIndex trap")}();(&array[jsString]{}).matchIndex()}()
 fmt.Println("ok")`
	if got := execute(t, &hir.Program{}, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestArrayExtraViewsHolesGroups(t *testing.T) {
	main := `check:=func(ok bool,what string){if !ok {panic(what)}}
 plain:=&array[any]{Items:[]any{box(int32(1))}}
 check(plain.extra==nil && !plain.isLive() && plain.dynamicSource()==nil && plain.holeSet()==nil && plain.matchGroups()==nil,"nil extra reads")
 check(plain.slice0().extra==nil && plain.reverse().extra==nil && plain.concat(plain).extra==nil,"dense ops stay lean")
 d:=parseXML(str("<root><x>a</x><x>b</x></root>")).get(str("root")).get(str("x"))
 v:=liveArrayView[jsString](d,liveStringElement)
 check(v.isLive() && v.dynamicSource()==d && v.length()==2 && v.get(1).Value==str("b"),"view")
 v.put(0,str("c"));v.push(str("e"))
 check(dynString(d.get(str("0")))==str("c") && dynNumber(d.get(str("length")))==3 && v.get(2).Value==str("e"),"view writes")
 check(v.pop().Value==str("e") && v.length()==2 && len(v.values())==2,"view pop")
 ro:=liveReadonlyArrayView[jsString](d,liveStringElement);check(ro.isLive() && ro.get(0).Value==str("c"),"readonly view")
 s:=sizedArray[int32](3);s.put(1,7)
 check(!s.has(0) && s.has(1) && !s.has(2) && !s.get(2).Has,"sized")
 q:=&array[int32]{Items:[]int32{1,2,3}};q.delete(1)
 check(q.extra!=nil && !q.has(1) && q.has(2),"delete")
 c:=q.slice0();check(!c.has(1) && c.has(0),"slice copies holes")
 q.reverse();check(!q.has(1) && q.get(0).Value==3,"reverse")
 q.put(5,9);check(!q.has(3) && !q.has(4) && q.has(5) && len(q.Items)==6,"put past end")
 q.shift();check(!q.has(0) && !q.has(2) && !q.has(3) && q.has(4),"shift")
 cat:=(&array[int32]{Items:[]int32{0}}).concat(q);check(!cat.has(1) && cat.has(5),"concat")
 sp:=cat.splice2(0,2);check(sp.has(0) && !sp.has(1) && !cat.has(1) && cat.has(3),"splice")
 cat.unshift(4);check(cat.has(0) && !cat.has(2),"unshift")
 g:=&array[jsString]{Items:[]jsString{str("a")}};g.ext().MatchGroups=box(str("x"))
 check(g.matchGroups()!=nil && g.missing()==nil,"groups")
 fmt.Println("ok")`
	if got := execute(t, arrayExtraProgram(), main); got != "ok\n" {
		t.Fatal(got)
	}
}
