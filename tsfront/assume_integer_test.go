package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func integerProbe(t *testing.T, source string) *Program {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{"probe.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"files":["probe.ts"]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAssumeIntegerContract(t *testing.T) {
	p := integerProbe(t, `export class Probe { static run(n:number):number { const tiny=3; return n+tiny; } }`)
	options := LowerOptions{AssumeOnlyIntegerCalculations: true}
	prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, options)
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	c := rangeClass(t, prog, ".Probe")
	if c.Methods[0].Params[0].Type.Kind != hir.I64 || c.Methods[0].Result.Kind != hir.I64 {
		t.Fatal(hir.Dump(prog))
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	// Explicit false overrides the test environment and reproduces the default.
	t.Setenv("ABAPITI_ASSUME_INT", "")
	normal, _, err := p.Lower([]string{"probe.ts"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	explicit, _, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if hir.Dump(normal) != hir.Dump(explicit) {
		t.Fatal("explicit default changed")
	}
}

func TestAssumeIntegerBlockingSites(t *testing.T) {
	for _, expression := range []string{"1000000000000000128", "9007199254740992", "0x20000000000000", "0b100000000000000000000000000000000000000000000000000000", "0o400000000000000000", "9_007_199_254_740_992", "5/2", "1.5", "1e3", "n**-1", "n**n", "Math.sqrt(n)", "Math.log(n)", "Math.pow(n,2)", "Math.random()", "Math.sin(n)", "Math.ceil(n)", "Math.floor(n)", "Math.round(n)", "Math.trunc(n)", `parseFloat("1.5")`, `Number("1.5")`, "n.toFixed(2)"} {
		t.Run(expression, func(t *testing.T) {
			p := integerProbe(t, `export class Probe { run(n:number) { return `+expression+`; } }`)
			_, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{AssumeOnlyIntegerCalculations: true})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, d := range ds {
				found = found || d.Category == "assume-integer" && strings.Contains(d.Message, expression)
			}
			if !found {
				t.Fatal("missing blocking integer diagnostic", ds)
			}
		})
	}
}

func TestAssumeIntegerException(t *testing.T) {
	source := `export class Probe { run(n:number):number { return n/2; } }`
	entry := IntegerException{File: "probe.ts", Start: strings.Index(source, "n/2"), SHA256: overrides.Fingerprint("n/2"), Reason: "caller supplies an even count; reject a fractional result"}
	p := integerProbe(t, source)
	options := LowerOptions{AssumeOnlyIntegerCalculations: true, IntegerExceptions: []IntegerException{entry}}
	prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, options)
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	files, err := abap.Emit(prog)
	if err != nil {
		t.Fatal(err)
	}
	checked := false
	for _, source := range files {
		checked = checked || strings.Contains(source, "<> trunc(") && strings.Contains(source, "RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds")
	}
	if !checked {
		t.Fatal("missing exact exception boundary")
	}
	for _, mutation := range []string{strings.Replace(source, "n/2", "n/3", 1), strings.Replace(source, "n/2", "n+2", 1)} {
		_, _, err := integerProbe(t, mutation).LowerWithOptions([]string{"probe.ts"}, options)
		if err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatal("stale exception accepted", err)
		}
	}
	options.IntegerExceptions[0].Reason = ""
	if _, _, err := p.LowerWithOptions([]string{"probe.ts"}, options); err == nil {
		t.Fatal("missing reason accepted")
	}
}

func TestAssumeIntegerClosureInventory(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	for name, lower := range map[string]func(*testing.T) (*hir.Program, []LowerDiagnostic){"lexer": lowerClosure, "statements": lowerStmtsClosure, "structures": func(t *testing.T) (*hir.Program, []LowerDiagnostic) {
		t.Setenv("STRUCTURES_EMIT", "1")
		return lowerStmtsClosure(t)
	}} {
		t.Run(name, func(t *testing.T) {
			prog, ds := lower(t)
			if hasBlocking(ds) {
				t.Fatal(ds)
			}
			for _, d := range ds {
				if d.Category == "note-integer-exception" {
					t.Fatal("expected empty exception inventory", d)
				}
			}
			if errs := hir.Verify(prog); len(errs) != 0 {
				t.Fatal(errs)
			}
			numberExpressions := 0
			for _, c := range prog.Classes {
				for _, m := range numberMethods(c) {
					walkNumberStmt(m.Body, func(*hir.Stmt) {}, func(e *hir.Expr) {
						if e.Type.Kind == hir.Number {
							numberExpressions++
						}
					})
				}
			}
			if numberExpressions != 0 {
				t.Fatalf("%s retains %d Number expressions without exceptions", name, numberExpressions)
			}
			t.Logf("%s: zero floating exceptions, zero Number expressions", name)
		})
	}
}

func TestAssumeIntegerRuntime(t *testing.T) {
	source := `export class Probe {
 static half(n:number):number { return n/2; }
 static double(n:number):number { return n/0.5; }
 static pair(n:number):number { return n/2+n/2; }
 static add(n:number,m:number):number { return n+m; }
 static sub(n:number,m:number):number { return n-m; }
 static mul(n:number,m:number):number { return n*m; }
 static neg(n:number):number { return -n; }
 static constantOverflow():number { return 94906267*94906267; }
 static cancel(n:number):number { return n+1-n; }
 static boundary():number { return 9007199254740991; }
 static render(n:number):string { return n.toString(); }
 }`
	options := LowerOptions{AssumeOnlyIntegerCalculations: true, IntegerExceptions: []IntegerException{
		{File: "probe.ts", Start: strings.Index(source, "n/2"), SHA256: overrides.Fingerprint("n/2"), Reason: "even integer counts; odd counts must raise"},
		{File: "probe.ts", Start: strings.Index(source, "n/0.5"), SHA256: overrides.Fingerprint("n/0.5"), Reason: "division by half gives an integral result for safe small counts"},
		{File: "probe.ts", Start: strings.Index(source, "0.5"), SHA256: overrides.Fingerprint("0.5"), Reason: "fractional operand stays float inside the approved division"},
	}}
	pairStart := strings.Index(source, "static pair")
	firstHalf := pairStart + strings.Index(source[pairStart:], "n/2")
	secondHalf := firstHalf + 3 + strings.Index(source[firstHalf+3:], "n/2")
	for _, start := range []int{firstHalf, secondHalf} {
		options.IntegerExceptions = append(options.IntegerExceptions, IntegerException{File: "probe.ts", Start: start, SHA256: overrides.Fingerprint("n/2"), Reason: "two half-counts sum to the original integer"})
	}
	prog, ds, err := integerProbe(t, source).LowerWithOptions([]string{"probe.ts"}, options)
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	driver := `CLASS ltcl_integer DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
 PRIVATE SECTION.
 METHODS boundary FOR TESTING.
 METHODS overflow FOR TESTING.
 METHODS product FOR TESTING.
 METHODS cancel FOR TESTING.
 ENDCLASS.
 CLASS ltcl_integer IMPLEMENTATION.
 METHOD boundary.
 DATA actual TYPE int8.
 DATA text TYPE string.
 DATA hi TYPE int8.
 hi = 900719925.
 hi = hi * 10000000.
 hi = hi + 4740991.
 actual = CLASSNAME=>HALF( N = 6 ).
 cl_abap_unit_assert=>assert_equals( act = actual exp = 3 ).
 actual = CLASSNAME=>DOUBLE( N = 4 ).
 cl_abap_unit_assert=>assert_equals( act = actual exp = 8 ).
 actual = CLASSNAME=>PAIR( N = 5 ).
 cl_abap_unit_assert=>assert_equals( act = actual exp = 5 ).
 TRY.
 actual = CLASSNAME=>HALF( N = 5 ).
 cl_abap_unit_assert=>fail( msg = 'fraction accepted' ).
 CATCH cx_sy_range_out_of_bounds.
 ENDTRY.
 actual = CLASSNAME=>BOUNDARY( ).
 cl_abap_unit_assert=>assert_equals( act = actual exp = hi ).
 text = CLASSNAME=>RENDER( N = hi ).
 cl_abap_unit_assert=>assert_equals( act = text exp = '9007199254740991' ).
 actual = CLASSNAME=>SUB( N = hi M = 1 ).
 text = CLASSNAME=>RENDER( N = actual ).
 cl_abap_unit_assert=>assert_equals( act = text exp = '9007199254740990' ).
 ENDMETHOD.
 METHOD overflow.
 DATA actual TYPE int8.
 DATA hi TYPE int8.
 hi = 900719925.
 hi = hi * 10000000.
 hi = hi + 4740991.
 DATA lo TYPE int8.
 lo = 0 - hi.
 actual = CLASSNAME=>NEG( N = lo ).
 cl_abap_unit_assert=>assert_equals( act = actual exp = hi ).
 actual = CLASSNAME=>NEG( N = hi ).
 cl_abap_unit_assert=>assert_equals( act = actual exp = lo ).
 TRY.
 actual = CLASSNAME=>ADD( N = hi M = 1 ).
 cl_abap_unit_assert=>fail( msg = 'addition overflow accepted' ).
 CATCH cx_sy_arithmetic_overflow.
 ENDTRY.
 TRY.
 actual = CLASSNAME=>SUB( N = lo M = 1 ).
 cl_abap_unit_assert=>fail( msg = 'subtraction overflow accepted' ).
 CATCH cx_sy_arithmetic_overflow.
 ENDTRY.
 TRY.
 actual = CLASSNAME=>MUL( N = hi M = 2 ).
 cl_abap_unit_assert=>fail( msg = 'multiplication overflow accepted' ).
 CATCH cx_sy_arithmetic_overflow.
 ENDTRY.
 TRY.
 actual = CLASSNAME=>NEG( N = lo - 1 ).
 cl_abap_unit_assert=>fail( msg = 'negation overflow accepted' ).
 CATCH cx_sy_arithmetic_overflow.
 ENDTRY.
 ENDMETHOD.
 METHOD product.
 DATA actual TYPE int8.
 TRY.
 actual = CLASSNAME=>CONSTOVERFLOW( ).
 cl_abap_unit_assert=>fail( msg = 'unsafe product accepted' ).
 CATCH cx_sy_arithmetic_overflow.
 ENDTRY.
 ENDMETHOD.
 METHOD cancel.
 DATA actual TYPE int8.
 DATA hi TYPE int8.
 hi = 900719925.
 hi = hi * 10000000.
 hi = hi + 4740991.
 TRY.
 actual = CLASSNAME=>CANCEL( N = hi ).
 cl_abap_unit_assert=>fail( msg = 'unsafe intermediate accepted' ).
 CATCH cx_sy_arithmetic_overflow.
 ENDTRY.
 ENDMETHOD.
 ENDCLASS.
 `
	for from, to := range map[string]string{"CLASSNAME": names.Get("probe.ts.Probe"), "HALF": names.Get("member.half"), "DOUBLE": names.Get("member.double"), "PAIR": names.Get("member.pair"), "ADD": names.Get("member.add"), "SUB": names.Get("member.sub"), "MUL": names.Get("member.mul"), "NEG": names.Get("member.neg"), "CONSTOVERFLOW": names.Get("member.constantOverflow"), "RENDER": names.Get("member.render"), "CANCEL": names.Get("member.cancel"), "BOUNDARY": names.Get("member.boundary"), "N =": names.Get("param.n") + " =", "M =": names.Get("param.m") + " ="} {
		driver = strings.ReplaceAll(driver, from, to)
	}
	files[names.Get("probe.ts.Probe")+".clas.testclasses.abap"] = driver
	if out := os.Getenv("ASSUME_INTEGER_OUT"); out != "" {
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

func TestAssumeIntegerCollectionsAndBuiltins(t *testing.T) {
	source := `export class Probe {
  static run(n:number, values:number[]):number { values.push(Number(n)); return Math.min(n, values[0]); }
 }`
	prog, ds, err := integerProbe(t, source).LowerWithOptions([]string{"probe.ts"}, LowerOptions{AssumeOnlyIntegerCalculations: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestAssumeIntegerLiteralExceptions(t *testing.T) {
	for _, expression := range []string{"0.5", "-0.5", "1e3"} {
		t.Run(expression, func(t *testing.T) {
			source := `export class Probe { run():number { return ` + expression + `; } }`
			span := strings.TrimPrefix(expression, "-")
			p := integerProbe(t, source)
			_, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{AssumeOnlyIntegerCalculations: true})
			if err != nil || !hasBlocking(ds) {
				t.Fatal("literal accepted without exception", err, ds)
			}
			options := LowerOptions{AssumeOnlyIntegerCalculations: true, IntegerExceptions: []IntegerException{{File: "probe.ts", Start: strings.Index(source, span), SHA256: overrides.Fingerprint(span), Reason: "literal boundary must reject a non-integral result"}}}
			prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, options)
			if err != nil || hasBlocking(ds) {
				t.Fatal(err, ds)
			}
			if errs := hir.Verify(prog); len(errs) != 0 {
				t.Fatal(errs)
			}
			found := false
			for _, c := range prog.Classes {
				for _, m := range numberMethods(c) {
					walkNumberStmt(m.Body, func(*hir.Stmt) {}, func(e *hir.Expr) {
						if e.Kind == hir.CheckedNumericConvert && e.X.Type.Kind == hir.Number {
							found = true
						}
					})
				}
			}
			if !found {
				t.Fatal("literal lost its exact floating boundary")
			}
			if _, err := abap.Emit(prog); err != nil {
				t.Fatal(err)
			}
		})
	}
}
