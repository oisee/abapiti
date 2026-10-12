package golang

import (
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Callable-storage classes model JS functions, not native class prototypes.
func (e *emitter) callableClass(c *hir.Class) bool {
	for base := c; base != nil; base = e.classBy(base.Super) {
		for _, name := range base.Implements {
			if name == "closure.thunk" || strings.HasPrefix(name, "closure.function.") {
				return true
			}
		}
	}
	return false
}

// The native catalogue is closed at emission. Each case calls the concrete
// virtual method and boxes its actual result, without a constituent downcast
// or a shared return-layout requirement. Dynamic property/function storage is
// deliberately not included in this capability.
func (b *body) nativeCall0(x *hir.Expr, receiver string) string {
	e := b.e
	if len(x.Args) != 1 || x.Args[0].Kind != hir.Lit {
		e.unsupported(x.Node, "native zero-argument call needs a literal method name")
		return "nil"
	}
	name, ok := x.Args[0].Value.(string)
	if !ok {
		e.unsupported(x.Node, "native call method name")
		return "nil"
	}
	var out strings.Builder
	cases := 0
	out.WriteString("func(d *dynamic) *dynamic {if d==nil || d.Tag!=2 {panic(newTrap(\"native method call requires a concrete class\"))};switch v:=d.Value.(type) {")
	for _, c := range e.p.Classes {
		if c.Structural || e.callableClass(c) || e.fieldOwner(c, name) != nil {
			continue
		}
		unsafe := false
		for base := c; base != nil; base = e.classBy(base.Super) {
			// Accessor slots use synthetic get.* HIR names. They may shadow
			// an inherited JS method without replacing its virtual HIR slot.
			// Refuse these prototype-dependent layouts, including setters.
			unsafe = unsafe || base.CoercionUnsafe || base.PrototypeAccessors
		}
		if unsafe {
			continue
		}
		m, _ := e.method(c, name)
		if m == nil || m.Static || len(m.Params) != 0 {
			continue
		}
		if m.Result.Kind == hir.Void {
			// An erased void slot does not certify the JS return value.
			// Use an existing value bridge, otherwise refuse that ABI.
			value, _ := e.method(c, name+"_value")
			if value == nil || value.Static || len(value.Params) != 0 || value.Result.Kind == hir.Void {
				continue
			}
			m = value
		}
		cases++
		out.WriteString("case *" + e.obj(c.Name) + ":if v==nil {panic(newRangeFault())};")
		call := "v." + e.member(m.Name) + "()"
		if m.Result.Kind == hir.Optional && !m.Result.Args[0].IsRef() {
			out.WriteString("result:=" + call + ";if !result.Has {return nil};return box(result.Value);")
		} else {
			out.WriteString("return box(" + call + ");")
		}
	}
	if cases == 0 {
		return "func(d *dynamic) *dynamic {panic(newTrap(" + strconv.Quote("native method "+name+" outside known concrete classes") + "))}(" + receiver + ")"
	}
	out.WriteString("};panic(newTrap(" + strconv.Quote("native method "+name+" outside known concrete classes") + "))}(" + receiver + ")")
	return out.String()
}
