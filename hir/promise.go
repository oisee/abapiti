package hir

// UsesSettledPromises detects the explicit async boundary before ABAP exceptions
// are emitted, so source throws can share a base without changing other programs.
func UsesSettledPromises(p *Program) bool {
	var typ func(Type) bool
	typ = func(t Type) bool {
		if t.Kind == Promise {
			return true
		}
		for _, a := range t.Args {
			if typ(a) {
				return true
			}
		}
		return false
	}
	method := func(m *Method) bool {
		if typ(m.Result) {
			return true
		}
		for _, p := range m.Params {
			if typ(p.Type) {
				return true
			}
		}
		return false
	}
	var stmt func(*Stmt) bool
	var expr func(*Expr) bool
	expr = func(x *Expr) bool {
		if x == nil {
			return false
		}
		if typ(x.Type) {
			return true
		}
		if expr(x.X) || expr(x.Y) || expr(x.Z) || stmt(x.Stmt) {
			return true
		}
		for _, a := range x.Args {
			if expr(a) {
				return true
			}
		}
		return false
	}
	stmt = func(s *Stmt) bool {
		if s == nil {
			return false
		}
		if s.Kind == Async || typ(s.Type) || expr(s.X) || expr(s.Y) || stmt(s.Body) || stmt(s.Else) {
			return true
		}
		for _, c := range s.List {
			if stmt(c) {
				return true
			}
		}
		return false
	}
	for _, i := range p.Interfaces {
		for _, m := range i.Methods {
			if method(m) {
				return true
			}
		}
	}
	for _, c := range p.Classes {
		for _, f := range c.Fields {
			if typ(f.Type) {
				return true
			}
		}
		for _, m := range c.Methods {
			if method(m) || stmt(m.Body) {
				return true
			}
		}
		if c.Ctor != nil && stmt(c.Ctor.Body) {
			return true
		}
	}
	return false
}
