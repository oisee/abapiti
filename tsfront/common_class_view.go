package tsfront

import "github.com/oisee/abapiti/hir"

// A TS assertion erases at runtime. A nonescaping local used only through
// inherited members needs the original ancestor reference, not a nominal
// downcast. Keep escaping or subclass-specific views at the checked boundary.
func (l *lowerer) preserveCommonClassViews() {
	for _, c := range l.out.Classes {
		methods := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			methods = append(methods, c.Ctor)
		}
		for _, m := range methods {
			l.commonClassViews(m)
		}
	}
}

func (l *lowerer) commonClassViews(m *hir.Method) {
	decls := map[string][]*hir.Stmt{}
	viewWalk(m.Body, func(s *hir.Stmt) {
		if s.Kind == hir.VarDecl {
			decls[s.Name] = append(decls[s.Name], s)
		}
	}, nil)
	for name, declarations := range decls {
		if len(declarations) != 1 {
			continue
		}
		decl := declarations[0]
		cast := decl.X
		if cast == nil || cast.X == nil || !decl.Type.Equal(cast.Type) || (cast.Type.Kind != hir.ClassRef && !(cast.Type.Kind == hir.Optional && cast.Type.Args[0].Kind == hir.ClassRef)) {
			continue
		}
		if !(cast.Kind == hir.RuntimeOp && cast.Op == "object.checkedClassView" || cast.Kind == hir.Cast || cast.Kind == hir.Narrow) {
			continue
		}
		shadowed := false
		for _, p := range m.Params {
			shadowed = shadowed || p.Name == name
		}
		if shadowed {
			continue
		}
		operand := cast.X
		source, target := operand.Type, cast.Type
		optional := target.Kind == hir.Optional
		if optional {
			target = target.Args[0]
		}
		if source.Kind == hir.Optional && source.Args[0].IsRef() {
			source = source.Args[0]
			// Reference absence uses nil storage. Retain it until a method
			// actually dereferences the value or an optional chain skips it.
			operand = &hir.Expr{Kind: hir.Narrow, Node: operand.Node, Type: source, X: operand}
		}
		if source.Kind != hir.ClassRef && source.Kind != hir.InterfaceRef {
			continue
		}
		if source.Kind == hir.InterfaceRef {
			// The interface can carry sibling classes. Select a real ancestor
			// layout supplying every used inherited member, then check only
			// that boundary. Never fabricate the asserted subclass.
			ancestor := l.classByName(target.Name)
			if ancestor == nil || ancestor.Super == "" {
				continue
			}
			source = hir.Ref(ancestor.Super)
		}
		names := map[string]bool{name: true}
		aliasInitializers := map[*hir.Expr]bool{}
		for changed := true; changed; {
			changed = false
			for alias, ds := range decls {
				if names[alias] || len(ds) != 1 {
					continue
				}
				if x := ds[0].X; x != nil && x.Kind == hir.Local && names[x.Name] {
					names[alias] = true
					aliasInitializers[x] = true
					changed = true
				}
			}
		}
		parents := map[*hir.Expr]*hir.Expr{}
		viewWalk(m.Body, nil, func(x, parent *hir.Expr) { parents[x] = parent })
		safe := true
		viewWalk(m.Body, func(s *hir.Stmt) {
			if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local && names[s.X.Name] {
				safe = false
			}
		}, func(x, parent *hir.Expr) {
			if x.Kind != hir.Local || !names[x.Name] {
				return
			}
			if !(x.Type.Equal(target) || x.Type.Kind == hir.Optional && x.Type.Args[0].Equal(target)) {
				safe = false
				return
			}
			if parent == nil {
				safe = safe && aliasInitializers[x]
				return
			}
			for parent != nil && parent.Kind == hir.Narrow && parent.X != nil && parent.Type.Equal(target) {
				parent = parents[parent]
			}
			if parent == nil {
				safe = false
				return
			}
			switch parent.Kind {
			case hir.VirtualCall, hir.DirectCall:
				if parent.X == x || parent.X != nil && parent.X.Kind == hir.Narrow && parent.X.X == x {
					safe = safe && l.commonMethodOwner(source.Name, parent.Name) != "" && l.commonMethodOwner(source.Name, parent.Name) == l.commonMethodOwner(target.Name, parent.Name)
				} else {
					owner := parent.Owner
					if owner == "" && parent.X != nil {
						owner = parent.X.Type.Name
					}
					method := l.commonViewCallee(owner, parent.Name)
					found := false
					for i, arg := range parent.Args {
						if arg == x || arg.Kind == hir.Narrow && arg.X == x {
							found = true
							safe = safe && method != nil && i < len(method.Params) && l.acceptsType(method.Params[i].Type, source)
						}
					}
					safe = safe && found
				}
			case hir.Cast:
				// Passing the view through a genuine ancestor layout cannot
				// observe the asserted subclass. Preserve original dispatch/identity.
				safe = safe && parent.Type.Kind == hir.ClassRef && l.acceptsType(parent.Type, source)
			case hir.InstanceOf, hir.IsUndefined, hir.ToBoolean:
			case hir.Binary:
				safe = safe && (parent.Op == "==" || parent.Op == "!=") && parent.X != nil && parent.Y != nil && parent.X.Kind == hir.Local && parent.Y.Kind == hir.Local && parent.X.Name == name && parent.Y.Name == name
			default:
				safe = false
			}
		})
		if !safe {
			continue
		}
		storage := source
		if optional {
			storage = hir.T(hir.Optional, source)
		}
		decl.X, decl.Type = operand, storage
		if operand.Type.Kind == hir.InterfaceRef {
			decl.X = l.rtOp("dynamic.classView", l.coerce(operand, hir.T(hir.Dynamic)), source)
		}
		for alias := range names {
			if alias != name {
				decls[alias][0].Type = storage
			}
		}
		viewWalk(m.Body, nil, func(x, _ *hir.Expr) {
			if x.Kind == hir.Local && names[x.Name] {
				if x.Type.Kind == hir.Optional {
					x.Type = storage
				} else {
					x.Type = source
				}
			}
			if x.Kind == hir.Narrow && x.X != nil && x.X.Kind == hir.Local && names[x.X.Name] && x.Type.Equal(target) {
				x.Type = source
			}
		})
	}
}

func (l *lowerer) commonMethodOwner(class, method string) string {
	for c := l.classByName(class); c != nil; c = l.classByName(c.Super) {
		for _, m := range c.Methods {
			if m.Name == method && !m.Static {
				return c.Name
			}
		}
	}
	return ""
}

func viewWalk(s *hir.Stmt, statement func(*hir.Stmt), expression func(*hir.Expr, *hir.Expr)) {
	var stmt func(*hir.Stmt)
	var expr func(*hir.Expr, *hir.Expr)
	expr = func(x, parent *hir.Expr) {
		if x == nil {
			return
		}
		if expression != nil {
			expression(x, parent)
		}
		expr(x.X, x)
		expr(x.Y, x)
		expr(x.Z, x)
		for _, a := range x.Args {
			expr(a, x)
		}
		stmt(x.Stmt)
	}
	stmt = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		if statement != nil {
			statement(s)
		}
		expr(s.X, nil)
		expr(s.Y, nil)
		stmt(s.Body)
		stmt(s.Else)
		for _, part := range s.List {
			stmt(part)
		}
	}
	stmt(s)
}

func (l *lowerer) commonViewCallee(owner, name string) *hir.Method {
	for c := l.classByName(owner); c != nil; c = l.classByName(c.Super) {
		for _, m := range c.Methods {
			if m.Name == name {
				return m
			}
		}
	}
	return nil
}
