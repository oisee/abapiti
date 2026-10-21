package abap

import (
	"github.com/oisee/abapiti/hir"
)

func (e *emitter) usesReviewedOp(op string) bool {
	if e.reviewedOps == nil {
		e.reviewedOps = map[string]bool{}
		for used := range hir.UsedRuntimeOps(e.p) {
			e.reviewedOps[used] = true
		}
	}
	return e.reviewedOps[op]
}
func (e *emitter) collectionProbeUsed() bool {
	return e.usesReviewedOp("dynamic.isMap") || e.usesReviewedOp("dynamic.isSet") || e.usesReviewedOp("dynamic.values")
}
func (e *emitter) collectionProbeRuntime() string {
	name := e.name("collection.probe")
	e.files[name+".intf.abap"] = "INTERFACE " + name + " PUBLIC.\nMETHODS kind RETURNING VALUE(result) TYPE i.\nMETHODS properties RETURNING VALUE(result) TYPE REF TO " + e.name("runtime.dynamic") + ".\nENDINTERFACE.\n"
	return name
}
func (e *emitter) collectionProbeDefinition(t hir.Type) (string, string) {
	if !e.collectionProbeUsed() {
		return "", ""
	}
	iface := e.collectionProbeRuntime()
	kind := "result = 2.\n"
	if t.Kind == hir.OrderedMap {
		kind = "IF js_map = abap_true.\nresult = 1.\nENDIF.\n"
	}
	impl := "METHOD " + iface + "~kind.\n" + kind + "ENDMETHOD.\nMETHOD " + iface + "~properties.\nCREATE OBJECT result.\nresult->tag = " + e.name("runtime.dynamic") + "=>tag_object.\n"
	if t.Kind == hir.OrderedMap && t.Args[0].Kind == hir.String {
		b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}}
		box := b.temp(hir.T(hir.Dynamic))
		b.line("IF js_map = abap_false.\nLOOP AT entries INTO DATA(property).")
		b.boxDynamic(box, "property-v", t.Args[1])
		b.line("APPEND VALUE #( k = property-k v = " + box + " ) TO result->entries.\nENDLOOP.\nENDIF.")
		impl += b.constantDeclarations() + b.code.String()
	}
	impl += "ENDMETHOD.\n"
	return "INTERFACES " + iface + ".\n", impl
}
func (b *body) u3DynamicRuntime(x *hir.Expr, n string) bool {
	if x.Op != "dynamic.isMap" && x.Op != "dynamic.isSet" && x.Op != "dynamic.values" {
		return false
	}
	e := b.e
	a := b.expr(x.X)
	d := e.name("runtime.dynamic")
	iface := e.collectionProbeRuntime()
	probe := b.rawTemp("REF TO " + iface)
	b.line("CLEAR " + probe + ".")
	b.line("IF " + a + " IS BOUND AND " + a + "->tag = " + d + "=>tag_ref.")
	b.line("TRY.\n" + probe + " ?= " + a + "->oval.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nENDIF.")
	if x.Op != "dynamic.values" {
		want := "1"
		if x.Op == "dynamic.isSet" {
			want = "2"
		}
		b.line("CLEAR " + n + ".\nIF " + probe + " IS BOUND.\n" + n + " = xsdbool( " + probe + "->kind( ) = " + want + " ).\nENDIF.")
		return true
	}
	// Snapshot keys before reading values. Native record maps first expose their
	// properties as a graph; live objects and arrays retain their own adapters.
	source := b.temp(hir.T(hir.Dynamic))
	b.line(source + " = " + a + ".")
	b.line("IF " + source + " IS NOT BOUND OR " + source + "->tag = " + d + "=>tag_null.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.")
	b.line("IF " + probe + " IS BOUND.\n" + source + " = " + probe + "->properties( ).\nENDIF.")
	b.line("CREATE OBJECT " + n + ".")
	b.line("CASE " + source + "->tag.\nWHEN " + d + "=>tag_string.")
	cursor := b.temp(hir.T(hir.I32))
	box := b.temp(hir.T(hir.Dynamic))
	ch := b.temp(hir.T(hir.String))
	b.line("WHILE " + cursor + " < strlen( " + source + "->sval ).")
	b.line(ch + " = " + source + "->sval+" + cursor + "(1).")
	b.boxDynamic(box, ch, hir.T(hir.String))
	b.line("APPEND " + box + " TO " + n + "->items.")
	b.line(cursor + " = " + cursor + " + 1.\nENDWHILE.")
	b.line("WHEN " + d + "=>tag_number OR " + d + "=>tag_boolean OR " + d + "=>tag_bigint OR " + d + "=>tag_class.")
	b.line("WHEN OTHERS.")
	e.ownKeysRuntime()
	keys := b.temp(hir.T(hir.Array, hir.T(hir.String)))
	key := b.temp(hir.T(hir.String))
	b.line(keys + " = " + e.name("runtime.ownKeys") + "=>keys( " + source + " ).")
	b.line("LOOP AT " + keys + "->items INTO " + key + ".")
	b.line(box + " = " + source + "->get( " + key + " ).")
	b.line("APPEND " + box + " TO " + n + "->items.\nENDLOOP.\nENDCASE.")
	return true
}
