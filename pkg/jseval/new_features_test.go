package jseval

import (
	"strings"
	"testing"
)

func TestNewFeatures(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		// Ternary
		{"ternary-true", `console.log(true ? "yes" : "no")`, "yes"},
		{"ternary-false", `console.log(false ? "yes" : "no")`, "no"},
		{"ternary-expr", `let x = 5; console.log(x > 3 ? "big" : "small")`, "big"},
		{"ternary-nested", `console.log(true ? false ? "a" : "b" : "c")`, "b"},

		// Arrow functions
		{"arrow-expr", `let f = (a, b) => a + b; console.log(f(3, 4))`, "7"},
		{"arrow-body", `let f = (x) => { return x * 2; }; console.log(f(21))`, "42"},
		{"arrow-single", `let f = x => x + 1; console.log(f(9))`, "10"},
		{"arrow-no-params", `let f = () => 42; console.log(f())`, "42"},
		{"arrow-closure", `function make(n) { return () => n; } console.log(make(7)())`, "7"},

		// Throw / try / catch
		{"try-no-throw", `try { console.log("ok"); } catch(e) { console.log("err"); }`, "ok"},
		{"try-catch", `try { throw "boom"; } catch(e) { console.log("caught:" + e); }`, "caught:boom"},
		{"try-catch-value", `let r = ""; try { throw "x"; } catch(e) { r = e; } console.log(r)`, "x"},
		{"throw-number", `try { throw 42; } catch(e) { console.log(e); }`, "42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Eval(tc.code)
			if err != nil { t.Fatalf("error: %v", err) }
			got := strings.TrimSpace(out)
			if got != tc.want { t.Errorf("got %q, want %q", got, tc.want) }
		})
	}
}

func TestForOfAndTemplates(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		// for...of
		{"for-of-array", `let a = [1,2,3]; let s = 0; for (const x of a) { s = s + x; } console.log(s)`, "6"},
		{"for-of-string-arr", `let a = ["a","b","c"]; let s = ""; for (let x of a) { s = s + x; } console.log(s)`, "abc"},
		{"for-of-break", `let a = [1,2,3,4,5]; let s = 0; for (const x of a) { if (x === 4) break; s = s + x; } console.log(s)`, "6"},
		// for...in
		{"for-in-obj", `let o = {a: 1, b: 2}; let keys = ""; for (const k in o) { keys = keys + k; } console.log(keys.length)`, "2"},
		// Template literals
		{"template-simple", "console.log(`hello`)", "hello"},
		{"template-expr", "let x = 42; console.log(`val=${x}`)", "val=42"},
		{"template-multi", "let a = 1; let b = 2; console.log(`${a}+${b}=${a+b}`)", "1+2=3"},
		{"template-nested", "console.log(`x=${true ? `yes` : `no`}`)", "x=yes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Eval(tc.code)
			if err != nil { t.Fatalf("error: %v", err) }
			got := strings.TrimSpace(out)
			if got != tc.want { t.Errorf("got %q, want %q", got, tc.want) }
		})
	}
}
