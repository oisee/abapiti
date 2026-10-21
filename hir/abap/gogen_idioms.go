package abap

import "github.com/oisee/abapiti/hir"

// The telemetry mapping uses elapsed microseconds; it cannot implement the
// epoch clock. Until a timestamp ABI exists ABAP refuses at the call site.
func init() {
	for _, op := range []string{"clock.epochMilliseconds"} {
		unsupportedOps[op] = true
	}
}

func (b *body) mapInsertions(x *hir.Expr, n string) bool {
	if x.Op != "map.insertions" {
		return false
	}
	b.line(n + " = " + b.expr(x.X) + "->insertions( ).")
	return true
}

// Dynamic graph objects carry identity in the box itself; native references
// carry it in oval. Primitive boxes never become weak collection keys.
func (b *body) weakKey(x *hir.Expr, n string) bool {
	if x.Op != "weak.lookupKey" && x.Op != "weak.writeKey" {
		return false
	}
	a := b.expr(x.X)
	d := b.e.name("runtime.dynamic") + "=>"
	b.mapLine("CLEAR " + n + ".\nIF " + a + " IS BOUND.\nCASE " + a + "->tag.\nWHEN " + d + "tag_ref.\n" + n + " = " + a + "->oval.\nWHEN " + d + "tag_array.\n" + n + " = " + a + ".\nWHEN " + d + "tag_class.\n" + n + " = " + a + "->cval.\nENDCASE.\nENDIF.")
	if x.Op == "weak.writeKey" {
		e := b.e
		e.errorUsed = true
		t := hir.Ref("builtin.TypeError")
		cls := e.name(t.Name)
		if e.classBy(t.Name) == nil {
			e.files[cls+".clas.abap"] = "CLASS " + cls + " DEFINITION PUBLIC INHERITING FROM " + e.name("builtin.Error") + " CREATE PUBLIC.\nPUBLIC SECTION.\nENDCLASS.\nCLASS " + cls + " IMPLEMENTATION.\nENDCLASS.\n"
		}
		b.line("IF " + n + " IS NOT BOUND.")
		message := b.temp(hir.T(hir.Optional, hir.T(hir.String)))
		b.mapLine(message + " = NEW #( ).\n" + message + "->has = abap_true.\n" + message + "->value = `Invalid value used as weak map key`.")
		value := b.temp(t)
		b.line(value + " = NEW " + cls + "( " + e.param("message") + " = " + message + " ).")
		wrapper := e.exception(t)
		if e.classBy(t.Name) == nil {
			e.files[wrapper+".clas.abap"] = "CLASS " + wrapper + " DEFINITION PUBLIC INHERITING FROM " + e.exception(hir.Ref("builtin.Error")) + " CREATE PUBLIC.\nPUBLIC SECTION.\nENDCLASS.\nCLASS " + wrapper + " IMPLEMENTATION.\nENDCLASS.\n"
		}
		err := b.rawTemp("REF TO " + wrapper)
		b.mapLine(err + " = NEW #( ).\n" + err + "->payload = " + value + ".")
		if e.promiseUsed {
			b.promisePayload(err+"->source_value", value, t)
		}
		b.mapLine("RAISE EXCEPTION " + err + ".\nENDIF.")
	}
	return true
}
