package abap

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestFiniteParseIntLiteralNodeOracle(t *testing.T) {
	inputs := []string{" 42abc", "-7", "+3", "\ufeff\u2007+12tail", "9007199254740991", "0xFF", "", "abc", "Infinity", strings.Repeat("9", 400), "9007199254740992", "-0", "\u008542"}
	raw, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(s=>{const n=parseInt(s,10);return {allowed:Number.isSafeInteger(n)&&!Object.is(n,-0),value:n}})))`, string(raw)).CombinedOutput()
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
	for i, input := range inputs {
		value, ok := finiteParseIntLiteral(hir.L(hir.T(hir.String), input))
		if ok != oracle[i].Allowed || ok && value != oracle[i].Value {
			t.Fatalf("input %q: Go %v %v Node %+v", input, value, ok, oracle[i])
		}
	}
}

func TestABAPNonfiniteParseIntEmissionBoundary(t *testing.T) {
	str := hir.T(hir.String)
	number := hir.T(hir.Number)
	for _, input := range []*hir.Expr{hir.L(str, "42x"), hir.L(str, ""), hir.L(str, "-0"), hir.V("input", str)} {
		parsed := &hir.Expr{Kind: hir.RuntimeOp, Op: "string.parseIntNumber", X: input, Type: number}
		p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "run", Static: true, Params: []hir.Param{{Name: "input", Type: str}}, Result: number, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: parsed})}}}}}
		_, err := Emit(p)
		want := input.Kind == hir.Lit && input.Value == "42x"
		if (err == nil) != want || err != nil && !strings.Contains(err.Error(), "finite safe-integer string literal") {
			t.Fatalf("%v: %v", input, err)
		}
	}
}
