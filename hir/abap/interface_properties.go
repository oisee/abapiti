package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Interface slots are properties of the original object, not of an admission
// wrapper or the interface's method table. Select the actual native layout;
// its property table retains optional boxing, live sources and inheritance.
// Method reads retain the table's stable function identity; optional calls
// already arrive as class dispatch in HIR, preserving the original receiver.
func (b *body) interfacePropertyLookup(x *hir.Expr, n, a, key string) {
	e := b.e
	id := "property.lookup." + x.X.Type.Name
	name := e.name(id)
	if !e.types[id] {
		e.types[id] = true
		e.liveRuntime()
		var iface *hir.Interface
		for _, i := range e.p.Interfaces {
			if i.Name == x.X.Type.Name {
				iface = i
				break
			}
		}
		var out strings.Builder
		fmt.Fprintf(&out, "CLASS %s DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\n", name)
		for _, op := range []string{"get", "own", "has"} {
			result := "abap_bool"
			if op == "get" {
				result = "REF TO " + e.name("runtime.dynamic")
			}
			fmt.Fprintf(&out, "CLASS-METHODS %s IMPORTING p0 TYPE REF TO object key TYPE string location TYPE string RETURNING VALUE(result) TYPE %s.\n", op, result)
		}
		fmt.Fprintf(&out, "PROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS %s IMPLEMENTATION.\n", name)
		for _, op := range []string{"get", "own", "has"} {
			fmt.Fprintf(&out, "METHOD %s.\nIF p0 IS NOT BOUND.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\n", op)
			serial := 0
			for _, c := range e.classesByDepth() {
				if iface == nil || c.Abstract || c.Structural {
					continue
				}
				complete := true
				for _, m := range iface.Methods {
					impl, _ := e.method(c, m.Name)
					complete = complete && impl != nil && !impl.Static
				}
				if !complete {
					continue
				}
				fmt.Fprintf(&out, "IF p0 IS INSTANCE OF %s.\nDATA native%d TYPE REF TO %s.\nnative%d ?= p0.\nresult = %s=>%s( p0 = native%d key = key ).\nRETURN.\nENDIF.\n", e.name(c.Name), serial, e.name(c.Name), serial, e.propertyLookup(hir.Ref(c.Name)), op, serial)
				serial++
			}
			// Unknown native/structural carriers have no proven property layout. Keep
			// the source location at the call site instead of reporting false absence.
			fmt.Fprintf(&out, "RAISE EXCEPTION TYPE %s EXPORTING reason = location.\nENDMETHOD.\n", e.name("view.refusal"))
		}
		out.WriteString("ENDCLASS.\n")
		e.files[name+".clas.abap"] = out.String()
	}
	method := map[string]string{"object.get": "get", "object.hasOwn": "own", "object.has": "has"}[x.Op]
	original := b.interfaceOriginal(a, x.X.Type)
	location := strings.ReplaceAll(fmt.Sprintf("node %d (%s): interface property lookup has no supported class layout", x.ID, x.Source), "`", "``")
	b.line(n + " = " + name + "=>" + method + "( p0 = " + original + " key = " + key + " location = `" + location + "` ).")
}
