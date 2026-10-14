package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Boxed string-key records keep their native map storage. Reads box the actual
// value (including constructor descriptors); writes update that same map.
// The adapter does not certify class layouts, own-key presence, or deletion.
func (e *emitter) liveRecordMapRuntime(t hir.Type) {
	if !e.types["runtime.liveView"] || t.Kind != hir.OrderedMap || t.Args[0].Kind != hir.String {
		return
	}
	name := e.name("runtime." + t.String())
	storage := e.name("view.storage")
	file := name + ".clas.abap"
	text := e.files[file]
	text = strings.Replace(text, "PUBLIC SECTION.\n", "PUBLIC SECTION.\nINTERFACES "+storage+".\n", 1)
	var code strings.Builder
	code.WriteString("METHOD " + storage + "~structural.\nENDMETHOD.\nMETHOD " + storage + "~source.\nENDMETHOD.\n")
	code.WriteString("METHOD " + storage + "~has.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_record_map.go: has: native record-map presence not implemented`.\nENDMETHOD.\nMETHOD " + storage + "~delete.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_record_map.go: delete: native record-map deletion not implemented`.\nENDMETHOD.\n")
	b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}}
	b.line("READ TABLE entries WITH KEY by_k COMPONENTS k = p0 INTO DATA(row).\nIF sy-subrc = 0.")
	b.boxDynamic("result", "row-v", t.Args[1])
	b.line("ENDIF.")
	code.WriteString("METHOD " + storage + "~get.\n" + b.constantDeclarations() + b.code.String() + "ENDMETHOD.\n")
	b = &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}}
	value := t.Args[1]
	v := "p1"
	if value.Kind != hir.Dynamic {
		b.line("IF p1 IS NOT BOUND.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_record_map.go: put: undefined cannot inhabit required map value`.\nENDIF.")
		switch value.Kind {
		case hir.String, hir.Number, hir.Bool, hir.ClassValue, hir.ClassRef, hir.InterfaceRef, hir.Array, hir.OrderedMap, hir.OrderedSet:
			v = b.unbox("p1", value)
		default:
			// Retain a checked refusal for representations without an exact unbox ABI.
			b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `live_record_map.go: put: map value has no exact unbox ABI`.")
			v = b.temp(value)
		}
	}
	ignored := b.temp(t)
	b.line(ignored + " = set( p0 = p0 p1 = " + v + " ).")
	code.WriteString("METHOD " + storage + "~put.\n" + b.constantDeclarations() + b.code.String() + "ENDMETHOD.\n")
	e.files[file] = strings.TrimSuffix(text, "ENDCLASS.\n") + code.String() + "ENDCLASS.\n"
}
