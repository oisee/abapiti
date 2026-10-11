package hir

// LiveClassViewLayouts identifies the layouts whose fields can proxy structural
// storage. Only explicit classView operations activate this ABI. Constructors
// are never executed for a view. Nominal assertions use their separate boundary.
func LiveClassViewLayouts(p *Program) map[string]bool {
	classes := map[string]*Class{}
	for _, c := range p.Classes {
		classes[c.Name] = c
	}
	out := map[string]bool{}
	for name := range classViewTargets(p) {
		if DataClassView(p, name) {
			for c := classes[name]; c != nil; c = classes[c.Super] {
				out[c.Name] = true
			}
		}
	}
	return out
}

// SealLiveClassViewBoundary preserves a refusal after optimizations remove a
// prototype call. Backend inlining must never invent methods on plain records.
func SealLiveClassViewBoundary(p *Program) {
	for name := range classViewTargets(p) {
		if !DataClassView(p, name) {
			for _, c := range p.Classes {
				if c.Name == name {
					c.ViewUnsafe = true
					break
				}
			}
		}
	}
}

func classViewTargets(p *Program) map[string]bool {
	out := map[string]bool{}
	var expr func(*Expr)
	var stmt func(*Stmt)
	expr = func(x *Expr) {
		if x == nil {
			return
		}
		if x.Kind == RuntimeOp && x.Op == "dynamic.classView" {
			out[x.Type.Name] = true
		}
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, a := range x.Args {
			expr(a)
		}
		stmt(x.Stmt)
	}
	stmt = func(s *Stmt) {
		if s == nil {
			return
		}
		expr(s.X)
		expr(s.Y)
		stmt(s.Body)
		stmt(s.Else)
		for _, v := range s.List {
			stmt(v)
		}
	}
	for _, c := range p.Classes {
		if c.Ctor != nil {
			stmt(c.Ctor.Body)
		}
		for _, m := range c.Methods {
			stmt(m.Body)
		}
	}
	return out
}

// DataClassView excludes prototype-dependent behavior, accessors, private
// fields and unknown ancestry. Unused field-reader declarations are safe;
// prototype dispatch is never fabricated.
func DataClassView(p *Program, name string) bool {
	classes := map[string]*Class{}
	for _, c := range p.Classes {
		classes[c.Name] = c
	}
	seen := map[string]bool{}
	for name != "" {
		c := classes[name]
		if c == nil || c.ViewUnsafe || seen[name] {
			return false
		}
		seen[name] = true
		for _, m := range c.Methods {
			if !m.Static && (m.Internal || m.Abstract || !fieldReader(m.Body) || prototypeDispatch(p, m.Name)) {
				return false
			}
		}
		for _, f := range c.Fields {
			if !f.Static && f.Private {
				return false
			}
		}
		name = c.Super
	}
	return len(seen) > 0
}

// A field reader adds no structural state. Its declaration may coexist with a
// view, but dispatch still requires the source object's real prototype method.
func fieldReader(s *Stmt) bool {
	if s == nil {
		return false
	}
	switch s.Kind {
	case Block:
		for _, part := range s.List {
			if !fieldReader(part) {
				return false
			}
		}
		return true
	case Return:
		return fieldReadExpr(s.X)
	default:
		return false
	}
}
func fieldReadExpr(x *Expr) bool {
	if x == nil {
		return true
	}
	switch x.Kind {
	case Lit, Local:
		return true
	case FieldGet:
		return x.X != nil && x.X.Kind == This
	case Binary:
		return fieldReadExpr(x.X) && fieldReadExpr(x.Y)
	case Unary, ToBoolean, IsUndefined:
		return fieldReadExpr(x.X)
	case Conditional:
		return fieldReadExpr(x.X) && fieldReadExpr(x.Y) && fieldReadExpr(x.Z)
	default:
		return false
	}
}

// Dispatch cannot be invented by an emitter guard that an HIR inliner could
// bypass. Refuse structural conversions if the closed program relies on a
// prototype callable of this name. Unknown interface ownership is conservative.
func prototypeDispatch(p *Program, name string) bool {
	found := false
	var expr func(*Expr)
	var stmt func(*Stmt)
	expr = func(x *Expr) {
		if x == nil {
			return
		}
		if (x.Kind == DirectCall || x.Kind == VirtualCall || x.Kind == SuperCall) && x.Name == name {
			found = true
		}
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, a := range x.Args {
			expr(a)
		}
		stmt(x.Stmt)
	}
	stmt = func(s *Stmt) {
		if s == nil {
			return
		}
		expr(s.X)
		expr(s.Y)
		stmt(s.Body)
		stmt(s.Else)
		for _, v := range s.List {
			stmt(v)
		}
	}
	for _, c := range p.Classes {
		if c.Ctor != nil {
			stmt(c.Ctor.Body)
		}
		for _, m := range c.Methods {
			stmt(m.Body)
		}
	}
	return found
}
