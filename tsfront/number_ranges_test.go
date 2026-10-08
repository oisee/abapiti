package tsfront

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

func rangeClass(t *testing.T, p *hir.Program, suffix string) *hir.Class {
	t.Helper()
	for _, c := range p.Classes {
		if strings.HasSuffix(c.Name, suffix) {
			return c
		}
	}
	t.Fatal("missing class", suffix)
	return nil
}
func rangeLocals(m *hir.Method) map[string]hir.Kind {
	locals := map[string]hir.Kind{}
	walkNumberStmt(m.Body, func(s *hir.Stmt) {
		if s.Kind == hir.VarDecl {
			locals[s.Name] = s.Type.Kind
		}
	}, func(e *hir.Expr) {})
	return locals
}
func TestNumberRangeDataflow(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
  private inc(n:number):number {return n+1;}
  run(s:string,external:number):number {
   const tiny=17; const wide=2147483647+1; const frac=0.25; const quotient=5/2;
   const publicValue=external+1; const negativeZero=-0; const signedZero=0*-1;
   const safe=Math.max(1,Math.min(s.length,99)); const remainder=5%3;
   let i=0; while(i<s.length) {i++;}
   let endless=0; while(external>0) {endless++;}
   let branch=0; if(external>0) {branch=2147483648;}
   return this.inc(tiny)+wide+frac+publicValue+safe+remainder+i+endless+branch+negativeZero+signedZero;
  }
 }`}, []string{"probe.ts"})
	c := rangeClass(t, p, ".Probe")
	var run, inc *hir.Method
	for _, m := range c.Methods {
		if m.Name == "run" {
			run = m
		}
		if m.Name == "inc" {
			inc = m
		}
	}
	kinds := rangeLocals(run)
	for name, want := range map[string]hir.Kind{"tiny": hir.I32, "wide": hir.I64, "frac": hir.Number, "quotient": hir.Number, "publicValue": hir.Number, "negativeZero": hir.Number, "signedZero": hir.Number, "safe": hir.I32, "remainder": hir.I32, "i": hir.I32, "endless": hir.Number, "branch": hir.I64} {
		if kinds[name] != want {
			t.Errorf("%s: %s, want %s", name, kinds[name], want)
		}
	}
	if inc.Params[0].Type.Kind != hir.I32 || inc.Result.Kind != hir.I32 {
		t.Fatalf("internal signature: %+v", inc)
	}
	if run.Params[1].Type.Kind != hir.Number || run.Result.Kind != hir.Number {
		t.Fatal("public ABI narrowed")
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}
func TestNumberRangesLexer(t *testing.T) {
	p, diags := lowerClosure(t)
	stream := rangeClass(t, p, "lexer_stream.ts.LexerStream")
	for _, f := range stream.Fields {
		want := hir.Number
		if f.Name == "offset" {
			want = hir.I32
		}
		if f.Type.Kind != hir.String && f.Type.Kind != want {
			t.Errorf("stream %s: %s want %s", f.Name, f.Type.Kind, want)
		}
	}
	// Checked public-entry conversion establishes the private index invariant.
	// The ABI stays Number; rejected inputs are diagnosed and tested below.
	buffer := rangeClass(t, p, "lexer_buffer.ts.LexerBuffer")
	for _, f := range buffer.Fields {
		if (f.Name == "start" || f.Name == "end") && f.Type.Kind != hir.I32 {
			t.Fatalf("checked buffer %s did not become I32: %s", f.Name, f.Type.Kind)
		}
	}
	var notes strings.Builder
	for _, d := range diags {
		if d.Category == "note-number-ranges" {
			notes.WriteString(d.Message + "\n")
		}
	}
	if out := os.Getenv("ABAPITI_RANGE_NOTES"); out != "" {
		if err := os.WriteFile(out, []byte(notes.String()), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	if out := os.Getenv("ABAPITI_RANGE_OUT"); out != "" {
		files, names, err := abap.EmitNamed(p)
		if err != nil {
			t.Fatal(err)
		}
		class := names.Get(buffer.Name)
		files[class+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_range_boundary DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS public_fraction FOR TESTING.
ENDCLASS.
CLASS ltcl_range_boundary IMPLEMENTATION.
METHOD public_fraction.
DATA buffer TYPE REF TO %s.
DATA offset TYPE f.
DATA actual TYPE f.
DATA expected TYPE f.
DATA text TYPE string.
DATA caught TYPE abap_bool.
buffer = NEW %s( %s = %cabc%c ).
offset = 0.
buffer->%s( %s = offset ).
actual = buffer->%s( ).
expected = 1.
cl_abap_unit_assert=>assert_equals( act = actual exp = expected ).
text = buffer->%s( ).
cl_abap_unit_assert=>assert_equals( act = text exp = %ca%c ).
offset = '-0.5'.
TRY.
buffer->%s( %s = offset ).
CATCH cx_sy_range_out_of_bounds.
caught = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_equals( act = caught exp = abap_true ).
CLEAR caught.
offset = '2147483647'.
TRY.
buffer->%s( %s = offset ).
CATCH cx_sy_range_out_of_bounds.
caught = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_equals( act = caught exp = abap_true ).
ENDMETHOD.
ENDCLASS.
`, class, class, names.Get("param.raw"), '`', '`', names.Get("member.add"), names.Get("param.offset"), names.Get("member.length"), names.Get("member.get"), '`', '`', names.Get("member.add"), names.Get("param.offset"), names.Get("member.add"), names.Get("param.offset"))
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestNumberRangesBranchField(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
  private counter=0;
  private uninitialized:number;
  tick(flag:boolean):number {if(flag){this.counter=1;} this.counter--;return this.counter;}
  missing():number {return this.uninitialized;}
  run(flag:boolean):number {let x=0;{let y=0.5;y+=1;}if(flag){x=2147483648;}return x;}
 }`}, []string{"probe.ts"})
	c := rangeClass(t, p, ".Probe")
	for _, f := range c.Fields {
		if f.Type.Kind != hir.Number {
			t.Errorf("field %s narrowed without an invariant: %s", f.Name, f.Type.Kind)
		}
	}
	for _, m := range c.Methods {
		if m.Name == "run" {
			if rangeLocals(m)["x"] != hir.I64 {
				t.Fatal("outer branch did not include wide integer assignment")
			}
		}
	}

}
func TestNumberIntervalsDoNotRoundOrEraseSignedZero(t *testing.T) {
	for _, value := range []float64{math.Copysign(0, -1), 0.5, math.Inf(1), math.NaN(), 9007199254740992} {
		if numericLiteral(hir.L(hir.T(hir.Number), value)).state != 2 {
			t.Errorf("accepted %v", value)
		}
	}
	for _, tc := range []struct {
		op   string
		a, b numberInterval
	}{
		{"+", integerRange(safeInteger, safeInteger), integerRange(1, 1)},
		{"*", integerRange(0, 0), integerRange(-1, -1)},
		{"%", integerRange(-4, -4), integerRange(2, 2)},
		{"/", integerRange(4, 4), integerRange(2, 2)},
	} {
		if numberArithmetic(tc.op, tc.a, tc.b).state != 2 {
			t.Errorf("unsafe %s accepted", tc.op)
		}
	}
	if got := numberArithmetic("+", integerRange(math.MaxInt32, math.MaxInt32), integerRange(1, 1)); got.kind() != hir.I64 {
		t.Fatal(got)
	}
}
func TestRangeProofMutationRejected(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `export class Probe {run(s:string):number {let i=0;while(i<s.length){i++;}return i;}}`}, []string{"probe.ts"})
	c := rangeClass(t, p, ".Probe")
	var proof *hir.Expr
	walkNumberStmt(c.Methods[0].Body, func(s *hir.Stmt) {}, func(e *hir.Expr) {
		if e.Kind == hir.NumericConvert && e.Type.Kind == hir.I32 {
			proof = e
		}
	})
	if proof == nil { // Construct an explicit illegal narrowing in the verified function.
		m := c.Methods[0]
		proof = &hir.Expr{Kind: hir.NumericConvert, Type: hir.T(hir.I32), X: hir.L(hir.T(hir.Number), 0.5)}
		m.Body = hir.B(&hir.Stmt{Kind: hir.Return, X: rangeConversion(proof, hir.T(hir.Number), numberTop)})
	} else {
		proof.Range = nil
	}
	if len(hir.Verify(p)) == 0 {
		t.Fatal("missing proof mutation passed")
	}
}
func TestRangeBoundConstantsOutsideLoop(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `export class Probe {run(s:string,n:number):string {let out="";while(n>0){out=s.charAt(n);n--;}return out;}}`}, []string{"probe.ts"})
	files, err := abap.Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range files {
		if strings.Contains(source, "range_i32_lower") {
			constant := strings.Index(source, "CONSTANTS range_i32_lower")
			loop := strings.Index(source, "DO.")
			if constant < 0 || loop < 0 || constant > loop {
				t.Fatal(name, "bound not hoisted")
			}
			if strings.Count(source, "VALUE '-2147483648'") != 1 {
				t.Fatal("bound repeated")
			}
			return
		}
	}
	t.Fatal("missing index-bound constants")
}
func TestNumberRangeNotesStatements(t *testing.T) {
	if out := os.Getenv("ABAPITI_RANGE_NOTES"); out != "" {
		_, diags := lowerStmtsClosure(t)
		var notes strings.Builder
		for _, d := range diags {
			if d.Category == "note-number-ranges" {
				notes.WriteString(d.Message + "\n")
			}
		}
		if err := os.WriteFile(filepath.Join(filepath.Dir(out), "statements-number-sites.txt"), []byte(notes.String()), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntervalTransfersEncloseConcreteJSArithmetic(t *testing.T) {
	for lo := -5; lo <= 5; lo++ {
		for hi := lo; hi <= 5; hi++ {
			for blo := 1; blo <= 5; blo++ {
				for bhi := blo; bhi <= 5; bhi++ {
					a, b := integerRange(int64(lo), int64(hi)), integerRange(int64(blo), int64(bhi))
					for _, op := range []string{"+", "-", "*", "%"} {
						r := numberArithmetic(op, a, b)
						if r.state != 1 {
							continue
						}
						for x := lo; x <= hi; x++ {
							for y := blo; y <= bhi; y++ {
								var expected float64
								switch op {
								case "+":
									expected = float64(x) + float64(y)
								case "-":
									expected = float64(x) - float64(y)
								case "*":
									expected = float64(x) * float64(y)
								case "%":
									expected = math.Mod(float64(x), float64(y))
								}
								if expected < float64(r.lo) || expected > float64(r.hi) || expected == 0 && math.Signbit(expected) {
									t.Fatalf("%d %s %d = %v outside %+v", x, op, y, expected, r)
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestNumberRangeLoopControlFlows(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
  private stop(s:string):number {
   let n=0; while(n<s.length) {if(s.length>0) {n=2147483648; break;} n=0;} return n;
  }
  private keep(s:string):number {
   let n=0; while(n<s.length) {if(s.length>0) {n=2147483648; continue;} n=0;} return n;
  }
  run(s:string):number {return this.stop(s)+this.keep(s);}
 }`}, []string{"probe.ts"})
	c := rangeClass(t, p, ".Probe")
	for _, m := range c.Methods {
		if m.Name == "stop" || m.Name == "keep" {
			if m.Result.Kind != hir.I64 {
				t.Fatalf("%s lost the control-flow exit value: %s", m.Name, m.Result.Kind)
			}
		}
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestNumberRangeGuardSamplesFieldBeforeCall(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
  private n:number=0;
  private change():number {this.n=2147483648; return 1;}
  private check():number {if(this.n<this.change()) {return this.n;} return 0;}
  private checkLazy():number {if(this.n<2 && this.change()>0) {return this.n;} return 0;}
  run():number {return this.check()+this.checkLazy();}
 }`}, []string{"probe.ts"})
	c := rangeClass(t, p, ".Probe")
	for _, m := range c.Methods {
		if (m.Name == "check" || m.Name == "checkLazy") && m.Result.Kind != hir.I64 {
			t.Fatalf("right-side call invalidated the sampled field: %s", m.Result.Kind)
		}
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestNumberRangeFieldInitializationDominatesReads(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class EarlyRead {
  private n:number;
  constructor() {const previous=this.n; this.n=0;}
  run():number {return this.n;}
 }
 export class OtherWrite {
  private n:number;
  constructor(other:OtherWrite) {other.n=0;}
  run():number {return this.n;}
 }
 export class RHSCall {
  private n:number;
  constructor() {this.n=this.read();}
  private read():number {if(this.n===0) {return 1;} return 2;}
  run():number {return this.n;}
 }`}, []string{"probe.ts"})
	for _, name := range []string{".EarlyRead", ".RHSCall", ".OtherWrite"} {
		c := rangeClass(t, p, name)
		for _, f := range c.Fields {
			if f.Name == "n" && f.Type.Kind != hir.Number {
				t.Fatalf("%s field initialized after a read/call narrowed to %s", name, f.Type.Kind)
			}
		}
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestNumberRangeImmutableLengthHasSingleInitialization(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
  private readonly raw:string;
  private offset:number;
  constructor(s:string) {this.raw=s; this.offset=this.raw.length; this.raw="";}
  private advance():void {if(this.offset===this.raw.length) {return;} this.offset++;}
  run():number {this.advance(); return this.offset;}
 }`}, []string{"probe.ts"})
	c := rangeClass(t, p, ".Probe")
	for _, f := range c.Fields {
		if f.Name == "offset" && f.Type.Kind != hir.Number {
			t.Fatalf("reassigned length source gave an invalid cap: %s", f.Type.Kind)
		}
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestNumberBoundaryAliasDoesNotCaptureExistingLocal(t *testing.T) {
	self := &hir.Expr{Kind: hir.This, Type: hir.Ref("Probe")}
	field := func() *hir.Expr {
		return &hir.Expr{Kind: hir.FieldGet, Name: "start", Type: hir.T(hir.Number), X: self}
	}
	write := &hir.Stmt{Kind: hir.Assign, X: field(), Y: hir.V("offset", hir.T(hir.Number))}
	m := &hir.Method{Name: "add", Params: []hir.Param{{Name: "offset", Type: hir.T(hir.Number)}}, Result: hir.T(hir.Void), Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "range_param_1", Type: hir.T(hir.Number), X: hir.L(hir.T(hir.Number), 99)}, write,
		&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "number.index", Type: hir.T(hir.I32), X: field()}},
	)}
	c := &hir.Class{Name: "Probe", Fields: []hir.Field{{Name: "start", Type: hir.T(hir.Number), Private: true}}, Methods: []*hir.Method{m}, Ctor: &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: field(), Y: hir.L(hir.T(hir.Number), 0)})}}
	symbol := &ast.Symbol{}
	l := &lowerer{out: &hir.Program{Classes: []*hir.Class{c}}, localSymbols: map[*hir.Expr]*ast.Symbol{write.Y: symbol}, paramSymbols: map[*hir.Method]map[string]*ast.Symbol{m: {"offset": symbol}}}
	l.checkIndexFieldBoundaries()
	if m.Body.List[0].Name == "range_param_1" || write.Y.Name != m.Body.List[0].Name {
		t.Fatal("checked alias captured the user's local instead of the public parameter")
	}
}

func TestNumberRangeIndirectFieldWrites(t *testing.T) {
	for _, value := range []string{"2147483648", "0.5"} {
		for _, receiver := range []string{"alias", "other"} {
			t.Run(value+"/"+receiver, func(t *testing.T) {
				p := lowerStatementsProbe(t, map[string]string{"probe.ts": fmt.Sprintf(`
export class Probe {
 private n=0;
 run(other:Probe):number {this.n=0; const alias=this; %s.n=%s; const observed=this.n; return observed;}
}`, receiver, value)}, []string{"probe.ts"})
				m := rangeClass(t, p, ".Probe").Methods[0]
				want := hir.I64
				if value == "0.5" {
					want = hir.Number
				}
				if got := rangeLocals(m)["observed"]; got != want {
					t.Fatalf("stale alias field proof: %s, want %s", got, want)
				}
				if _, err := abap.Emit(p); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestNumberBoundaryShadowedParameter(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
export class Probe {
 private start=0;
 private raw="abc";
 add(offset:number):number {this.start=offset; {const offset=7; return offset;}}
 get():string {return this.raw.charAt(this.start);}
}`}, []string{"probe.ts"})
	m := rangeClass(t, p, ".Probe").Methods[0]
	var returned *hir.Expr
	var shadow *hir.Stmt
	walkNumberStmt(m.Body, func(s *hir.Stmt) {
		if s.Kind == hir.VarDecl && s.X != nil && s.X.Kind == hir.Lit {
			shadow = s
		}
		if s.Kind == hir.Return {
			returned = unwrapNumber(s.X)
		}
	}, func(e *hir.Expr) {})
	for returned != nil && returned.Kind == hir.NumericConvert {
		returned = returned.X
	}
	if returned == nil || returned.Kind != hir.Local || shadow == nil || returned.Name != shadow.Name || strings.HasPrefix(returned.Name, "range_param_") {
		t.Fatalf("shadowed return captured checked parameter: %+v", returned)
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestNumberRangeFieldEffectsThroughCallsAndClosures(t *testing.T) {
	for _, change := range []string{
		`const alias=this; alias.change();`,
		`other.change();`,
		`const mutate=()=>{this.n=0.5;}; mutate();`,
		`const alias=this; const mutate=()=>{alias.n=0.5;}; mutate();`,
	} {
		t.Run(change, func(t *testing.T) {
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": `export class Probe {
private n=0;
private change():void {this.n=0.5;}
run(other:Probe):number {this.n=0; ` + change + ` const observed=this.n;return observed;}
}`}, []string{"probe.ts"})
			for _, m := range rangeClass(t, p, ".Probe").Methods {
				if m.Name == "run" && rangeLocals(m)["observed"] != hir.Number {
					t.Fatal("call retained stale field fact")
				}
			}
			if _, err := abap.Emit(p); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNumberRangeInheritedAliasWrite(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `export class Probe {
private n=0;
run(other:Derived):number {this.n=0;other.n=0.5;const observed=this.n;return observed;}
}
export class Derived extends Probe {}`}, []string{"probe.ts"})
	m := rangeClass(t, p, ".Probe").Methods[0]
	if rangeLocals(m)["observed"] != hir.Number {
		t.Fatal("inherited alias write missed declaring-class summary")
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestNumberBoundaryShadowedAssignment(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `export class Probe {
private start=0;
private raw="abc";
add(offset:number):number {this.start=offset;{let offset=7;offset=8;}return offset;}
get():string {return this.raw.charAt(this.start);}
}`}, []string{"probe.ts"})
	m := rangeClass(t, p, ".Probe").Methods[0]
	checked := false
	walkNumberStmt(m.Body, func(s *hir.Stmt) {}, func(e *hir.Expr) {
		if e.Kind == hir.CheckedNumericConvert {
			checked = true
		}
	})
	if !checked {
		t.Fatal("shadowed assignment treated as parameter mutation")
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestNumberBoundaryAssignedParameter(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `export class Probe {
private start=0;
private raw="abc";
add(offset:number):number {this.start=offset;offset=0.5;return offset;}
get():string {return this.raw.charAt(this.start);}
}`}, []string{"probe.ts"})
	m := rangeClass(t, p, ".Probe").Methods[0]
	walkNumberStmt(m.Body, func(s *hir.Stmt) {}, func(e *hir.Expr) {
		if e.Kind == hir.CheckedNumericConvert {
			t.Fatal("mutated parameter acquired a checked alias")
		}
	})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestNumberRangeShadowedClosureCapture(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `export class Probe {
run(n:number):number {{const n=7;const read=()=>n;return read();}}
}`}, []string{"probe.ts"})
	var m *hir.Method
	for _, candidate := range rangeClass(t, p, ".Probe").Methods {
		if candidate.Name == "run" {
			m = candidate
		}
	}
	walkNumberStmt(m.Body, func(s *hir.Stmt) {}, func(e *hir.Expr) {
		if e.Kind != hir.VirtualCall || !strings.HasPrefix(e.Name, "fn_") {
			return
		}
		if len(e.Args) != 1 {
			t.Fatal("missing capture")
		}
		arg := e.Args[0]
		for arg.Kind == hir.NumericConvert {
			arg = arg.X
		}
		if arg.Kind != hir.Local || arg.Name == "n" {
			t.Fatalf("shadowed closure captured outer parameter: %+v", arg)
		}
	})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

// This fixture exercises observable results on both ABAP runtimes when exported.
func TestNumberFix1Semantics(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `
export class Overflow {private n=0;run():number {this.n=0;const alias=this;alias.n=2147483648;return this.n;}}
export class Fraction {private n=0;run():number {this.n=0;const alias=this;alias.n=0.5;return this.n;}}
export class Shadow {private start=0;private raw="abc";add(offset:number):number {this.start=offset;{const offset=7;return offset;}}get():string {return this.raw.charAt(this.start);}}
`}, []string{"probe.ts"})
	files, names, err := abap.EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	source.WriteString("CLASS ltcl_fix1 DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS semantics FOR TESTING.\nENDCLASS.\nCLASS ltcl_fix1 IMPLEMENTATION.\nMETHOD semantics.\nDATA actual TYPE f.\nDATA expected TYPE f.\n")
	for i, probe := range []struct{ name, value, method, args string }{
		{"Overflow", "2147483648", "run", ""},
		{"Fraction", "0.5", "run", ""},
		{"Shadow", "7", "add", names.Get("param.offset") + " = CONV f( 3 )"},
	} {
		class := names.Get(rangeClass(t, p, "."+probe.name).Name)
		fmt.Fprintf(&source, "DATA(ref%d) = NEW %s( ).\ncl_abap_unit_assert=>assert_equals( act = xsdbool( ref%d IS INSTANCE OF %s ) exp = abap_true ).\nactual = ref%d->%s( %s ).\nexpected = '%s'.\ncl_abap_unit_assert=>assert_equals( act = actual exp = expected ).\n", i, class, i, class, i, names.Get("member."+probe.method), probe.args, probe.value)
	}
	source.WriteString("ENDMETHOD.\nENDCLASS.\n")
	class := names.Get(rangeClass(t, p, ".Overflow").Name)
	files[class+".clas.testclasses.abap"] = source.String()
	if out := os.Getenv("ABAPITI_FIX1_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
