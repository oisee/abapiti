package abap

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Materialize emitted ABAP Unit cases for the OSG gogen/kernel gate. Node
// supplies expected values and acceptance boundaries, including signed zero.
func TestNumericFixtures(t *testing.T) {
	inputs := []string{"42abc", "-7", "+3", "\ufeff\u2007+12tail", "9007199254740991", "-9007199254740991", "0xFF", "0", "+000tail", "00000000000000000000000000000000000000123e4", "", "abc", "Infinity", "NaN", "-0", "-000tail", "9007199254740992", "-9007199254740992", strings.Repeat("9", 400), "\u008542", "+", "--1", "1e21", "1.9", "-1.9", "  \t\r\n22x"}
	raw, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(s=>{const n=parseInt(s,10);return {allowed:Number.isSafeInteger(n)&&!Object.is(n,-0),value:Number.isFinite(n)?n:0}})))`, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	var oracle []struct {
		Allowed bool
		Value   float64
	}
	if err := json.Unmarshal(out, &oracle); err != nil {
		t.Fatal(err)
	}
	str, num, i64, dyn := hir.T(hir.String), hir.T(hir.Number), hir.T(hir.I64), hir.T(hir.Dynamic)
	rt := func(op string, x *hir.Expr, typ hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.RuntimeOp, Op: op, X: x, Type: typ}
	}
	method := func(name string, param hir.Type, result hir.Type, x *hir.Expr) *hir.Method {
		return &hir.Method{Name: name, Static: true, Params: []hir.Param{{Name: "input", Type: param}}, Result: result, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: x})}
	}
	boxed := rt("dynamic.of", hir.V("input", i64), dyn)
	p := &hir.Program{Classes: []*hir.Class{{Name: "Numeric", Methods: []*hir.Method{
		method("parse", str, num, rt("string.parseIntNumber", hir.V("input", str), num)),
		method("ceil", num, num, rt("number.ceil", hir.V("input", num), num)),
		method("box", i64, dyn, boxed),
		// Number argument conversion must precede parsing. The dynamic ToString
		// boundary permits safe integers, and raises on fractions/unsafe numbers.
		method("number_parse", num, num, rt("string.parseIntNumber", rt("dynamic.toString", rt("dynamic.of", hir.V("input", num), dyn), str), num)),
		{Name: "zero", Static: true, Result: num, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: rt("number.ceil", hir.L(num, 0.0), num)})},
	}}}}
	// Revisit the same emitted parser inside a loop: all its state must reset.
	loop := method("loop", str, num, hir.V("total", num))
	loop.Body = hir.B(decl("i", hir.T(hir.I32), lit(0)), decl("total", num, hir.L(num, 0.0)),
		&hir.Stmt{Kind: hir.While, X: binary("<", local("i", hir.T(hir.I32)), lit(3), hir.T(hir.Bool)), Body: hir.B(
			assign("total", num, binary("+", hir.V("total", num), rt("string.parseIntNumber", hir.V("input", str), num), num)),
			assign("i", hir.T(hir.I32), binary("+", local("i", hir.T(hir.I32)), lit(1), hir.T(hir.I32))))},
		&hir.Stmt{Kind: hir.Return, X: hir.V("total", num)})
	p.Classes[0].Methods = append(p.Classes[0].Methods, loop)
	opt := hir.T(hir.Optional, i64)
	p.Classes[0].Methods = append(p.Classes[0].Methods, method("optional_box", opt, dyn, rt("dynamic.of", hir.V("input", opt), dyn)))
	optLoop := method("optional_loop", opt, dyn, nil)
	optLoop.Body = hir.B(decl("value", opt, hir.V("input", opt)), decl("boxed", dyn, nil), decl("i", hir.T(hir.I32), lit(0)),
		&hir.Stmt{Kind: hir.While, X: binary("<", local("i", hir.T(hir.I32)), lit(2), hir.T(hir.Bool)), Body: hir.B(
			assign("boxed", dyn, rt("dynamic.of", hir.V("value", opt), dyn)),
			assign("value", opt, hir.L(opt, nil)),
			assign("i", hir.T(hir.I32), binary("+", local("i", hir.T(hir.I32)), lit(1), hir.T(hir.I32))))},
		ret(hir.V("boxed", dyn)))
	p.Classes[0].Methods = append(p.Classes[0].Methods, optLoop)
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	class := names.Get("Numeric")
	var decl, impl strings.Builder
	count := 0
	add := func(body string) {
		name := fmt.Sprintf("case_%d", count)
		count++
		fmt.Fprintf(&decl, "METHODS %s FOR TESTING.\n", name)
		fmt.Fprintf(&impl, "METHOD %s.\n%s\nENDMETHOD.\n", name, body)
	}
	check := func(call string, allowed bool, value float64) string {
		if !allowed {
			return "TRY.\nDATA actual TYPE f.\nactual = " + call + ".\ncl_abap_unit_assert=>fail( msg = 'expected numeric refusal' ).\nCATCH cx_sy_range_out_of_bounds.\nENDTRY."
		}
		return fmt.Sprintf("DATA actual TYPE f.\nDATA expected TYPE f.\nactual = %s.\nexpected = '%s'.\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).", call, fmt.Sprint(value))
	}
	for i, input := range inputs {
		// Use the emitter for literals too, preserving UTF-16 and the line envelope.
		lit := &hir.Program{Classes: []*hir.Class{{Name: fmt.Sprintf("Input%d", i), Methods: []*hir.Method{{Name: "text", Static: true, Result: str, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(str, input)})}}}}}
		fs, ns, e := emitABAPNamed(t, lit)
		if e != nil {
			t.Fatal(e)
		}
		for n, s := range fs {
			files[n] = s
		}
		call := class + "=>" + names.Get("member.parse") + "( " + ns.Get(fmt.Sprintf("Input%d", i)) + "=>" + ns.Get("member.text") + "( ) )"
		add(check(call, oracle[i].Allowed, oracle[i].Value))
	}
	ceils := []float64{-1.5, -1, -0.5, math.Copysign(0, -1), 0, math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64, 0.5, 1.5, 4503599627370495.5, 4503599627370496, 9007199254740992, 1e100, -1e100, math.MaxFloat64, -math.MaxFloat64}
	raw, err = json.Marshal(ceils)
	if err != nil {
		t.Fatal(err)
	}
	out, err = exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(n=>Math.ceil(n))))`, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	var ceilOracle []float64
	if err = json.Unmarshal(out, &ceilOracle); err != nil {
		t.Fatal(err)
	}
	for i, v := range ceils {
		add(check(fmt.Sprintf("%s=>%s( '%s' )", class, names.Get("member.ceil"), fmt.Sprint(v)), !(v > -1 && v < 0), ceilOracle[i]))
	}
	add(check(class+"=>"+names.Get("member.zero")+"( )", true, 0))
	add(check(class+"=>"+names.Get("member.loop")+"( `-7` )", true, -21))
	for _, v := range []int64{0, 1, -1, 2147483648, 9007199254740991, -9007199254740991, 9007199254740992, -9007199254740992, math.MaxInt64, math.MinInt64} {
		allowed := v >= -9007199254740991 && v <= 9007199254740991
		call := fmt.Sprintf("%s=>%s( '%d' )", class, names.Get("member.box"), v)
		if !allowed {
			add("TRY.\nDATA boxed TYPE REF TO " + names.Get("runtime.dynamic") + ".\nboxed = " + call + ".\ncl_abap_unit_assert=>fail( msg = 'expected boxing refusal' ).\nCATCH cx_sy_range_out_of_bounds.\nENDTRY.")
			continue
		}
		add(fmt.Sprintf("DATA boxed TYPE REF TO %s.\nDATA actual TYPE f.\nDATA expected TYPE f.\nboxed = %s.\nactual = boxed->as_number( ).\nexpected = '%d'.\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\ncl_abap_unit_assert=>assert_equals( act = boxed->type_of( ) exp = `number` ).", names.Get("runtime.dynamic"), call, v))
	}
	for _, v := range []float64{0, math.Copysign(0, -1), 42, -7, 9007199254740991, 1.9, -1.9, 1e21, 1e-7} {
		add(check(fmt.Sprintf("%s=>%s( '%s' )", class, names.Get("member.number_parse"), fmt.Sprint(v)), v == math.Trunc(v) && math.Abs(v) <= 9007199254740991, v))
	}
	optionalDecl := "DATA input TYPE REF TO " + names.Get("runtime."+opt.String()) + ".\nDATA boxed TYPE REF TO " + names.Get("runtime.dynamic") + ".\n"
	absent := "IF boxed IS BOUND.\ncl_abap_unit_assert=>fail( msg = 'expected absent integer box' ).\nENDIF."
	add(optionalDecl + "boxed = " + class + "=>" + names.Get("member.optional_box") + "( input ).\n" + absent)
	add(optionalDecl + "CREATE OBJECT input.\nboxed = " + class + "=>" + names.Get("member.optional_box") + "( input ).\n" + absent)
	add(optionalDecl + "CREATE OBJECT input.\ninput->has = abap_true.\ninput->value = 42.\nboxed = " + class + "=>" + names.Get("member.optional_box") + "( input ).\ncl_abap_unit_assert=>assert_equals( act = boxed->nval exp = 42 ).")
	add(optionalDecl + "CREATE OBJECT input.\ninput->has = abap_true.\ninput->value = 42.\nboxed = " + class + "=>" + names.Get("member.optional_loop") + "( input ).\n" + absent)
	add(optionalDecl + "CREATE OBJECT input.\ninput->has = abap_true.\ninput->value = '9007199254740992'.\nTRY.\nboxed = " + class + "=>" + names.Get("member.optional_box") + "( input ).\ncl_abap_unit_assert=>fail( msg = 'expected optional integer refusal' ).\nCATCH cx_sy_range_out_of_bounds.\nENDTRY.")
	files[class+".clas.testclasses.abap"] = "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\n" + decl.String() + "ENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
	dir := t.TempDir()
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		dir = filepath.Join(base, "TestNumericFixtures")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for n, s := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d emitted numeric ABAP Unit cases", count)
}

func TestNumericNonfiniteLiteralRefusals(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1)} {
		x := &hir.Expr{Kind: hir.RuntimeOp, Op: "number.ceil", X: hir.L(hir.T(hir.Number), v), Type: hir.T(hir.Number)}
		p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "run", Static: true, Result: x.Type, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: x})}}}}}
		if _, err := emitABAP(t, p); err == nil {
			t.Fatalf("accepted nonrepresentable %v", v)
		}
	}
}
