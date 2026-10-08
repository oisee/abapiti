package abap

import (
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

var i32 = hir.T(hir.I32)
var boolean = hir.T(hir.Bool)
var str = hir.T(hir.String)

func lit(n int) *hir.Expr       { return hir.L(i32, n) }
func ret(x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Return, X: x} }
func decl(n string, t hir.Type, x *hir.Expr) *hir.Stmt {
	return &hir.Stmt{Kind: hir.VarDecl, Name: n, Type: t, X: x}
}
func local(n string, t hir.Type) *hir.Expr { return hir.V(n, t) }
func binary(op string, a, b *hir.Expr, t hir.Type) *hir.Expr {
	return &hir.Expr{Kind: hir.Binary, Op: op, X: a, Y: b, Type: t}
}
func run(x *hir.Expr) *hir.Stmt   { return &hir.Stmt{Kind: hir.ExprStmt, X: x} }
func newObj(t hir.Type) *hir.Expr { return &hir.Expr{Kind: hir.New, Type: t} }
func rt(op string, x *hir.Expr, t hir.Type, args ...*hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.RuntimeOp, Op: op, X: x, Type: t, Args: args}
}
func method(n string, result hir.Type, body *hir.Stmt) *hir.Method {
	return &hir.Method{Name: n, Result: result, Body: body}
}
func call(kind hir.ExprKind, x *hir.Expr, owner, n string, t hir.Type, args ...*hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: kind, X: x, Owner: owner, Name: n, Type: t, Args: args}
}
func assign(n string, t hir.Type, x *hir.Expr) *hir.Stmt {
	return &hir.Stmt{Kind: hir.Assign, X: local(n, t), Y: x}
}

type fixture struct {
	name string
	p    *hir.Program
	want int
}

func fixtures() []fixture {
	var fs []fixture
	add := func(n string, p *hir.Program, b *hir.Stmt, want int) {
		m := method("run", i32, b)
		m.Static = true
		p.Classes = append(p.Classes, &hir.Class{Name: n, Methods: []*hir.Method{m}})
		fs = append(fs, fixture{n, p, want})
	}
	base := method("value", i32, ret(lit(3)))
	base.Virtual = true
	child := method("value", i32, ret(binary("+", call(hir.SuperCall, nil, "", "value", i32), lit(4), i32)))
	child.Virtual = true
	ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("Derived")}, Name: "n", Type: i32}, Y: local("p", i32)}, assign("p", i32, lit(99))))
	ctor.Params = []hir.Param{{Name: "p", Type: i32}}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Base", Methods: []*hir.Method{base}}, {Name: "Derived", Super: "Base", Fields: []hir.Field{{Name: "n", Type: i32}}, Ctor: ctor, Methods: []*hir.Method{child}}}}
	obj := newObj(hir.Ref("Derived"))
	obj.Args = []*hir.Expr{local("arg", i32)}
	add("virtual_fixture", p, hir.B(decl("arg", i32, lit(9)), decl("d", hir.Ref("Derived"), obj), decl("x", hir.Ref("Base"), local("d", hir.Ref("Derived"))), &hir.Stmt{Kind: hir.If, X: binary("==", local("arg", i32), lit(9), boolean), Body: &hir.Stmt{Kind: hir.If, X: binary("==", &hir.Expr{Kind: hir.FieldGet, Type: i32, X: local("d", hir.Ref("Derived")), Name: "n"}, lit(9), boolean), Body: ret(call(hir.VirtualCall, local("x", hir.Ref("Base")), "", "value", i32))}}, ret(lit(99))), 7)
	iface := &hir.Interface{Name: "Readable", Methods: []*hir.Method{{Name: "read", Result: i32}}}
	abs := &hir.Class{Name: "AbstractReader", Abstract: true, Implements: []string{"Readable"}, Methods: []*hir.Method{{Name: "read", Result: i32, Abstract: true, Virtual: true}}}
	read := method("read", i32, ret(lit(11)))
	read.Virtual = true
	ir := hir.Type{Kind: hir.InterfaceRef, Name: "Readable"}
	add("interface_fixture", &hir.Program{Interfaces: []*hir.Interface{iface}, Classes: []*hir.Class{abs, {Name: "Reader", Super: abs.Name, Methods: []*hir.Method{read}}}}, hir.B(decl("r", ir, newObj(hir.Ref("Reader"))), ret(call(hir.VirtualCall, local("r", ir), "", "read", i32))), 11)
	inst := func(owner string) *hir.Expr {
		return &hir.Expr{Kind: hir.InstanceOf, X: local("x", hir.Ref("Root")), Owner: owner, Type: boolean}
	}
	rootRef := hir.Ref("Root")
	add("instance_fixture", &hir.Program{Classes: []*hir.Class{{Name: "Root"}, {Name: "Middle", Super: "Root"}, {Name: "Leaf", Super: "Middle"}, {Name: "Sibling", Super: "Root"}}}, hir.B(decl("x", rootRef, nil), decl("n", i32, lit(0)), &hir.Stmt{Kind: hir.If, X: inst("Root"), Body: assign("n", i32, lit(99))}, assign("x", rootRef, newObj(hir.Ref("Leaf"))), &hir.Stmt{Kind: hir.If, X: inst("Root"), Body: assign("n", i32, lit(1))}, &hir.Stmt{Kind: hir.If, X: inst("Middle"), Body: assign("n", i32, binary("+", local("n", i32), lit(1), i32))}, &hir.Stmt{Kind: hir.If, X: inst("Sibling"), Body: assign("n", i32, lit(99))}, ret(local("n", i32))), 2)
	opt := hir.T(hir.Optional, i32)
	oref := hir.T(hir.Optional, hir.Ref("OptionalObject"))
	test := func(k hir.ExprKind, x *hir.Expr) *hir.Expr { return &hir.Expr{Kind: k, X: x, Type: boolean} }
	add("optional_fixture", &hir.Program{Classes: []*hir.Class{{Name: "OptionalObject"}}}, hir.B(decl("missing", opt, hir.L(opt, nil)), decl("zero", opt, hir.L(opt, 0)), decl("present", opt, lit(5)), decl("ref", oref, hir.L(oref, nil)), decl("n", i32, lit(0)), &hir.Stmt{Kind: hir.If, X: test(hir.IsUndefined, local("missing", opt)), Body: assign("n", i32, lit(1))}, &hir.Stmt{Kind: hir.If, X: test(hir.IsUndefined, local("zero", opt)), Body: assign("n", i32, lit(99))}, &hir.Stmt{Kind: hir.If, X: test(hir.ToBoolean, local("zero", opt)), Body: assign("n", i32, lit(99))}, &hir.Stmt{Kind: hir.If, X: test(hir.ToBoolean, local("present", opt)), Body: assign("n", i32, binary("+", local("n", i32), lit(1), i32))}, &hir.Stmt{Kind: hir.If, X: test(hir.IsUndefined, local("ref", oref)), Body: assign("n", i32, binary("+", local("n", i32), lit(1), i32))}, assign("ref", oref, newObj(hir.Ref("OptionalObject"))), &hir.Stmt{Kind: hir.If, X: test(hir.ToBoolean, local("ref", oref)), Body: assign("n", i32, binary("+", local("n", i32), lit(1), i32))}, ret(local("n", i32))), 4)
	mt, st := hir.T(hir.OrderedMap, i32, str), hir.T(hir.OrderedSet, i32)
	ml, sl := local("m", mt), local("s", st)
	loop := func(x *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.ForEach, Name: "k", Type: i32, X: x, Body: assign("n", i32, binary("+", binary("*", local("n", i32), lit(10), i32), local("k", i32), i32))}
	}
	add("collection_fixture", &hir.Program{}, hir.B(decl("m", mt, newObj(mt)), decl("s", st, newObj(st)), run(rt("map.set", ml, mt, lit(1), hir.L(str, "a"))), run(rt("map.set", ml, mt, lit(2), hir.L(str, "b"))), run(rt("map.set", ml, mt, lit(1), hir.L(str, "updated"))), run(rt("set.add", sl, st, lit(3))), run(rt("set.add", sl, st, lit(4))), run(rt("set.add", sl, st, lit(3))), decl("n", i32, lit(0)), loop(rt("map.keys", ml, hir.T(hir.Array, i32))), loop(rt("set.values", sl, hir.T(hir.Array, i32))), ret(local("n", i32))), 1234)
	boom := method("boom", i32, &hir.Stmt{Kind: hir.Throw, X: hir.L(str, "payload")})
	boom.Static = true
	add("exception_fixture", &hir.Program{Classes: []*hir.Class{{Name: "Thrower", Methods: []*hir.Method{boom}}}}, hir.B(&hir.Stmt{Kind: hir.Try, Name: "caught", Type: str, Body: ret(call(hir.DirectCall, nil, "Thrower", "boom", i32)), Else: hir.B(&hir.Stmt{Kind: hir.If, X: binary("==", local("caught", str), hir.L(str, "payload"), boolean), Body: ret(lit(7))}, ret(lit(99)))}), 7)

	c := fs[4].p.Classes[0]
	check := func(x *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", X: x, Type: boolean}, Body: ret(lit(99))}
	}
	eq := func(a, b *hir.Expr) *hir.Expr { return binary("==", a, b, boolean) }
	// Exercise independent boxes, absent and present operands, and both equality operators.
	fs[3].p.Classes[1].Methods[0].Body.List = append([]*hir.Stmt{
		check(eq(hir.L(opt, 0), hir.L(opt, 0))),
		check(eq(hir.L(opt, nil), hir.L(opt, nil))),
		check(binary("!=", hir.L(opt, nil), hir.L(opt, 0), boolean)),
		check(binary("!=", hir.L(opt, 0), hir.L(opt, nil), boolean)),
		check(binary("!=", hir.L(opt, 1), hir.L(opt, 2), boolean)),
		check(&hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: binary("!=", hir.L(opt, 0), hir.L(opt, 0), boolean)}),
		check(eq(hir.L(hir.T(hir.Optional, str), "same"), hir.L(hir.T(hir.Optional, str), "same"))),
		check(eq(hir.L(hir.T(hir.Optional, boolean), false), hir.L(hir.T(hir.Optional, boolean), false))),
		check(eq(hir.L(hir.T(hir.Optional, hir.T(hir.Number)), 2.5), hir.L(hir.T(hir.Optional, hir.T(hir.Number)), 2.5))),
	}, fs[3].p.Classes[1].Methods[0].Body.List...)
	arr := hir.T(hir.Array, i32)
	al := local("a", arr)
	sm := hir.T(hir.OrderedMap, str, i32)
	sml := local("sm", sm)
	om := hir.T(hir.OrderedMap, hir.Ref(c.Name), hir.T(hir.I64))
	oml := local("om", om)
	bad := method("explode", boolean, &hir.Stmt{Kind: hir.Throw, X: hir.L(str, "unexpected")})
	bad.Static = true
	boomCall := func() *hir.Expr { return call(hir.DirectCall, nil, c.Name, "explode", boolean) }
	extra := method("extra", i32, hir.B(
		decl("a", arr, newObj(arr)), run(rt("array.push", al, i32, lit(0))), run(rt("array.push", al, i32, lit(3))),
		check(eq(rt("array.length", al, i32), lit(2))), check(test(hir.IsUndefined, rt("array.get", al, opt, lit(8)))),
		check(&hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: test(hir.IsUndefined, rt("array.get", al, opt, lit(0)))}),
		&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.IndexGet, Type: i32, X: al, Y: lit(0)}, Y: lit(6)},
		check(eq(&hir.Expr{Kind: hir.IndexGet, Type: i32, X: al, Y: lit(0)}, lit(6))),
		decl("alias", arr, al), run(rt("array.push", local("alias", arr), i32, lit(7))), check(eq(rt("array.length", al, i32), lit(3))),
		decl("sm", sm, newObj(sm)), run(rt("map.set", sml, sm, hir.L(str, "key"), lit(0))), check(rt("map.has", sml, boolean, hir.L(str, "key"))), check(eq(rt("map.size", sml, i32), lit(1))),
		check(&hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: test(hir.ToBoolean, rt("map.get", sml, opt, hir.L(str, "key")))}),
		check(eq(rt("map.get", sml, opt, hir.L(str, "key")), hir.L(opt, 0))),
		check(test(hir.IsUndefined, rt("map.get", sml, opt, hir.L(str, "missing")))),
		run(rt("map.set", sml, sm, hir.L(str, "other"), lit(7))),
		check(eq(rt("map.get", sml, opt, hir.L(str, "other")), hir.L(opt, 7))),
		run(rt("map.set", sml, sm, hir.L(str, "key"), lit(9))),
		check(eq(rt("map.get", sml, opt, hir.L(str, "key")), hir.L(opt, 9))),
		check(eq(rt("map.size", sml, i32), lit(2))),
		decl("om", om, newObj(om)), decl("key", hir.Ref(c.Name), newObj(hir.Ref(c.Name))),
		run(rt("map.set", oml, om, local("key", hir.Ref(c.Name)), hir.L(hir.T(hir.I64), int64(9007199254740993)))),
		check(rt("map.has", oml, boolean, local("key", hir.Ref(c.Name)))),
		check(eq(rt("map.get", oml, hir.T(hir.Optional, hir.T(hir.I64)), local("key", hir.Ref(c.Name))), hir.L(hir.T(hir.Optional, hir.T(hir.I64)), int64(9007199254740993)))),
		check(&hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: rt("map.has", oml, boolean, newObj(hir.Ref(c.Name)))}),
		check(eq(binary("/", lit(-5), lit(2), i32), lit(-2))),
		check(eq(binary("%", lit(-5), lit(-2), i32), lit(-1))),
		check(eq(binary("/", hir.L(hir.T(hir.I64), int64(9223372036854775807)), hir.L(hir.T(hir.I64), int64(3)), hir.T(hir.I64)), hir.L(hir.T(hir.I64), int64(3074457345618258602)))),
		check(eq(binary("+", hir.L(hir.T(hir.I64), int64(math.MinInt64)), hir.L(hir.T(hir.I64), int64(math.MaxInt64)), hir.T(hir.I64)), hir.L(hir.T(hir.I64), int64(-1)))),
		check(eq(binary("-", hir.L(hir.T(hir.I64), int64(9007199254740993)), hir.L(hir.T(hir.I64), int64(9007199254740992)), hir.T(hir.I64)), hir.L(hir.T(hir.I64), int64(1)))),
		check(eq(binary("+", hir.L(hir.T(hir.I64), int64(math.MinInt32)-1), hir.L(hir.T(hir.I64), int64(math.MaxInt32)+1), hir.T(hir.I64)), hir.L(hir.T(hir.I64), int64(-1)))),
		check(eq(binary("+", hir.L(hir.T(hir.I64), int64(math.MinInt32)), hir.L(hir.T(hir.I64), int64(math.MaxInt32)), hir.T(hir.I64)), hir.L(hir.T(hir.I64), int64(-1)))),
		check(eq(binary("/", hir.L(hir.T(hir.Number), 5.0), hir.L(hir.T(hir.Number), 2.0), hir.T(hir.Number)), hir.L(hir.T(hir.Number), 2.5))),
		// Supplementary length/indexing is checked in the dedicated runtime
		// probe; keep these shared collection fixtures within the pin's BMP envelope.
		check(eq(rt("string.length", hir.L(str, "€A"), i32), lit(2))),
		check(eq(rt("string.substring", hir.L(str, "abcd "), str, lit(4), lit(1)), hir.L(str, "bcd"))),
		check(eq(rt("string.concat", hir.L(str, "a "), str, hir.L(str, "b")), hir.L(str, "a b"))),
		check(eq(rt("string.charCodeAt", hir.L(str, "€"), i32, lit(0)), lit(8364))),
		check(binary("||", hir.L(boolean, true), boomCall(), boolean)),
		check(&hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: binary("&&", hir.L(boolean, false), boomCall(), boolean)}),
		check(&hir.Expr{Kind: hir.Conditional, Type: boolean, X: hir.L(boolean, true), Y: hir.L(boolean, true), Z: boomCall()}),
		decl("n", i32, lit(0)), &hir.Stmt{Kind: hir.While, X: binary("<", local("n", i32), lit(3), boolean), Body: hir.B(assign("n", i32, binary("+", local("n", i32), lit(1), i32)), &hir.Stmt{Kind: hir.If, X: eq(local("n", i32), lit(2)), Body: &hir.Stmt{Kind: hir.Continue}}, &hir.Stmt{Kind: hir.If, X: eq(local("n", i32), lit(3)), Body: &hir.Stmt{Kind: hir.Break}})},
		ret(local("n", i32)),
	))
	extra.Static = true
	c.Methods = append(c.Methods, bad, extra)
	return fs
}

// Keep global definitions under review, including empty visibility sections and
// runtime dependencies. HIR dump goldens do not cover ABAP.
func TestGlobalClassDefinitions(t *testing.T) {
	definitions := map[string]string{}
	for _, f := range fixtures() {
		files, err := Emit(f.p)
		if err != nil {
			t.Fatal(err)
		}
		for name, src := range files {
			if !strings.HasSuffix(name, ".clas.abap") {
				continue
			}
			definition, _, found := strings.Cut(src, "ENDCLASS.\n")
			if !found {
				t.Fatalf("%s: missing ENDCLASS", name)
			}
			previous := -1
			for _, section := range []string{"PUBLIC", "PROTECTED", "PRIVATE"} {
				statement := section + " SECTION.\n"
				index := strings.Index(definition, statement)
				if index <= previous || strings.Count(definition, statement) != 1 {
					t.Fatalf("%s: missing, repeated or out-of-order %s SECTION", name, section)
				}
				previous = index
			}
			definitions[name] = definition + "ENDCLASS.\n"
		}
	}
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	var actual strings.Builder
	for _, name := range names {
		actual.WriteString(name + "\n" + definitions[name] + "\n")
	}
	path := filepath.Join("testdata", "global_definitions_v750.txt")
	gold, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual.String() != string(gold) {
		t.Fatalf("global definitions differ from %s", path)
	}
}

func TestFixtures(t *testing.T) {
	out := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out = filepath.Join(base, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) {
			files, err := Emit(f.p)
			if err != nil {
				t.Fatal(err)
			}
			names := hir.NewNames()
			cls := names.Get(f.name)
			m := names.Get("member.run")
			files[cls+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE i.\nactual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = %d ).\nENDMETHOD.\nENDCLASS.\n", cls, m, f.want)

			if f.name == "collection_fixture" {
				file := cls + ".clas.testclasses.abap"
				extra := fmt.Sprintf("actual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = 3 ).\n", cls, names.Get("member.extra"))
				oracleFiles, assertions := int8Oracle(t)
				for n, src := range oracleFiles {
					files[n] = src
				}
				extra += assertions
				files[file] = strings.Replace(files[file], "ENDMETHOD.", extra+"ENDMETHOD.", 1)
			}
			for n, s := range files {
				for _, line := range strings.Split(s, "\n") {
					if len(line) > 255 {
						t.Fatalf("%s: long line", n)
					}
					if strings.HasPrefix(line, "*") || strings.Contains(line, "\"") {
						t.Fatalf("%s: comment", n)
					}
				}
				if err := os.WriteFile(filepath.Join(out, n), []byte(s), 0644); err != nil {
					t.Fatal(err)
				}
			}
			dump := hir.Dump(f.p)
			path := filepath.Join("testdata", f.name+".hir")
			gold, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(gold) != dump {
				t.Fatalf("dump differs from %s", path)
			}
		})
	}
}

// Expected decimal strings come from Go, independently of int8Lit. The ABAP
// template formats the runtime int8 directly, without reconstructing its parts.
func int8Oracle(t *testing.T) (map[string]string, string) {
	t.Helper()
	i64 := hir.T(hir.I64)
	values := []int64{
		math.MinInt64, math.MinInt64 + 1, math.MaxInt64 - 1, math.MaxInt64,
		math.MinInt32 - 1, math.MinInt32, math.MinInt32 + 1,
		math.MaxInt32 - 1, math.MaxInt32, math.MaxInt32 + 1,
		-9007199254740993, -9007199254740992, 9007199254740992, 9007199254740993,
		-1_000_000_001, -1_000_000_000, -999_999_999, -1, 0, 1,
		999_999_999, 1_000_000_000, 1_000_000_001, 3074457345618258602,
	}
	c := &hir.Class{Name: "int8_oracle"}
	names := hir.NewNames()
	cls := names.Get(c.Name)
	var assertions strings.Builder
	assertions.WriteString("DATA value TYPE int8.\nDATA adjacent TYPE int8.\nDATA decimal TYPE string.\n")
	for i, value := range values {
		n := fmt.Sprintf("value_%d", i)
		m := method(n, i64, ret(hir.L(i64, value)))
		m.Static = true
		c.Methods = append(c.Methods, m)
		fmt.Fprintf(&assertions, "value = %s=>%s( ).\ndecimal = |{ value }|.\n", cls, names.Get("member."+n))
		fmt.Fprintf(&assertions, "cl_abap_unit_assert=>assert_equals( act = decimal exp = `%s` ).\n", strconv.FormatInt(value, 10))
		// Avoid overflowing MinInt64 while checking adjacent arithmetic.
		if value != math.MinInt64 {
			assertions.WriteString("adjacent = value - 1.\nadjacent = adjacent + 1.\n")
		} else {
			assertions.WriteString("adjacent = value + 1.\nadjacent = adjacent - 1.\n")
		}
		assertions.WriteString("cl_abap_unit_assert=>assert_equals( act = adjacent exp = value ).\n")
	}
	files, err := Emit(&hir.Program{Classes: []*hir.Class{c}})
	if err != nil {
		t.Fatal(err)
	}
	return files, assertions.String()
}
func TestRejectUnverified(t *testing.T) {
	p := &hir.Program{Classes: []*hir.Class{{Name: "bad", Methods: []*hir.Method{method("f", i32, ret(local("unknown", i32)))}}}}
	if files, err := Emit(p); files != nil || err == nil {
		t.Fatal("emitted invalid HIR")
	}
}

func TestTargetDiagnostics(t *testing.T) {
	number := hir.T(hir.Number)
	for _, x := range []*hir.Expr{
		{Node: hir.Node{ID: 17, Source: "input.ts:4"}, Kind: hir.Binary, Op: "/", Type: number, X: hir.L(number, 1.0), Y: hir.L(number, 0.0)},
		{Node: hir.Node{ID: 17, Source: "input.ts:4"}, Kind: hir.Binary, Op: "/", Type: number, X: hir.L(number, 1.0), Y: hir.L(number, math.Copysign(0, -1))},
		{Node: hir.Node{ID: 17, Source: "input.ts:4"}, Kind: hir.Binary, Op: "/", Type: number, X: hir.L(number, 1.0), Y: binary("+", hir.L(number, 1.0), hir.L(number, 1.0), number)},
		{Node: hir.Node{ID: 17, Source: "input.ts:4"}, Kind: hir.Binary, Op: "/", Type: number, X: hir.L(number, 1.0), Y: local("divisor", number)},
		{Node: hir.Node{ID: 17, Source: "input.ts:4"}, Kind: hir.Binary, Op: "%", Type: hir.T(hir.Number), X: hir.L(hir.T(hir.Number), 0.5), Y: hir.L(hir.T(hir.Number), 0.1)},
	} {
		m := method("f", x.Type, ret(x))
		m.Params = []hir.Param{{Name: "divisor", Type: number}}
		m.Static = true
		p := &hir.Program{Classes: []*hir.Class{{Name: "target", Methods: []*hir.Method{m}}}}
		files, err := Emit(p)
		if files != nil || err == nil || !strings.Contains(err.Error(), "node 17 (input.ts:4)") {
			t.Fatalf("missing target diagnostic: %v", err)
		}
	}
}
func TestLongLiteral(t *testing.T) {
	m := method("f", str, ret(hir.L(str, strings.Repeat("€` {|}\\ ", 300)+"\n\t")))
	m.Static = true
	files, err := Emit(&hir.Program{Classes: []*hir.Class{{Name: "long", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Fatal("long literal source line")
			}
		}
		if !strings.Contains(src, "uccpi( 10 )") {
			t.Fatal("newline not encoded")
		}
	}
}

// Dynamic receivers ensure these checks exercise lowering, not literal folding.
func TestStringUnitAccess(t *testing.T) {
	for _, op := range []string{"string.length", "string.charAt", "string.substring", "string.substr", "string.charCodeAt"} {
		t.Run(op, func(t *testing.T) {
			result := str
			var args []*hir.Expr
			switch op {
			case "string.length":
				result = i32
			case "string.charAt", "string.charCodeAt":
				args = []*hir.Expr{local("index", i32)}
				if op == "string.charCodeAt" {
					result = i32
				}
			default:
				args = []*hir.Expr{local("index", i32), lit(2)}
			}
			m := method("access", result, ret(rt(op, local("source", str), result, args...)))
			m.Static = true
			m.Params = []hir.Param{{Name: "source", Type: str}, {Name: "index", Type: i32}}
			files, err := Emit(&hir.Program{Classes: []*hir.Class{{Name: "access", Methods: []*hir.Method{m}}}})
			if err != nil {
				t.Fatal(err)
			}
			var src string
			for _, file := range files {
				src += file
			}
			if !strings.Contains(src, "strlen( ") {
				t.Fatal("missing native UTF-16 length")
			}
			section := regexp.MustCompile(`(?:DATA\()?([a-z]\w*)\)? = (?:CONV string\( )?\w+\+\w+\((?:1|\w+)\)(?: \))?\.`).FindStringSubmatch(src)
			if op != "string.length" && section == nil {
				t.Fatal("missing native string section")
			}
			if op == "string.charCodeAt" {
				if !regexp.MustCompile(`EXPORTING\s+data\s*=\s*`+section[1]+`\s+IMPORTING\s+buffer`).MatchString(src) || !strings.Contains(src, " * 256.") || len(regexp.MustCompile(`->convert\s*\(`).FindAllStringIndex(src, -1)) != 1 {
					t.Fatalf("expected one character conversion with direct UTF-16LE decoding:\n%s", src)
				}
			} else if strings.Contains(src, "cl_abap_conv") || strings.Contains(src, "xstrlen") {
				t.Fatal("string length/section must not convert codepages")
			}
		})
	}
}

func TestMalformedLiterals(t *testing.T) {
	for _, typ := range []hir.Type{str, boolean, hir.T(hir.I64), hir.T(hir.Optional, str)} {
		x := hir.L(typ, struct{}{})
		x.Node = hir.Node{ID: 42, Source: "bad.ts:7"}
		p := &hir.Program{Classes: []*hir.Class{{Name: "bad", Methods: []*hir.Method{method("f", typ, ret(x))}}}}
		if files, err := Emit(p); files != nil || err == nil || !strings.Contains(err.Error(), "node 42 (bad.ts:7)") {
			t.Fatalf("%s: missing diagnostic: %v", typ, err)
		}
	}
	for _, op := range []string{"string.substring", "string.charCodeAt"} {
		x := hir.L(str, 123)
		x.Node = hir.Node{ID: 42, Source: "bad.ts:7"}
		expr := rt(op, x, str, lit(0), lit(1))
		if op == "string.charCodeAt" {
			expr.Type = hir.T(hir.Number)
			expr.Args = expr.Args[:1]
		}
		p := &hir.Program{Classes: []*hir.Class{{Name: "bad", Methods: []*hir.Method{method("f", expr.Type, ret(expr))}}}}
		if files, err := Emit(p); files != nil || err == nil || !strings.Contains(err.Error(), "node 42 (bad.ts:7)") {
			t.Fatalf("%s: missing diagnostic: %v", op, err)
		}
	}
}

// This runs on both ABAP runtimes when exported, exercising literals whose
// template escaping matters and temporaries that must reset on every iteration.
func Test750Semantics(t *testing.T) {
	root := hir.Ref("SyntaxRoot")
	arr := hir.T(hir.Array, i32)
	eq := func(a, b *hir.Expr) *hir.Expr { return binary("==", a, b, boolean) }
	check := func(x *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", X: x, Type: boolean}, Body: ret(lit(99))}
	}
	instance := func() *hir.Expr {
		return &hir.Expr{Kind: hir.InstanceOf, Type: boolean, Owner: root.Name, X: local("ref", root)}
	}
	m := method("run", i32, hir.B(
		decl("ref", root, nil),
		check(&hir.Expr{Kind: hir.Unary, Type: boolean, Op: "!", X: instance()}),
		assign("ref", root, newObj(hir.Ref("SyntaxLeaf"))), check(instance()),
		decl("a", arr, newObj(arr)), run(rt("array.push", local("a", arr), i32, lit(7))),
		decl("n", i32, lit(0)),
		&hir.Stmt{Kind: hir.While, X: binary("<", local("n", i32), lit(3), boolean), Body: hir.B(
			decl("zero", i32, nil), check(eq(local("zero", i32), lit(0))), assign("zero", i32, lit(9)),
			decl("v", i32, &hir.Expr{Kind: hir.IndexGet, Type: i32, X: local("a", arr), Y: local("n", i32)}),
			check(eq(local("v", i32), &hir.Expr{Kind: hir.Conditional, Type: i32, X: eq(local("n", i32), lit(0)), Y: lit(7), Z: lit(0)})),
			assign("n", i32, binary("+", local("n", i32), lit(1), i32)),
		)}, ret(local("n", i32)),
	))
	m.Static = true
	// These expectations come from the original Go strings, never stringLit or
	// another emitted HIR literal. Byte comparison also preserves trailing blanks.
	literals := []struct{ name, value string }{
		{"backslash", `\`},
		{"braces", `{}`},
		{"pipe", `|`},
		{"quotes", `"'`},
		{"controls", "\x00\x01\b\t\n\v\f\r\x1f\x7f"},
		{"blanks", "  trailing  "},
		{"unicode", "€😀𐐷"},
		{"empty", ""},
		{"byte_mode", "€€€€€€ IN BYTE MODE end"},
		{"character_mode", "€€€€€€ IN CHARACTER MODE end"},
		{"long", strings.Repeat("{|}\\€😀\"' ", 100) + "\n\tend  "},
	}
	// The current chunk budget is 60 input bytes. Place each special character
	// just before, at, and after that boundary, including multi-byte characters
	// and controls that split template runs. Repeated padding crosses more chunks.
	for _, special := range []string{"\\", "{", "}", "|", "\"", "'", "\n", "\t", "😀", " "} {
		for _, padding := range []int{59, 60, 61, 119, 120, 121} {
			literals = append(literals, struct{ name, value string }{
				fmt.Sprintf("boundary_%d", len(literals)),
				strings.Repeat("a", padding) + special + strings.Repeat("b", 65) + special + "  ",
			})
		}
	}
	methods := []*hir.Method{m}
	for _, literal := range literals {
		getter := method(literal.name, str, ret(hir.L(str, literal.value)))
		getter.Static = true
		methods = append(methods, getter)
	}
	// Retain the concatenation regression, with an independent byte expectation.
	concat := method("concat", str, ret(rt("string.concat", hir.L(str, " {|}\\`€ \"' \n\t"), str, hir.L(str, "end  "))))
	concat.Static = true
	methods = append(methods, concat)
	literals = append(literals, struct{ name, value string }{"concat", " {|}\\`€ \"' \n\tend  "})
	p := &hir.Program{Classes: []*hir.Class{{Name: root.Name}, {Name: "SyntaxLeaf", Super: root.Name}, {Name: "Syntax750", Methods: methods}}}
	files, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	names := hir.NewNames()
	cls := names.Get("Syntax750")
	var unit strings.Builder
	fmt.Fprintf(&unit, "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE xstring.\nDATA expected TYPE xstring.\nDATA hex_chunk TYPE xstring.\ncl_abap_unit_assert=>assert_equals( act = %s=>%s( ) exp = 3 ).\n", cls, names.Get("member.run"))
	for _, literal := range literals {
		fmt.Fprintf(&unit, "actual = cl_abap_codepage=>convert_to( source = %s=>%s( ) ).\nCLEAR expected.\n", cls, names.Get("member."+literal.name))
		// Assignment to xstring decodes hex text; BYTE MODE joins raw bytes.
		bytes := []byte(literal.value)
		for len(bytes) > 0 {
			n := min(len(bytes), 60)
			fmt.Fprintf(&unit, "hex_chunk = '%s'.\nCONCATENATE expected hex_chunk INTO expected IN BYTE MODE.\n", strings.ToUpper(hex.EncodeToString(bytes[:n])))
			bytes = bytes[n:]
		}
		fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = actual exp = expected msg = '%s' ).\n", literal.name)
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[cls+".clas.testclasses.abap"] = unit.String()
	for _, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Fatal("long source line")
			}
		}
	}
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out := filepath.Join(base, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for n, src := range files {
			if err := os.WriteFile(filepath.Join(out, n), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func Test750Syntax(t *testing.T) {
	p := fixtures()[2].p
	modern, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	var a string
	for _, src := range modern {
		a += src
	}
	if !strings.Contains(a, "IS INSTANCE OF") || !strings.Contains(a, "VALUE abap_bool( )") || strings.Contains(a, "narrowed ?=") {
		t.Fatal("default syntax or helper regression")
	}
}
