package golang

import (
	"slices"

	"github.com/oisee/abapiti/hir"
)

// Boxed field storage needs a dynamic property adapter. Method-valued properties
// and creation of new properties remain unsupported, not fabricated.
func (e *emitter) collectBoxedShapes() {
	e.boxedShapes = map[string]bool{}
	e.boxedFields = map[string]bool{}
	for _, u := range e.p.Unions {
		for _, a := range u.Alternatives {
			e.boxedShapes[a.Payload.Name] = true
			e.boxedFields[a.Payload.Name] = true
		}
	}
	dynamicRows := false
	guardRows := false
	classViews := false
	var boxed []hir.Type
	var expr func(*hir.Expr)
	var stmt func(*hir.Stmt)
	expr = func(x *hir.Expr) {
		if x == nil {
			return
		}
		if x.Kind == hir.New && x.Type.Kind == hir.Array && len(x.Args) == 1 {
			e.usesArrayPresence = true
		}
		if x.Kind == hir.IndexGet && x.Type.Kind == hir.Dynamic {
			dynamicRows = true
		}
		if x.Kind == hir.RuntimeOp {
			guardRows = guardRows || x.Op == "dynamic.get" && x.X != nil && x.X.Kind == hir.RuntimeOp && x.X.Op == "dynamic.of" && x.X.X != nil && x.X.X.Type.Name == hir.RootObject || x.Op == "lsp.isTextDocumentEdit" || x.Op == "lsp.isRenameFile" || x.Op == "lsp.isCreateFile" || x.Op == "lsp.isDeleteFile"
			if x.Op == "chai.throwMatch" {
				e.chaiThrow = true
				e.chaiDeep = true
				e.usesArrayPresence = true
				e.usesTypedCatch = true
			}
			if x.Op == "chai.deepEqual" || x.Op == "chai.deepInclude" || x.Op == "chai.box" || x.Op == "chai.arrayInclude" || x.Op == "chai.members" {
				e.chaiDeep = true
				e.usesArrayPresence = true
			}
			if x.Op == "array.get" && x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Dynamic {
				dynamicRows = true
			}
			if x.Op == "dynamic.of" && x.X != nil && x.X.Type.Kind == hir.ClassRef {
				e.boxedShapes[x.X.Type.Name] = true
				e.boxedFields[x.X.Type.Name] = true
			}
			if x.Op == "dynamic.of" && x.X != nil {
				boxed = append(boxed, x.X.Type)
			}
			classViews = classViews || x.Op == "dynamic.classView" || x.Op == "container.readonlyClassView"
			e.usesArrayPresence = e.usesArrayPresence || (x.Op == "array.has" || x.Op == "array.spread0")
			e.usesArrayDeletion = e.usesArrayDeletion || x.Op == "array.delete"
		}
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, a := range x.Args {
			expr(a)
		}
		stmt(x.Stmt)
	}
	stmt = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		expr(s.X)
		expr(s.Y)
		stmt(s.Body)
		stmt(s.Else)
		for _, t := range s.List {
			stmt(t)
		}
	}
	for _, c := range e.p.Classes {
		if c.Ctor != nil {
			stmt(c.Ctor.Body)
		}
		for _, m := range c.Methods {
			stmt(m.Body)
		}
	}
	if guardRows {
		for _, c := range e.p.Classes {
			if c.Structural {
				e.boxedFields[c.Name] = true
			}
		}
	}
	if dynamicRows {
		for _, c := range e.p.Classes {
			e.boxedShapes[c.Name] = true
		}
	}
	if classViews {
		e.collectPrototypeViewSources(boxed)
	}
}

// A structural view can be attached to any boxed class instance: an erased
// assertion (`obj as unknown as {def?: unknown}`) keeps the prototype-bearing
// object. Its reads need the class's property adapter, also on subclasses,
// whose own fields the inherited adapter would miss. The static type selects
// the candidates: a class and its descendants, or every class with all methods
// of an interface. Other run-time classes keep refusing at view construction.
func (e *emitter) collectPrototypeViewSources(boxed []hir.Type) {
	seen := map[string]bool{}
	for _, t := range boxed {
		for t.Kind == hir.Optional {
			t = t.Args[0]
		}
		if (t.Kind != hir.ClassRef || t.Name == hir.RootObject) && t.Kind != hir.InterfaceRef || seen[string(t.Kind)+":"+t.Name] {
			continue
		}
		seen[string(t.Kind)+":"+t.Name] = true
		var iface *hir.Interface
		if t.Kind == hir.InterfaceRef {
			for _, i := range e.p.Interfaces {
				if i.Name == t.Name && len(i.StructuralMembers) == 0 && len(i.Methods) != 0 {
					iface = i
				}
			}
			if iface == nil {
				continue
			}
		}
		for _, c := range e.p.Classes {
			if c.Structural {
				continue
			}
			for base := c; base != nil; base = e.classBy(base.Super) {
				if t.Kind == hir.ClassRef && base.Name == t.Name || iface != nil && e.implementsMethods(base, iface) {
					e.boxedFields[c.Name] = true
					break
				}
			}
		}
	}
}

func (e *emitter) implementsMethods(c *hir.Class, iface *hir.Interface) bool {
	for _, m := range iface.Methods {
		found := false
		for base := c; base != nil && !found; base = e.classBy(base.Super) {
			for _, own := range base.Methods {
				found = found || !own.Static && own.Name == m.Name
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (e *emitter) shapeFields(c *hir.Class) {
	e.line("func (self *%s) dynamicGet(key jsString) *dynamic {", e.obj(c.Name))
	e.line("if self==nil {panic(newRangeFault())}")
	if e.liveClasses[c.Name] {
		e.line("if self.viewSource!=nil {return self.viewSource.get(key)}")
	}
	if e.recordDeletion {
		e.line("if self.deletedKeys[key] {return nil}")
	}
	e.line("switch key {")
	fields := map[string]hir.Field{}
	var order []string
	for base := c; base != nil; base = e.classBy(base.Super) {
		for _, f := range base.Fields {
			if f.Static || f.PrivateSlot {
				continue
			}
			if _, ok := fields[f.Name]; !ok {
				fields[f.Name] = f
				order = append(order, f.Name)
			}
		}
	}
	for _, name := range order {
		f := fields[name]
		value := "self." + e.member(f.Name)
		owner := e.fieldOwner(c, f.Name)
		if e.liveClasses[owner.Name] {
			value = "self." + e.getter(owner.Name) + "()." + e.name("view.read."+owner.Name+"."+f.Name) + "()"
		}
		e.line("case str(%q):", f.Name)
		if f.Type.Kind == hir.Optional && !f.Type.Args[0].IsRef() {
			e.line("if !%s.Has {return nil};return box(%s.Value)", value, value)
		} else {
			e.line("return box(%s)", value)
		}
	}
	if _, shadowed := fields["constructor"]; !c.Structural && !shadowed {
		e.line("case str(\"constructor\"): return box(self.constructorDescriptor())")
	}
	if !c.Structural {
		// A prototype function is not an absent property, and a layout that
		// lost fields or keeps accessors cannot prove one absent either.
		var methods []string
		opaque := false
		for base := c; base != nil; base = e.classBy(base.Super) {
			opaque = opaque || base.FieldsDropped || base.PrototypeAccessors
			for _, m := range base.Methods {
				if _, shadowed := fields[m.Name]; !m.Static && !shadowed && !slices.Contains(methods, m.Name) && m.Name != "constructor" {
					methods = append(methods, m.Name)
				}
			}
		}
		for _, name := range methods {
			e.line("case str(%q): panic(newTrap(\"erased prototype function reads are unsupported\"))", name)
		}
		if opaque {
			e.line("default: panic(newTrap(\"property read on a class layout that cannot prove absence\"))")
		}
	}
	if len(e.liveClasses) > 0 && c.Structural {
		e.line("};if self.viewExtras!=nil {return self.viewExtras.get(key).Value};return nil }")
	} else {
		e.line("};return nil }")
	}
	e.line("func (self *%s) dynamicPut(key jsString, value *dynamic) {", e.obj(c.Name))
	e.line("if self==nil {panic(newRangeFault())}")
	if e.liveClasses[c.Name] {
		e.line("if self.viewSource!=nil {self.viewSource.put(key,value);return}")
	}
	if e.tracksOwnKeys(c) {
		e.line("if self.ownKeys!=nil {self.ownKeys.set(key,nil)}")
	}
	if e.recordDeletion {
		e.line("delete(self.deletedKeys,key);if self.deletionOrder!=nil {self.deletionOrder.set(key,nil)}")
	}
	e.line("switch key {")
	for _, name := range order {
		f := fields[name]
		typ := f.Type
		if typ.Kind == hir.Optional {
			typ = typ.Args[0]
		}
		if !(len(e.liveClasses) > 0 && c.Structural) && typ.Kind != hir.String && typ.Kind != hir.Bool && typ.Kind != hir.Number && typ.Kind != hir.Dynamic {
			continue
		}
		owner := e.fieldOwner(c, f.Name)
		if len(e.liveClasses) > 0 && c.Structural {
			e.line("case str(%q): self.%s=%s;return", name, e.member(name), e.viewValue(f.Type, "value"))
		} else if e.liveClasses[owner.Name] {
			e.line("case str(%q): self.%s().%s(%s(value));return", name, e.getter(owner.Name), e.name("view.write."+owner.Name+"."+f.Name), e.materializer(f.Type))
		} else {
			e.line("case str(%q): self.%s=%s(value);return", name, e.member(name), e.materializer(f.Type))
		}
	}
	if len(e.liveClasses) > 0 && c.Structural {
		e.line("};if key==str(\"__proto__\") {panic(newTrap(\"prototype mutation through class view is unsupported\"))};if self.viewExtras==nil {self.viewExtras=&fastOrderedMap[jsString,*dynamic]{}};self.viewExtras.set(key,value) }")
	} else {
		e.line("};panic(newTrap(\"dynamic field assignment outside certified storage\")) }")
	}

	if e.recordDeletion {
		e.deleteFields(c, fields, order)
	}
}
