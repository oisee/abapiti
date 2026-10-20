package golang

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) liveStorage() bool { return len(e.liveClasses) > 0 || e.usesLiveArrays }

// A live structural class can expose a Map whose value type has a different
// nominal Go instantiation on the source and view sides. Enable the existing
// branded Map proxy in that case so reads and mutations remain attached to
// the same JavaScript Map while values pass through their structural views.
func (e *emitter) enableLiveMapValueViews() {
	roots := make(map[string]bool, len(e.liveClasses))
	for name := range e.liveClasses {
		roots[name] = true
	}
	markedValues := e.markedMapValueClasses()
	targets, sources := map[string]bool{}, map[string]bool{}
	bridge := false
	// Expanding one map value class can expose another map field. Iterate the
	// same HIR row/ancestor expansion to a fixed point before emitting layouts.
	for {
		changed := false
		for _, c := range e.p.Classes {
			if !e.liveClasses[c.Name] {
				continue
			}
			for _, f := range c.Fields {
				if f.Static {
					continue
				}
				value, ok := liveStringMapClassValue(f.Type)
				if !ok || !hir.DataClassView(e.p, value) {
					continue
				}
				bridge = true
				if !targets[value] {
					targets[value], roots[value], changed = true, true, true
				}
			}
		}
		// map.markJS installs the source-side decoder needed by writes through a
		// projected map. Only structurally type-compatible source classes receive
		// that decoder in this live-field-only path.
		for _, source := range markedValues {
			if !sources[source] && hir.DataClassView(e.p, source) && e.structuralMapValueMatch(source, targets) {
				sources[source], roots[source], changed = true, true, true
			}
		}
		expanded := hir.LiveClassViewLayoutsWith(e.p, roots)
		if len(expanded) != len(e.liveClasses) {
			changed = true
		} else {
			for name := range expanded {
				if !e.liveClasses[name] {
					changed = true
					break
				}
			}
		}
		e.liveClasses = expanded
		if !changed {
			break
		}
	}
	if !bridge {
		return
	}
	e.liveMapValueTypes = sources
	e.liveMapValueViews = true
	_, e.dynamicMaps = hir.UsedRuntimeOps(e.p)["dynamic.mapView"]
	e.mapViews, e.mapViewsKnown = true, true
	e.collectionProbe = true
}

func liveStringMapClassValue(t hir.Type) (string, bool) {
	for t.Kind == hir.Optional {
		t = t.Args[0]
	}
	if t.Kind != hir.OrderedMap || len(t.Args) != 2 || t.Args[0].Kind != hir.String {
		return "", false
	}
	return mapViewClassName(t.Args[1])
}

func (e *emitter) markedMapValueClasses() []string {
	seen := map[string]bool{}
	var values []string
	for _, c := range e.p.Classes {
		visit := func(s *hir.Stmt) {
			walkStmt(s, func(*hir.Stmt) {}, func(x, _ *hir.Expr) {
				if x.Kind == hir.RuntimeOp && x.Op == "map.markJS" && len(x.Type.Args) == 2 {
					if name, ok := mapViewClassName(x.Type.Args[1]); ok && !seen[name] {
						seen[name] = true
						values = append(values, name)
					}
				}
			})
		}
		if c.Ctor != nil {
			visit(c.Ctor.Body)
		}
		for _, m := range c.Methods {
			visit(m.Body)
		}
	}
	return values
}

func mapViewClassName(t hir.Type) (string, bool) {
	for t.Kind == hir.Optional {
		t = t.Args[0]
	}
	return t.Name, t.Kind == hir.ClassRef
}

func (e *emitter) structuralMapValueMatch(source string, targets map[string]bool) bool {
	for target := range targets {
		if e.structuralMapClassMatch(source, target, map[[2]string]bool{}) {
			return true
		}
	}
	return false
}

func (e *emitter) classFieldTypes(name string) map[string]hir.Type {
	fields := map[string]hir.Type{}
	for c := e.classBy(name); c != nil; c = e.classBy(c.Super) {
		for _, f := range c.Fields {
			if !f.Static {
				if _, overridden := fields[f.Name]; overridden {
					continue
				}
				fields[f.Name] = f.Type
			}
		}
	}
	return fields
}

func (e *emitter) structuralMapClassMatch(source, target string, seen map[[2]string]bool) bool {
	pair := [2]string{source, target}
	if seen[pair] {
		return true
	}
	seen[pair] = true
	sourceFields, targetFields := e.classFieldTypes(source), e.classFieldTypes(target)
	if len(sourceFields) == 0 || len(sourceFields) != len(targetFields) {
		return false
	}
	for name, targetType := range targetFields {
		sourceType, ok := sourceFields[name]
		if !ok || !e.structuralMapTypeMatch(sourceType, targetType, seen) {
			return false
		}
	}
	return true
}

func (e *emitter) structuralMapTypeMatch(source, target hir.Type, seen map[[2]string]bool) bool {
	if source.Equal(target) {
		return true
	}
	if source.Kind != target.Kind || len(source.Args) != len(target.Args) {
		return false
	}
	switch source.Kind {
	case hir.Optional, hir.Array, hir.OrderedSet:
		return e.structuralMapTypeMatch(source.Args[0], target.Args[0], seen)
	case hir.OrderedMap:
		return e.structuralMapTypeMatch(source.Args[0], target.Args[0], seen) && e.structuralMapTypeMatch(source.Args[1], target.Args[1], seen)
	case hir.ClassRef:
		if !hir.DataClassView(e.p, source.Name) || !hir.DataClassView(e.p, target.Name) {
			return false
		}
		return e.structuralMapClassMatch(source.Name, target.Name, seen)
	default:
		return false
	}
}

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
	if base.Kind == hir.UnionRef {
		// An unchecked array annotation does not prove presence. Validate
		// present members, but let consumers observe an absent slot as nil.
		return "func() " + e.typ(t) + " {v:=" + value + ";if nilRef(v) {return nil};return " + e.unionValidator(base.Name, "") + "(box(v))}()"
	}
	if base.Kind == hir.InterfaceRef {
		if view := e.structuralUnion(base); view != "" {
			return "func() " + e.typ(t) + " {v:=" + value + ";if nilRef(v) {return nil};return " + view + "(box(v))}()"
		}
	}
	if base.Kind == hir.ClassRef && e.liveClasses[base.Name] {
		return "func() " + e.typ(t) + " {v:=" + value + ";if row,ok:=any(v).(" + e.typ(t) + ");ok {return row};return " + e.liveView(base) + "(box(v))}()"
	}
	return "castRef[" + e.typ(t) + "](" + value + ")"
}

// liveView retains native references unchanged and attaches a data-only layout
// to structural storage. All embedded ancestor layouts share that same source.
func (e *emitter) liveView(t hir.Type) string {
	return e.liveViewMode(t, false)
}

// A member projection needs its own provenance guard. A fresh wrapper leaves
// the original native object untouched and keeps identity through viewSource.
func (e *emitter) liveViewMode(t hir.Type, fresh bool) string {
	key := "view." + t.Name
	if fresh {
		key += ".fresh"
	}
	name := e.name(key)
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
	line("if d==nil || d.Tag==tagNull {return nil}")
	if !fresh {
		line("if d.Tag==2 {if v,ok:=d.Value.(%s);ok {return v}}", e.typ(t))
	}
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
	return e.viewValueWithLocal(t, d, false)
}

// Getter locals need an explicit use when conversion is refused. Other callers
// pass parameters or expressions, and retain their original conversion bodies.
func (e *emitter) viewValueWithLocal(t hir.Type, d string, local bool) string {
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
	case hir.ClassValue:
		return "dynClass(" + d + ")"
	case hir.UnionRef:
		return e.unionValidator(t.Name, "") + "(" + d + ")"
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
		helper := "liveArrayView"
		if t.Readonly {
			helper = "liveReadonlyArrayView"
		}
		return helper + "[" + e.typ(arrayStorage(t).Args[0]) + "](" + d + ",func(d *dynamic) " + e.typ(arrayStorage(t).Args[0]) + " {return " + decode + "})"
	case hir.InterfaceRef:
		if view := e.structuralUnion(t); view != "" {
			return view + "(" + d + ")"
		}
		return "castRef[" + e.typ(t) + "](dynRef(" + d + "))"
	case hir.OrderedMap, hir.OrderedSet:
		if t.Readonly && e.readonlyViews {
			return e.readonlyContainerView(t, d)
		}
		if t.Kind == hir.OrderedMap && e.liveMapValueViews && len(t.Args) == 2 && t.Args[0].Kind == hir.String {
			if name, ok := mapViewClassName(t.Args[1]); ok && e.liveClasses[name] {
				return e.liveMapValueView(t, d)
			}
		}
		return "castRef[" + e.typ(t) + "](dynRef(" + d + "))"
	default:
		use := ""
		if local {
			use = "_ = " + d + ";"
		}
		return "func() " + e.typ(t) + " {" + use + "panic(newTrap(\"unsupported live class field conversion\"))}()"
	}
}

func (e *emitter) liveMapValueView(t hir.Type, d string) string {
	// Keep the branded proxy contract in sync with body.dynamicMapView in
	// map_view.go; this path additionally preserves an already exact map type.
	mapType, valueType := e.typ(t), e.typ(t.Args[1])
	decode := e.viewValue(t.Args[1], "value")
	return fmt.Sprintf(`func(d *dynamic) %s {
 if d==nil || d.Tag!=2 {panic(newRangeFault())}
 if m,ok:=d.Value.(%s);ok {if storage,branded:=any(m).(stringMapStorage);branded && storage.stringMapBrand() {return m};panic(newRangeFault())}
 src,ok:=d.Value.(stringMapStorage);if !ok || !src.stringMapBrand() {panic(newRangeFault())}
 return &%s{jsMap:true,proxy:&mapProxy[jsString,%s]{source:d,
 get:func(k jsString) optional[%s] {value,has:=src.stringMapGet(k);if !has{return optional[%s]{}};return present(%s)},
 set:func(k jsString,v %s){src.stringMapSetView(k,mapValueBox(v))},has:src.stringMapHas,remove:src.stringMapDelete,keys:src.stringMapKeys,size:src.mapSize,insertions:src.mapInsertions}}
 }(%s)`, mapType, mapType, strings.TrimPrefix(mapType, "*"), valueType, valueType, valueType, decode, valueType, d)
}
func (e *emitter) liveFields(c *hir.Class) {
	for _, f := range c.Fields {
		if f.Static {
			continue
		}
		read := e.name("view.read." + c.Name + "." + f.Name)
		write := e.name("view.write." + c.Name + "." + f.Name)
		e.line("func (self *%s) %s() %s {if self==nil {panic(newRangeFault())};%sif self.viewSource!=nil {d:=self.viewSource.get(str(%q));return %s};return self.%s}", e.obj(c.Name), read, e.typ(f.Type), "if self.viewCheck!=nil {self.viewCheck(self.viewSource)};"+e.unionPayloadGuard(c.Name), f.Name, e.viewValueWithLocal(f.Type, "d", true), e.member(f.Name))
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
