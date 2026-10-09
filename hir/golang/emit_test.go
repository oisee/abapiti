package golang

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func execute(t *testing.T, p *hir.Program, main string) string {
	t.Helper()
	files, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Emit(p)
	if err != nil || !reflect.DeepEqual(files, again) {
		t.Fatal("nondeterministic emission", err)
	}
	dir := t.TempDir()
	files["go.mod"] = "module fixture\n\ngo 1.26.0\n"
	files["main.go"] = "package main\nimport \"fmt\"\nfunc main(){" + main + "}\n"
	for n, s := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s\ngenerated module: %s", err, out, dir)
	}
	return string(out)
}
func entry(c, m string) string { return hir.NewNames().Get("body." + c + "." + m) }
func TestFixtures(t *testing.T) {
	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) {
			gold, err := os.ReadFile(filepath.Join("..", "abap", "testdata", f.name+".hir"))
			if err != nil {
				t.Fatal(err)
			}
			if hir.Dump(f.p) != string(gold) {
				t.Fatal("copied fixture differs from ABAP golden")
			}
			main := fmt.Sprintf("fmt.Println(%s())", entry(f.name, "run"))
			want := strconv.Itoa(f.want) + "\n"
			if f.name == "collection_fixture" {
				main += fmt.Sprintf(";fmt.Println(%s())", entry(f.name, "extra"))
				want += "3\n"
				// The ABAP int8 oracle's exact values and adjacent arithmetic, in the same module.
				values := []int64{math.MinInt64, math.MinInt64 + 1, math.MaxInt64 - 1, math.MaxInt64, math.MinInt32 - 1, math.MinInt32, math.MinInt32 + 1, math.MaxInt32 - 1, math.MaxInt32, math.MaxInt32 + 1, -9007199254740993, -9007199254740992, 9007199254740992, 9007199254740993, -1000000001, -1000000000, -999999999, -1, 0, 1, 999999999, 1000000000, 1000000001, 3074457345618258602}
				c := &hir.Class{Name: "int8_oracle"}
				f.p.Classes = append(f.p.Classes, c)
				for i, v := range values {
					n := fmt.Sprintf("value_%d", i)
					m := method(n, hir.T(hir.I64), ret(hir.L(hir.T(hir.I64), v)))
					m.Static = true
					c.Methods = append(c.Methods, m)
					main += fmt.Sprintf(";{v:=%s();fmt.Println(v);", entry(c.Name, n))
					if v == math.MinInt64 {
						main += "a:=v+1;a=a-1;"
					} else {
						main += "a:=v-1;a=a+1;"
					}
					main += "if a!=v {panic(\"adjacent arithmetic\")}}"
					want += strconv.FormatInt(v, 10) + "\n"
				}
			}
			if got := execute(t, f.p, main); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
func TestRuntimeCatalogue(t *testing.T) {
	supported, unsupported := RuntimeOps()
	t.Log("supported:", strings.Join(supported, ", "))
	t.Log("not supported:", strings.Join(unsupported, ", "))
	if len(supported)+len(unsupported) != len(hir.RuntimeSpecs)+len(hir.SpecialOps) {
		t.Fatal("catalogue incomplete")
	}
	for _, op := range unsupported {
		t.Run(op, func(t *testing.T) {
			receiver := hir.T(hir.RuntimeSpecs[op].Receiver)
			switch receiver.Kind {
			case hir.Array, hir.OrderedSet:
				receiver.Args = []hir.Type{i32}
			case hir.OrderedMap:
				receiver.Args = []hir.Type{str, i32}
			}
			params, result, ok := hir.RuntimeSignature(op, receiver)
			if !ok {
				switch op {
				case "object.classOf":
					receiver = hir.Ref("Object")
					result = hir.T(hir.ClassValue)
				case "classvalue.new":
					receiver = hir.T(hir.ClassValue)
					result = hir.Ref("Object")
				case "regexp.new":
					receiver = str
					params = []hir.Type{str}
					result = hir.T(hir.RegExp)
				default:
					t.Fatalf("missing catalogue signature for %s", op)
				}
			}
			x := rt(op, local("receiver", receiver), result)
			m := method("probe", result, ret(x))
			m.Static = true
			m.Params = []hir.Param{{Name: "receiver", Type: receiver}}
			for i, typ := range params {
				n := fmt.Sprintf("arg%d", i)
				m.Params = append(m.Params, hir.Param{Name: n, Type: typ})
				x.Args = append(x.Args, local(n, typ))
			}
			p := &hir.Program{Classes: []*hir.Class{{Name: "Object"}, {Name: "Probe", Methods: []*hir.Method{m}}}}
			if _, err := Emit(p); err == nil || !strings.Contains(err.Error(), "not supported in the Go prototype: "+op) {
				t.Fatal(err)
			}
		})
	}
}
func TestRejectUnverified(t *testing.T) {
	if _, err := Emit(nil); err == nil {
		t.Fatal("accepted nil program")
	}
}

func TestSemanticEdges(t *testing.T) {
	i64 := hir.T(hir.I64)
	number := hir.T(hir.Number)
	optString := hir.T(hir.Optional, str)
	c := &hir.Class{Name: "Edges"}
	p := &hir.Program{Classes: []*hir.Class{c, {Name: "Object"}}}
	add := func(n string, x *hir.Expr) {
		m := method(n, x.Type, ret(x))
		m.Static = true
		c.Methods = append(c.Methods, m)
	}
	for _, test := range []struct {
		name, op string
		a, b     int64
		checked  bool
	}{
		{"safe", "+", 9007199254740991, 0, true},
		{"unsafe", "+", 9007199254740991, 1, true},
		{"multiply", "*", math.MaxInt64, math.MaxInt64, true},
		{"native", "+", math.MaxInt64, 1, false},
	} {
		x := binary(test.op, hir.L(i64, test.a), hir.L(i64, test.b), i64)
		x.CheckIntegerOverflow = test.checked
		add(test.name, x)
	}
	neg := &hir.Expr{Kind: hir.Unary, Type: i64, Op: "-", X: hir.L(i64, math.MinInt64), CheckIntegerOverflow: true}
	add("negate", neg)
	add("supplementary_length", rt("string.length", hir.L(str, "😀A"), i32))
	add("high_surrogate", rt("string.charCodeAt", rt("string.substring", hir.L(str, "😀A"), str, lit(0), lit(1)), i32, lit(0)))
	add("low_surrogate", rt("string.charCodeAt", hir.L(str, "😀A"), i32, lit(1)))
	add("char_bounds", rt("string.charCodeAt", hir.L(str, "A"), i32, lit(1)))
	add("negative_remainder", rt("i64.remainder2", hir.L(i64, -3), i64))
	add("negative_number_remainder", rt("number.remainder2", hir.L(number, -3.0), number))
	add("index", rt("number.index", hir.L(number, 1e20), i32))
	add("from_i32", rt("number.fromI32", lit(-9), number))
	add("decimal", rt("number.toString", hir.L(number, 9007199254740991.0), str))
	add("fraction", rt("number.toString", hir.L(number, 0.5), str))
	add("integer_decimal", rt("i32.toString", lit(-2147483648), str))
	add("wide_decimal", rt("i64.toString", hir.L(i64, math.MinInt64), str))
	dyn := hir.T(hir.Dynamic)
	boxed := rt("dynamic.of", hir.L(str, "😀"), dyn)
	add("dynamic_string", rt("string.length", rt("dynamic.asString", boxed, str), i32))
	add("dynamic_tag", rt("dynamic.isString", boxed, boolean))
	add("dynamic_function", rt("dynamic.isFunction", boxed, boolean))
	add("dynamic_ref", &hir.Expr{Kind: hir.InstanceOf, Type: boolean, Owner: "Object", X: rt("dynamic.asRef", rt("dynamic.of", newObj(hir.Ref("Object")), dyn), hir.Ref("Object"))})
	// Optional reference reads must return a nil reference when absent and retain identity when present.
	arr := hir.T(hir.Array, hir.Ref("Object"))
	oref := hir.T(hir.Optional, hir.Ref("Object"))
	ar := local("a", arr)
	obj := local("o", hir.Ref("Object"))
	m := method("ref_get", boolean, hir.B(decl("a", arr, newObj(arr)), decl("o", hir.Ref("Object"), newObj(hir.Ref("Object"))), decl("ref", oref, obj), run(rt("array.push", ar, i32, obj)),
		ret(binary("&&", binary("==", rt("array.get", ar, oref, lit(0)), local("ref", oref), boolean), &hir.Expr{Kind: hir.IsUndefined, Type: boolean, X: rt("array.get", ar, oref, lit(8))}, boolean))))
	m.Static = true
	c.Methods = append(c.Methods, m)
	// The inner catch has the same Go storage type, but a distinct HIR payload type.
	typed := method("typed", i32, hir.B(&hir.Stmt{Kind: hir.Try, Name: "outer", Type: optString, Body: &hir.Stmt{Kind: hir.Try, Name: "inner", Type: str, Body: &hir.Stmt{Kind: hir.Throw, X: hir.L(optString, "x")}, Else: ret(lit(99))}, Else: ret(lit(9))}))
	typed.Static = true
	c.Methods = append(c.Methods, typed)
	trapped := method("trapped", i32, hir.B(&hir.Stmt{Kind: hir.Try, Name: "caught", Type: str, Body: &hir.Stmt{Kind: hir.Trap, Name: "coverage"}, Else: ret(lit(99))}))
	trapped.Static = true
	c.Methods = append(c.Methods, trapped)
	mt := hir.T(hir.OrderedMap, i32, str)
	ml := local("m", mt)
	snapshot := method("snapshot", i32, hir.B(decl("m", mt, newObj(mt)), run(rt("map.set", ml, mt, lit(1), hir.L(str, "a"))), run(rt("map.set", ml, mt, lit(2), hir.L(str, "b"))), decl("n", i32, lit(0)),
		&hir.Stmt{Kind: hir.ForEach, Name: "k", Type: i32, X: rt("map.keys", ml, hir.T(hir.Array, i32)), Body: hir.B(run(rt("map.set", ml, mt, lit(3), hir.L(str, "c"))), assign("n", i32, binary("+", binary("*", local("n", i32), lit(10), i32), local("k", i32), i32)))}, ret(local("n", i32))))
	snapshot.Static = true
	c.Methods = append(c.Methods, snapshot)
	main := ""
	// Check the panic kind as well: the method's deferred return recovery must not swallow faults.
	for _, n := range []string{"unsafe", "multiply", "native", "negate", "char_bounds", "fraction"} {
		main += fmt.Sprintf("func(){defer func(){if _,ok:=recover().(rangeFault);!ok{panic(\"wrong fault\")}}();%s();panic(\"missing fault\")}();", entry(c.Name, n))
	}
	main += fmt.Sprintf("func(){defer func(){if _,ok:=recover().(trap);!ok{panic(\"trap was caught\")}}();%s();panic(\"missing trap\")}();", entry(c.Name, "trapped"))
	names := []string{"safe", "supplementary_length", "high_surrogate", "low_surrogate", "negative_remainder", "negative_number_remainder", "index", "from_i32", "decimal", "integer_decimal", "wide_decimal", "dynamic_string", "dynamic_tag", "dynamic_function", "dynamic_ref", "ref_get", "typed", "snapshot"}
	for _, n := range names {
		main += fmt.Sprintf("fmt.Println(%s());", entry(c.Name, n))
	}
	want := "9007199254740991\n3\n55357\n56832\n-1\n-1\n2147483647\n-9\n9007199254740991\n-2147483648\n-9223372036854775808\n2\ntrue\nfalse\ntrue\ntrue\n9\n12\n"
	if got := execute(t, p, main); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPackageAndDeclarationOrder(t *testing.T) {
	p := fixtures()[0].p
	files, err := EmitPackage(p, "generated")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range files {
		if !strings.HasPrefix(src, "package generated\n") {
			t.Fatal("wrong package")
		}
	}
	for i, j := 0, len(p.Classes)-1; i < j; i, j = i+1, j-1 {
		p.Classes[i], p.Classes[j] = p.Classes[j], p.Classes[i]
	}
	again, err := EmitPackage(p, "generated")
	if err != nil || !reflect.DeepEqual(files, again) {
		t.Fatal("declaration order changed output", err)
	}
	for _, name := range []string{"", "_", "for", "a-b"} {
		if _, err := EmitPackage(p, name); err == nil {
			t.Fatalf("accepted package name %q", name)
		}
	}
}
