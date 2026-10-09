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
	main.WriteString(`check:=func(a,b int64,op string,checked bool,bits int,want int64,fault bool){defer func(){if r:=recover();r!=nil{if _,ok:=r.(rangeFault);!ok||!fault{panic("unexpected fault")}}else if fault{panic("missing fault")}}();if integerArithmetic(a,b,op,checked,bits)!=want{panic("wrong result")}};`)
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
