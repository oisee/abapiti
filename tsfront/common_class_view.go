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
		if cast == nil || cast.X == nil || !decl.Type.Equal(cast.Type) || cast.Type.Kind != hir.ClassRef {
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
		safe := true
		viewWalk(m.Body, func(s *hir.Stmt) {
			if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local && s.X.Name == name {
				safe = false
			}
		}, func(x, parent *hir.Expr) {
			if x.Kind != hir.Local || x.Name != name {
				return
			}
			if !x.Type.Equal(target) || parent == nil {
				safe = false
				return
			}
			switch parent.Kind {
			case hir.VirtualCall, hir.DirectCall:
				if parent.X != x || l.commonMethodOwner(source.Name, parent.Name) == "" || l.commonMethodOwner(source.Name, parent.Name) != l.commonMethodOwner(target.Name, parent.Name) {
					safe = false
				}
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
		decl.X, decl.Type = operand, source
		if operand.Type.Kind == hir.InterfaceRef {
			decl.X = l.rtOp("dynamic.classView", l.coerce(operand, hir.T(hir.Dynamic)), source)
		}
		viewWalk(m.Body, nil, func(x, _ *hir.Expr) {
			if x.Kind == hir.Local && x.Name == name {
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
