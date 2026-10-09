package golang

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestSubsetAndRegexpEdges(t *testing.T) {
	main := `
 check:=func(ok bool){if !ok{panic("semantic mismatch")}}
 fault:=func(f func()){defer func(){check(recover()!=nil)}();f()}
 check(str("ßﬃ").upper()==str("SSFFI"))
 check(str("ΟΣ ΟΣΑ İ").lower()==str("ος οσα i\u0307"))
 lone:=unit(0xd800);check(lone.upper()==lone && lone.lower()==lone)
 section:=str("a😀z").substring(1,3);check(section.charCodeAt(0)==0xd83d && section.charCodeAt(1)==0xde00)
 fault(func(){section.charCodeAt(-1)});fault(func(){section.charCodeAt(2)});fault(func(){str("").charCodeAt(0)})
 check(str("😀x").split(str("")).Items[0]==unit(0xd83d))
 check(str("😀x").replaceAll(unit(0xde00),str("z"))==unit(0xd83d)+str("zx"))
 check(str("\ufeff\u2007 -12x").parseInt10().Value == -12)
 check(!str("x12").parseInt10().Has)
 check(str("9007199254740993").parseInt10i64().Value==9007199254740992)
 d:=parseJSON(str("{\"s\":\"\\ud800\",\"a\":[null,false,1.25],\"x\":1,\"x\":2}"))
 check(d.get(str("s")).Value.(jsString)==lone)
 check(d.get(str("x")).Value.(float64)==2)
 check(d.get(str("a")).get(str("length")).Value.(float64)==3)
 for _,s:=range []string{"", "[1,]", "{\"a\":1,}", "01", "1.", "+1", "NaN", "{} x", "\"\\x41\"", "\"unterminated"} {fault(func(){parseJSON(str(s))})}
 x:=parseXML(str("<?xml version=\"1.0\"?><a><b>x&amp;y</b><b/><c> z </c></a>"))
 check(x.get(str("?xml")).Value.(jsString)==str(""))
 check(x.get(str("a")).get(str("b")).get(str("0")).Value.(jsString)==str("x&y"))
 check(x.get(str("a")).get(str("c")).Value.(jsString)==str(" z "))
 for _,s:=range []string{"<a>", "<a></b>", "<a>&unknown;</a>", "<!DOCTYPE a><a/>", "<a><!--x--></a>", "<constructor/>"} {fault(func(){parseXML(str(s))})}
 r:=newRegExp(str("a.c"),str("i"))
 check(r.test(str("ABC")) && !r.test(str("a\nc")) && !r.test(str("a\rc")) && !r.test(str("a\u2028c")) && !r.test(str("a😀c")))
 check(r.test(str("a")+lone+str("c")))
 check(newRegExp(str("test$"),str("i")).test(str("TEST")))
 check(!newRegExp(str("test$"),str("i")).test(str("teſt")))
 g:=newRegExp(str("x/y"),str("gi"));check(g.toString()==str("/x\\/y/gi"))
 check(g.test(str("X/Y x/y")) && g.LastIndex==3)
 check(g.test(str("X/Y x/y")) && g.LastIndex==7)
 check(!g.test(str("X/Y x/y")) && g.LastIndex==0)
 check(g.match_test(str("x/y")) && g.LastIndex==0)
 check(str("x/y X/Y").replaceRegex(g,str("z"))==str("z z"))
 fault(func(){newRegExp(str("(?=a)a"),str(""))})
 fmt.Println("ok")
 `
	if got := execute(t, &hir.Program{}, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestRejectUnreviewedRegexp(t *testing.T) {
	for _, pattern := range []string{"(?=a)a", "(a)\\1", "a.*c", "\\s", "^Y$", "."} {
		regexp := &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(str, pattern), hir.L(str, "")}}
		m := method("probe", hir.T(hir.RegExp), ret(regexp))
		m.Static = true
		_, err := Emit(&hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}})
		if err == nil || !strings.Contains(err.Error(), "not supported in the Go prototype: JavaScript regexp") {
			t.Fatal(pattern, err)
		}
	}
}

func TestDescriptors(t *testing.T) {
	p := &hir.Program{Classes: []*hir.Class{{Name: "pkg.Base", Fields: []hir.Field{{Name: "staticMember", Type: i32, Static: true}}}, {Name: "pkg.Child", Super: "pkg.Base"}}}
	n := hir.NewNames()
	main := "b:=&" + n.Get("descriptor.pkg.Base") + "; c:=&" + n.Get("descriptor.pkg.Child") + ";v:=c.Factory();"
	main += "if classOf(v)!=c || !descriptorInstance(v,b) || c.has(str(\"staticMember\")) || !b.has(str(\"staticMember\")) {panic(\"descriptor\")};"
	main += "if dynClass(box(c))!=c || dynTypeof(box(c))!=str(\"function\") {panic(\"box\")};fmt.Println(c.Name)"
	if got := execute(t, p, main); got != "Child\n" {
		t.Fatal(got)
	}
}

func TestReferenceCollectionSnapshots(t *testing.T) {
	ref := hir.Ref("Object")
	arr := hir.T(hir.Array, ref)
	mt := hir.T(hir.OrderedMap, ref, ref)
	st := hir.T(hir.OrderedSet, ref)
	a, m, s, o := local("a", arr), local("m", mt), local("s", st), local("o", ref)
	body := hir.B(decl("o", ref, newObj(ref)), decl("a", arr, newObj(arr)), run(rt("array.push", a, i32, o)), decl("m", mt, newObj(mt)), run(rt("map.set", m, mt, o, o)), decl("s", st, newObj(st)), run(rt("set.fromArray", s, st, a)))
	for _, x := range []*hir.Expr{rt("map.keys", m, arr), rt("map.values", m, arr), rt("set.values", s, arr)} {
		body.List = append(body.List, &hir.Stmt{Kind: hir.ForEach, Name: "item", Type: ref, X: x, Body: &hir.Stmt{Kind: hir.If, X: binary("!=", local("item", ref), o, boolean), Body: ret(lit(99))}})
	}
	body.List = append(body.List, ret(lit(1)))
	probe := method("run", i32, body)
	probe.Static = true
	p := &hir.Program{Classes: []*hir.Class{{Name: "Object"}, {Name: "Probe", Methods: []*hir.Method{probe}}}}
	if got := execute(t, p, "fmt.Println("+entry("Probe", "run")+"())"); got != "1\n" {
		t.Fatal(got)
	}
}

// Compare boundary arithmetic to the former arbitrary-precision implementation.
func TestIntegerArithmeticBoundaries(t *testing.T) {
	values := []int64{math.MinInt64, math.MinInt64 + 1, math.MaxInt64, math.MaxInt64 - 1, math.MinInt32, math.MaxInt32, -9007199254740992, -9007199254740991, 9007199254740991, 9007199254740992, -3, -1, 0, 1, 3}
	var main strings.Builder
	main.WriteString(`check:=func(a,b int64,op string,checked bool,bits int,want int64,fault bool){defer func(){if r:=recover();r!=nil{if _,ok:=r.(rangeFault);!ok||!fault{panic("unexpected fault")}}else if fault{panic("missing fault")}}();got:=integerArithmetic(a,b,op,checked,bits)
 if bits==64 && (op=="+"||op=="-") {if op=="+"{got=checkedAddI64(a,b)}else{got=checkedSubI64(a,b)};if checked{got=safeInteger(got)}}
 if got!=want{panic("wrong result")}};`)
	for _, a := range values {
		for _, b := range values {
			for _, op := range []string{"+", "-", "*", "/", "%"} {
				x, y := big.NewInt(a), big.NewInt(b)
				zero := (op == "/" || op == "%") && b == 0
				if !zero {
					switch op {
					case "+":
						x.Add(x, y)
					case "-":
						x.Sub(x, y)
					case "*":
						x.Mul(x, y)
					case "/":
						x.Quo(x, y)
					case "%":
						x.Rem(x, y)
					}
				}
				for _, bits := range []int{32, 64} {
					for _, checked := range []bool{false, true} {
						fault := zero || !x.IsInt64()
						v := x.Int64()
						fault = fault || bits == 32 && (v < math.MinInt32 || v > math.MaxInt32) || checked && (v < -9007199254740991 || v > 9007199254740991)
						fmt.Fprintf(&main, "check(%d,%d,%q,%t,%d,%d,%t);", a, b, op, checked, bits, v, fault)
					}
				}
			}
		}
	}
	main.WriteString(`fmt.Println("ok")`)
	if got := execute(t, &hir.Program{}, main.String()); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestIndexedCollectionEdges(t *testing.T) {
	p := &hir.Program{Classes: []*hir.Class{{Name: "Base"}, {Name: "Child", Super: "Base"}}}
	n := hir.NewNames()
	main := fmt.Sprintf(`
 check:=func(ok bool){if !ok{panic("collection mismatch")}}
 var child *%s
 var ref %s=child
 m:=&orderedMap[any,int32]{}
 m.set(ref,1);m.set(nil,2);check(len(m.Entries)==1&&m.get(ref).Value==2&&m.get(nil).Value==2)
 a,b:=&%s{},&%s{}
 m.set(a,3);m.set(b,4);keys:=m.keys();check(m.delete(a)&&!m.has(a)&&m.get(b).Value==4)
 m.set(a,5);check(m.Entries[1].Key==b&&m.Entries[2].Key==a&&keys.Items[1]==a)
 s:=&orderedSet[any]{};s.add(ref).add(nil).add(a).add(b);snap:=s.values();check(len(s.Items)==3)
 check(s.delete(a)&&s.has(b));s.add(a);check(s.Items[1]==b&&s.Items[2]==a&&snap.Items[1]==a)
 nums:=&orderedSet[float64]{};nums.add(negativeZero()).add(0);check(len(nums.Items)==1&&nums.has(0))
 nan:=negativeZero()/negativeZero();nums.add(nan).add(nan);check(len(nums.Items)==3&&!nums.has(nan)&&!nums.delete(nan))
 nums.add(63).add(64).add(127).add(128).add(255).add(256).add(-1).add(0.5)
 for _,v:=range []float64{0,63,64,127,128,255,256,-1,0.5}{check(nums.has(v))}
 check(!nums.has(254)&&nums.delete(64)&&!nums.has(64));nums.add(64);check(nums.has(64))
 check(nums.delete(0)&&!nums.has(negativeZero()));nums.add(negativeZero());check(nums.has(0))
 opts:=&orderedSet[optional[int32]]{};opts.add(optional[int32]{}).add(present(int32(0)));check(len(opts.Items)==2)
 fmt.Println("ok")`, n.Get("struct.Child"), n.Get("ref.Base"), n.Get("struct.Child"), n.Get("struct.Child"))
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestCheckedInt32ConversionEdges(t *testing.T) {
	number := hir.T(hir.Number)
	convert := func(name string, min, max int64) *hir.Method {
		m := method(name, i32, ret(&hir.Expr{Kind: hir.CheckedNumericConvert, Type: i32, X: local("value", number), Range: &hir.IntegerRange{Min: min, Max: max}}))
		m.Static = true
		m.Params = []hir.Param{{Name: "value", Type: number}}
		return m
	}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Conversion", Methods: []*hir.Method{convert("full", math.MinInt32, math.MaxInt32), convert("limited", 0, 10)}}}}
	main := `check:=func(ok bool){if !ok{panic("conversion mismatch")}};fault:=func(f func()){defer func(){if _,ok:=recover().(rangeFault);!ok{panic("wrong fault")}}();f();panic("missing fault")};`
	full, limited := entry("Conversion", "full"), entry("Conversion", "limited")
	for _, v := range []int64{math.MinInt32, -1, 0, 1, math.MaxInt32} {
		main += fmt.Sprintf("check(%s(%d)==%d);", full, v, v)
	}
	for _, v := range []string{"-2147483649", "2147483648", "4294967296", "9007199254740991", "0.5", "-0.5", "negativeZero()/negativeZero()", "1/negativeZero()", "-1/negativeZero()"} {
		main += fmt.Sprintf("fault(func(){%s(%s)});", full, v)
	}
	main += fmt.Sprintf(`check(%s(negativeZero())==0);check(%s(0)==0);check(%s(10)==10);fault(func(){%s(-1)});fault(func(){%s(11)});fmt.Println("ok")`, full, limited, limited, limited, limited)
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestImmutableCharacterMembershipEdges(t *testing.T) {
	number := hir.T(hir.Number)
	set := hir.T(hir.OrderedSet, number)
	field := &hir.Expr{Kind: hir.StaticGet, Owner: "Characters", Name: "members", Type: set}
	builder := local("builder", set)
	init := hir.B(decl("builder", set, newObj(set)))
	for _, v := range []float64{-1, 0, 127, 255} {
		init.List = append(init.List, run(rt("set.add", builder, set, hir.L(number, v))))
	}
	init.List = append(init.List, &hir.Stmt{Kind: hir.Assign, X: field, Y: builder})
	ctor := method("class_constructor", hir.T(hir.Void), init)
	ctor.Static = true
	has := method("has", boolean, ret(rt("set.has", field, boolean, local("value", number))))
	has.Static = true
	has.Params = []hir.Param{{Name: "value", Type: number}}
	c := &hir.Class{Name: "Characters", Fields: []hir.Field{{Name: "members", Type: set, Static: true, Readonly: true}}, Methods: []*hir.Method{ctor, has}}
	p := &hir.Program{Classes: []*hir.Class{c}}
	files, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(files["hir.go"], "[256]bool") {
		t.Fatal("membership-only set was not classified")
	}
	main := fmt.Sprintf(`check:=func(ok bool){if !ok {panic("classification")}};has:=%s;zero:=float64(0);check(has(-1)&&has(0)&&has(-zero)&&has(127)&&has(255));check(!has(-2)&&!has(256)&&!has(65535)&&!has(.5)&&!has(-.5)&&!has(0/zero)&&!has(1/zero));fmt.Println("ok")`, entry("Characters", "has"))
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
	// A readonly binding can still expose a mutable set. Either mutation or
	// returning an alias must prevent classification.
	mutate := method("mutate", hir.T(hir.Void), hir.B(run(rt("set.add", field, set, hir.L(number, float64(9))))))
	mutate.Static = true
	alias := method("alias", set, ret(field))
	alias.Static = true
	for _, extra := range []*hir.Method{mutate, alias} {
		c.Methods = append([]*hir.Method{ctor, has}, extra)
		files, err = Emit(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(files["hir.go"], "[256]bool") {
			t.Fatal("mutable or escaping set was classified")
		}
	}
}
