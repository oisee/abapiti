package golang

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) liveStorage() bool { return len(e.liveClasses) > 0 || e.usesLiveArrays }

// A native reference array can receive a parsed row through a live alias.
// Keep already compatible rows, and view tagged structural rows on the caller's
// typed reads as well. Nominal assertions retain their separate cast boundary.
func (e *emitter) arrayReference(t hir.Type, value string) string {
	base := t
	if base.Kind == hir.Optional {
		base = base.Args[0]
	}
	if base.Kind == hir.Dynamic {
		// Erased native reference slots contain the original objects, not
		// necessarily pre-boxed tags. Boxing reads preserves that identity.
		return "box(" + value + ")"
	}
	if base.Kind == hir.ClassRef && e.liveClasses[base.Name] {
		return "func() " + e.typ(t) + " {v:=" + value + ";if row,ok:=any(v).(" + e.typ(t) + ");ok {return row};return " + e.liveView(base) + "(box(v))}()"
	}
	return "castRef[" + e.typ(t) + "](" + value + ")"
}

// liveView retains native references unchanged and attaches a data-only layout
// to structural storage. All embedded ancestor layouts share that same source.
func (e *emitter) liveView(t hir.Type) string {
	name := e.name("view." + t.Name)
	if e.liveViews == nil {
		e.liveViews = map[string]bool{}
	}
	if e.liveViews[name] {
		return name
	}
	e.liveViews[name] = true
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	line("func %s(d *dynamic) %s {", name, e.typ(t))
	line("if d==nil {return nil};if d.Tag==2 {if v,ok:=d.Value.(%s);ok {return v}}", e.typ(t))
	if !hir.DataClassView(e.p, t.Name) {
		line("panic(newTrap(%q))", "structural class view requires data-only public ancestry: "+t.Name)
	} else {
		primitiveReads := false
		for c := e.classBy(t.Name); c != nil; c = e.classBy(c.Super) {
			primitiveReads = primitiveReads || c.PrimitiveReads
		}
		// JS permits reading ordinary missing properties on primitive rule
		// configs before a setter installs defaults. Keep the primitive as the
		// backing value; writes still use the checked adapter and refuse.
		allowed := "d.Tag!=tagObject"
		if primitiveReads {
			allowed += " && d.Tag!=1 && d.Tag!=3 && d.Tag!=4 && d.Tag!=5 && d.Tag!=6"
		}
		line("if %s {if d.Tag!=2 {panic(newRangeFault())};if _,ok:=d.Value.(interface{structuralRecord()});!ok {panic(newTrap(\"structural class view refuses prototype-bearing source\"))}}", allowed)
		c := e.classBy(t.Name)
		line("out:=&%s{}", e.obj(c.Name))
		for base := c; base != nil; base = e.classBy(base.Super) {
			line("out.%s().viewSource=d", e.getter(base.Name))
		}
		line("return out")
	}
	line("}")
	e.extra.WriteString(b.String())
	return name
}

func (e *emitter) viewValue(t hir.Type, d string) string {
	if t.Kind == hir.Optional {
		v := e.viewValue(t.Args[0], d)
		if !t.Args[0].IsRef() {
			v = "present(" + v + ")"
		}
		return "func() " + e.typ(t) + " {if " + d + "==nil {var z " + e.typ(t) + ";return z};return " + v + "}()"
	}
	switch t.Kind {
	case hir.Dynamic:
		return d
	case hir.String:
		return "dynString(" + d + ")"
	case hir.Bool:
		return "dynBoolean(" + d + ")"
	case hir.Number:
		return "dynNumber(" + d + ")"
	case hir.I32:
		e.hirMath = true
		return "func() int32 {v:=dynNumber(" + d + ");if v!=math.Trunc(v) || v < -2147483648 || v > 2147483647 {panic(newRangeFault())};return int32(v)}()"
	case hir.I64:
		e.hirMath = true
		return "func() int64 {v:=dynNumber(" + d + ");if v!=math.Trunc(v) || v < -9007199254740991 || v > 9007199254740991 {panic(newRangeFault())};return int64(v)}()"
	case hir.ClassRef:
		if t.Name != hir.RootObject && e.liveClasses[t.Name] {
			return e.liveView(t) + "(" + d + ")"
		}
		return "castRef[" + e.typ(t) + "](dynRef(" + d + "))"
	case hir.Array:
		decode := e.viewValue(t.Args[0], "d")
		if t.Args[0].Kind == hir.String {
			decode = "liveStringElement(d)"
		}
		return "liveArrayView[" + e.typ(arrayStorage(t).Args[0]) + "](" + d + ",func(d *dynamic) " + e.typ(arrayStorage(t).Args[0]) + " {return " + decode + "})"
	case hir.InterfaceRef, hir.OrderedMap, hir.OrderedSet:
		return "castRef[" + e.typ(t) + "](dynRef(" + d + "))"
	default:
		return "func() " + e.typ(t) + " {panic(newTrap(\"unsupported live class field conversion\"))}()"
	}
}
func (e *emitter) liveFields(c *hir.Class) {
	for _, f := range c.Fields {
		if f.Static {
			continue
		}
		read := e.name("view.read." + c.Name + "." + f.Name)
		write := e.name("view.write." + c.Name + "." + f.Name)
		e.line("func (self *%s) %s() %s {if self==nil {panic(newRangeFault())};if self.viewSource!=nil {d:=self.viewSource.get(str(%q));return %s};return self.%s}", e.obj(c.Name), read, e.typ(f.Type), f.Name, e.viewValue(f.Type, "d"), e.member(f.Name))
		value := "box(value)"
		if f.Type.Kind == hir.Optional && !f.Type.Args[0].IsRef() {
			value = "func() *dynamic {if !value.Has {return nil};return box(value.Value)}()"
		}
		metadata := ""
		if e.tracksOwnKeys(c) {
			metadata = ";if self.ownKeys!=nil {self.ownKeys.set(str(" + fmt.Sprintf("%q", f.Name) + "),nil)}"
		}
		if e.recordDeletion {
			metadata += ";self.propertyWritten(str(" + fmt.Sprintf("%q", f.Name) + "))"
		}
		e.line("func (self *%s) %s(value %s) {if self==nil {panic(newRangeFault())};if self.viewSource!=nil {self.viewSource.put(str(%q),%s);return};self.%s=value%s}", e.obj(c.Name), write, e.typ(f.Type), f.Name, value, e.member(f.Name), metadata)
	}
}
