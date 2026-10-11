package golang

import "github.com/oisee/abapiti/hir"

// Boxed field storage needs a dynamic property adapter. Method-valued properties
// and creation of new properties remain unsupported, not fabricated.
func (e *emitter) collectBoxedShapes() {
	e.boxedShapes = map[string]bool{}
	e.boxedFields = map[string]bool{}
	dynamicRows := false
	var expr func(*hir.Expr)
	var stmt func(*hir.Stmt)
	expr = func(x *hir.Expr) {
		if x == nil {
			return
		}
		if x.Kind == hir.IndexGet && x.Type.Kind == hir.Dynamic {
			dynamicRows = true
		}
		if x.Kind == hir.RuntimeOp {
			if x.Op == "array.get" && x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Dynamic {
				dynamicRows = true
			}
			if x.Op == "dynamic.of" && x.X != nil && x.X.Type.Kind == hir.ClassRef {
				e.boxedShapes[x.X.Type.Name] = true
				e.boxedFields[x.X.Type.Name] = true
			}
			e.usesArrayPresence = e.usesArrayPresence || x.Op == "array.has"
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
	if dynamicRows {
		for _, c := range e.p.Classes {
			e.boxedShapes[c.Name] = true
		}
	}
}

func (e *emitter) shapeFields(c *hir.Class) {
	e.line("func (self *%s) dynamicGet(key jsString) *dynamic {", e.obj(c.Name))
	e.line("if self==nil {panic(newRangeFault())};switch key {")
	fields := map[string]hir.Field{}
	var order []string
	for base := c; base != nil; base = e.classBy(base.Super) {
		for _, f := range base.Fields {
			if f.Static {
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
		e.line("case str(%q):", f.Name)
		if f.Type.Kind == hir.Optional && !f.Type.Args[0].IsRef() {
			e.line("if !%s.Has {return nil};return box(%s.Value)", value, value)
		} else {
			e.line("return box(%s)", value)
		}
	}
	e.line("};return nil }")
	e.line("func (self *%s) dynamicPut(key jsString, value *dynamic) {", e.obj(c.Name))
	e.line("if self==nil {panic(newRangeFault())};switch key {")
	for _, name := range order {
		f := fields[name]
		typ := f.Type
		if typ.Kind == hir.Optional {
			typ = typ.Args[0]
		}
		if typ.Kind != hir.String && typ.Kind != hir.Bool && typ.Kind != hir.Number && typ.Kind != hir.Dynamic {
			continue
		}
		e.line("case str(%q): self.%s=%s(value);return", name, e.member(name), e.materializer(f.Type))
	}
	e.line("};panic(newTrap(\"dynamic field assignment outside certified storage\")) }")
}
