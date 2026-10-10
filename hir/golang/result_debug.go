package golang

import "github.com/oisee/abapiti/hir"

// This finite fixed point is solely a debug traversal filter. Dynamic shapes
// stay open. Interface implementations are overapproximated by method names.
func (e *emitter) initResultDebugTypes() {
	e.resultDebugClassIndex = map[string]*hir.Class{}
	for _, c := range e.p.Classes {
		e.resultDebugClassIndex[c.Name] = c
	}
	e.resultDebugTypes = map[string]bool{"src/abap/2_statements/result.ts.Result": true}
	for changed := true; changed; {
		changed = false
		for _, c := range e.p.Classes {
			if e.resultDebugTypes[c.Name] {
				continue
			}
			has := e.resultDebugTypes[c.Super]
			for _, f := range c.Fields {
				if !f.Static && e.resultMayHold(f.Type) {
					has = true
				}
			}
			if has {
				e.resultDebugTypes[c.Name] = true
				changed = true
			}
		}
	}
	e.resultDebugTypeCache = map[string]bool{}
}
func (e *emitter) resultMayHold(t hir.Type) bool {
	key := t.String()
	if e.resultDebugTypeCache != nil {
		if v, ok := e.resultDebugTypeCache[key]; ok {
			return v
		}
	}
	value := e.resultMayHoldUncached(t)
	if e.resultDebugTypeCache != nil {
		e.resultDebugTypeCache[key] = value
	}
	return value
}
func (e *emitter) resultMayHoldUncached(t hir.Type) bool {
	switch t.Kind {
	case hir.Dynamic:
		return true
	case hir.ClassRef:
		if e.resultDebugTypes[t.Name] {
			return true
		}
		for _, c := range e.p.Classes {
			if !e.resultDebugTypes[c.Name] {
				continue
			}
			for base := c; base != nil; base = e.classBy(base.Super) {
				if base.Name == t.Name {
					return true
				}
			}
		}
	case hir.InterfaceRef:
		var iface *hir.Interface
		for _, in := range e.p.Interfaces {
			if in.Name == t.Name {
				iface = in
				break
			}
		}
		if iface == nil {
			return true
		}
		for _, c := range e.p.Classes {
			if !e.resultDebugTypes[c.Name] {
				continue
			}
			matches := true
			for _, m := range iface.Methods {
				method, _ := e.method(c, m.Name)
				if method == nil {
					matches = false
					break
				}
			}
			if matches {
				return true
			}
		}
	case hir.Array, hir.OrderedMap, hir.OrderedSet, hir.Optional:
		for _, arg := range t.Args {
			if e.resultMayHold(arg) {
				return true
			}
		}
	}
	return false
}
func (e *emitter) resultDebugShapes() map[string]map[string]bool {
	shapes := map[string]map[string]bool{"classDescriptor": {}}
	statics := map[string]bool{}
	shapes["__statics"] = statics
	for _, c := range e.p.Classes {
		fields := map[string]bool{}
		if c.Super != "" {
			fields[e.obj(c.Super)] = e.resultDebugTypes[c.Super]
		}
		for _, f := range c.Fields {
			if f.Static {
				statics[e.name("static."+c.Name+"."+f.Name)] = e.resultMayHold(f.Type)
			}
			if !f.Static {
				fields[e.member(f.Name)] = e.resultMayHold(f.Type)
			}
		}
		if c.Super == "" && len(c.Methods) == 0 && c.Ctor != nil && len(c.Ctor.Params) == len(c.Fields) {
			fields["source"] = true
		}
		shapes[e.obj(c.Name)] = fields
	}
	return shapes
}
