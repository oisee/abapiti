package abap

import "github.com/oisee/abapiti/hir"

func (b *body) u3HostRuntime(x *hir.Expr, n string) bool {
	if x.Op != "host.getenv" && x.Op != "host.stderrWrite" {
		return false
	}
	for _, binding := range b.e.p.Externs {
		sig := binding.Signature
		if sig.ABAPClass == "" || len(sig.Params) != 1 || sig.Params[0].Kind != hir.String || sig.Rest || len(sig.ParamNames) != 1 {
			continue
		}
		env := x.Op == "host.getenv" && binding.Name == "env" && binding.Result.Equal(hir.T(hir.Optional, hir.T(hir.String)))
		stderr := x.Op == "host.stderrWrite" && (binding.Name == "writeStderr" || binding.Name == "stderrWrite") && binding.Result.Kind == hir.Void
		if !env && !stderr {
			continue
		}
		call := &hir.Expr{Node: x.Node, Kind: hir.ExternCall, Owner: binding.Module, Name: binding.Name, Type: binding.Result, Extern: &sig, Args: []*hir.Expr{x.X}}
		method, parameter := externABAPIdentifier(binding.Name), externABAPIdentifier(sig.ParamNames[0])
		if !externABAPName.MatchString(method) || !externABAPName.MatchString(parameter) || parameter == "result" {
			b.e.refuse(hir.Error{Node: x.Node, Message: "invalid ABAP host capability identifier"})
			return true
		}
		argument := b.expr(x.X)
		invoke := sig.ABAPClass + "=>" + method + "( " + parameter + " = " + argument + " )"
		if env {
			raw := b.externTemp("REF TO string")
			b.line(raw + " = " + invoke + ".")
			b.line(n + " = " + b.externResult(call, raw) + ".")
		} else {
			b.line(invoke + ".")
		}
		return true
	}
	// ABAP has no process environment. Without a declared env host capability,
	// undefined is represented by an unbound optional, including on loop reuse.
	// The hostio ABI has stdout but no stderr. Dropping diagnostic stderr keeps
	// the reference stdout byte stream unchanged; never redirect it to stdout.
	b.expr(x.X)
	if x.Op == "host.getenv" {
		b.line("CLEAR " + n + ".")
	}
	return true
}
