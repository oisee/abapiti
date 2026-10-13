package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestFunctionCaptureValues(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"canonical parameter collision", `export function run(): number { const arg0 = 100; let fn = (x: number): number => arg0 + x; return fn(2); }`, "102\n"},
		{"default parameter", `export function run(): number { let fn = (value: number = 10): number => { const read = (x: number): number => {value = value + x; return value;}; return read(2); }; return fn(); }`, "12\n"},
		{"shadowed cells", `export function run(): number { let value = 1; let outer = (x: number): number => {value = value + x;return value;}; {let value = 10; let inner = (x: number): number => {value = value + x;return value;}; return outer(2) + inner(3);} }`, "16\n"},
		{"function identity", `export function run(): number { const fn = (x: number): number => x + 2; const alias = fn; return fn === alias ? 1 : 0; }`, "1\n"},
		{"parameter cell collision", `export function work(value: number): number { const value_cell = 100; const fn = (x: number): number => { value = value + x; return value + value_cell; }; return fn(2); } export function run(): number { return work(10); }`, "112\n"},
		{"lifted parameter", `export function run(): number { const make = (value: number): (x: number) => number => (x: number): number => {value = value + x; return value;}; const fn = make(10); return fn(2) + fn(3); }`, "27\n"},
		{"parameter names", `export function run(): number { let fn = (a: number): number => a + 1; fn = (b: number): number => b + 2; return fn(4); }`, "6\n"},
		{"parameter", `export function work(value: number): number { let fn = (x: number): number => { value = value + x; return value; }; return fn(2) + fn(3); } export function run(): number { return work(10); }`, "27\n"},
		{"returned closure", `export function make(value: number): (x: number) => number { return (x: number): number => { value = value + x; return value; }; } export function run(): number { const fn = make(10); return fn(2) + fn(3); }`, "27\n"},
		{"declaration", `export function run(): number { function parse(x: number): number { return x + 1; } let result = 0; const thunk = () => { result = parse(4); }; thunk(); return result; }`, "5\n"},
		{"mutable binding", `export function run(): number { let fn = (x: number): number => x + 1; let result = 0; const thunk = () => { result = fn(4); }; fn = (x: number): number => x + 3; thunk(); return result; }`, "7\n"},
		{"shared local", `export function run(): number { let value = 1; let fn = (x: number): number => { value = value + x; return value; }; value = 10; return fn(2) + fn(3); }`, "27\n"},
		{"callable field", `class Helper { constructor(public fix: (x: number) => number) {} } export function run(): number { const fn = (x: number): number => x + 2; const helper = new Helper(fn); return helper.fix(4); }`, "6\n"},
		{"instance method", `class Helper { fix(x: number): number { return x + 2; } } export function run(): number { const fix = new Helper().fix; let result = 0; const thunk = () => { result = fix(4); }; thunk(); return result; }`, "6\n"},
		{"static method", `class Helper { static fix(x: number): number { return x + 2; } } export function run(): number { const fix = Helper.fix; let result = 0; const thunk = () => { result = fix(4); }; thunk(); return result; }`, "6\n"},
		{"alias", `export function run(): number { const fn = (x: number): number => x + 2; const alias = fn; let result = 0; const thunk = () => { result = alias(4); }; thunk(); return result; }`, "6\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := lowerStatementsProbe(t, map[string]string{"probe.ts": c.source}, []string{"probe.ts"})
			for _, iface := range prog.Interfaces {
				if !strings.HasPrefix(iface.Name, "closure.function.") {
					continue
				}
				for _, class := range prog.Classes {
					for _, name := range class.Implements {
						if name != iface.Name {
							continue
						}
						for _, method := range class.Methods {
							if method.Name != "call" {
								continue
							}
							for i, param := range method.Params {
								if param.Name != iface.Methods[0].Params[i].Name {
									t.Fatal("callable ABAP parameter names differ")
								}
							}
						}
					}
				}
			}
			if _, err := emitABAP(t, prog); err != nil {
				t.Fatal(err)
			}
			if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.module", "run"))); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
