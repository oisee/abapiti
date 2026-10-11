package abap

import "github.com/oisee/abapiti/hir"

func (e *emitter) promiseExceptionBase() string {
	name := e.name("exception.ts_throw")
	e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nDATA source_value TYPE " + e.typ(hir.T(hir.Dynamic)) + ".\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nENDCLASS.\n"
	return name
}
func (b *body) awaitPromise(x *hir.Expr, result string) {
	p := b.expr(x.X)
	if x.X.Type.Kind == hir.Optional {
		b.line("IF " + p + " IS BOUND.")
	} else {
		b.line("IF " + p + " IS NOT BOUND.")
		b.stmt(&hir.Stmt{Kind: hir.Trap, Name: "await provider did not return a certified settled promise"})
		b.line("ENDIF.")
	}
	b.line("IF " + p + "->rejection IS BOUND.")
	b.line("RAISE EXCEPTION " + p + "->rejection.")
	b.line("ENDIF.")
	if x.Type.Kind != hir.Void {
		b.line(result + " = " + p + "->value.")
	}
	if x.X.Type.Kind == hir.Optional {
		b.line("ENDIF.")
	}
}

// Integral payload specialization must not change the tagged JS number seen by
// an async catch. A source-proven safe int8 has an exact binary64 representation.
func (b *body) promisePayload(target, value string, t hir.Type) {
	if t.Kind == hir.I64 {
		n := b.temp(hir.T(hir.Number))
		b.line(n + " = " + value + ".")
		value = n
		t = hir.T(hir.Number)
	}
	b.boxDynamic(target, value, t)
}
