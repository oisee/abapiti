package tsfront

import "testing"

func TestStep4EmitTabIndentCaptureNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static indent(text:string):string{const m=/^(\t*)/.exec(text);return m?m[1]:"missing";}
 static run():string{return ["", "x", "\t\tx", " \tx", "\tx\n\ty", "\t😀"].map(s=>Probe.indent(s).length).join(",");}}`)
}
func TestStep4EmitVarBindingScannerNodeOracle(t *testing.T) {
	a20Oracle(t, `class Probe {static run():string {return ["var", "var(", "var (x", "var\n", "var\r\n", "xvar", "_var", "évar", "var\u00a0(", "var\u180e(", "var\ufeff(", "var x", "var)", "var\u2028"].map(s=>/\bvar\s*(?:\(|$)/.test(s)?"1":"0").join("");}}`)
}
