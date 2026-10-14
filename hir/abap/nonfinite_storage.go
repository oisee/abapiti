package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Native f remains the finite fast path. Values reaching a nonfinite-producing
// boundary use the existing tagged numeric box, including across scalar fields
// and call signatures. This is a storage lowering, not ToNumber(undefined)=0.
// Work on a copy: Go emission and the input HIR retain their Number semantics.
func retainNonfiniteStorage(p *hir.Program) (*hir.Program, map[*hir.Expr]bool) {
	if !hasNaNBoundary(p) {
		return p, nil
	}
	q := copyNumberProgram(p)
	a := &numberStorage{
		classes:    map[string]*hir.Class{},
		interfaces: map[string]*hir.Interface{},
		methods:    map[string]*hir.Method{},
		owners:     map[*hir.Method]string{},
		locals:     map[*hir.Method]map[string]*hir.Type{},
		tagged:     map[*hir.Type]bool{},
		exprs:      map[*hir.Expr]bool{},
	}
	for _, c := range q.Classes {
		a.classes[c.Name] = c
		if c.Ctor != nil {
			a.addMethod(c.Name, c.Ctor)
		}
		for _, m := range c.Methods {
			a.addMethod(c.Name, m)
		}
	}
	for _, i := range q.Interfaces {
		a.interfaces[i.Name] = i
		for _, m := range i.Methods {
			a.addMethod(i.Name, m)
		}
	}
	for {
		a.changed = false
		for _, c := range q.Classes {
			if c.Ctor != nil {
				a.stmt(c.Ctor, c.Ctor.Body)
			}
			for _, m := range c.Methods {
				a.stmt(m, m.Body)
			}
			for _, m := range c.Methods {
				a.linkMethods(m, a.lookupMethod(c.Super, m.Name))
			}
			// Interface forwarding methods can use an inherited implementation.
			// Link those signatures too, even if the subclass declares no body.
			for _, owner := range c.Implements {
				if i := a.interfaces[owner]; i != nil {
					for _, n := range i.Methods {
						if m := a.lookupMethod(c.Name, n.Name); m != nil {
							a.linkMethods(m, n)
						}
					}
				}
			}
		}
		if !a.changed {
			break
		}
	}
	for t := range a.tagged {
		*t = hir.T(hir.Optional, hir.T(hir.Number))
	}
	for x := range a.exprs {
		x.Type = hir.T(hir.Optional, hir.T(hir.Number))
	}
	return q, a.exprs
}

// The frontend's absent numeric operand uses this checked NaN producer.
// Dynamic parseFloat and other nonfinite boundaries keep their existing
// explicit domain checks until their complete storage paths are certified.
func nanStorageBoundary(x *hir.Expr) bool {
	return x.Kind == hir.RuntimeOp && x.Op == "string.parseFloatNumber" && x.X != nil && x.X.Kind == hir.Lit && x.X.Value == "NaN"
}

func hasNaNBoundary(p *hir.Program) bool {
	found := false
	var expr func(*hir.Expr)
	expr = func(x *hir.Expr) {
		if x == nil || found {
			return
		}
		found = nanStorageBoundary(x)
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, v := range x.Args {
			expr(v)
		}
		(&emitter{}).walkStmt(x.Stmt, expr)
	}
	for _, c := range p.Classes {
		if c.Ctor != nil {
			(&emitter{}).walkStmt(c.Ctor.Body, expr)
		}
		for _, m := range c.Methods {
			(&emitter{}).walkStmt(m.Body, expr)
		}
	}
	return found
}

type numberStorage struct {
	classes    map[string]*hir.Class
	interfaces map[string]*hir.Interface
	methods    map[string]*hir.Method
	owners     map[*hir.Method]string
	locals     map[*hir.Method]map[string]*hir.Type
	tagged     map[*hir.Type]bool
	exprs      map[*hir.Expr]bool
	changed    bool
}

func (a *numberStorage) addMethod(owner string, m *hir.Method) {
	a.methods[owner+"\x00"+m.Name] = m
	a.owners[m] = owner
	slots := map[string]*hir.Type{}
	for i := range m.Params {
		slots[m.Params[i].Name] = &m.Params[i].Type
	}
	var walk func(*hir.Stmt)
	walk = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		if s.Kind == hir.VarDecl || s.Kind == hir.ForEach || s.Kind == hir.Try {
			slots[s.Name] = &s.Type
		}
		walk(s.Body)
		walk(s.Else)
		for _, child := range s.List {
			walk(child)
		}
		var expr func(*hir.Expr)
		expr = func(x *hir.Expr) {
			if x == nil {
				return
			}
			walk(x.Stmt)
			expr(x.X)
			expr(x.Y)
			expr(x.Z)
			for _, v := range x.Args {
				expr(v)
			}
		}
		expr(s.X)
		expr(s.Y)
	}
	walk(m.Body)
	a.locals[m] = slots
}
func (a *numberStorage) lookupMethod(owner, name string) *hir.Method {
	for owner != "" {
		if m := a.methods[owner+"\x00"+name]; m != nil {
			return m
		}
		c := a.classes[owner]
		if c == nil {
			break
		}
		owner = c.Super
	}
	return nil
}
func (a *numberStorage) slot(m *hir.Method, x *hir.Expr) *hir.Type {
	if x == nil {
		return nil
	}
	if x.Kind == hir.Local {
		return a.locals[m][x.Name]
	}
	if x.Kind != hir.FieldGet && x.Kind != hir.StaticGet {
		return nil
	}
	owner := x.Owner
	if x.X != nil {
		owner = x.X.Type.Name
	}
	for c := a.classes[owner]; c != nil; c = a.classes[c.Super] {
		for i := range c.Fields {
			if c.Fields[i].Name == x.Name {
				return &c.Fields[i].Type
			}
		}
	}
	return nil
}
func (a *numberStorage) mark(t *hir.Type) {
	if t != nil && t.Kind == hir.Number && !a.tagged[t] {
		a.tagged[t] = true
		a.changed = true
	}
}
func (a *numberStorage) linkMethods(m, n *hir.Method) {
	if n == nil {
		return
	}
	if a.tagged[&m.Result] || a.tagged[&n.Result] {
		a.mark(&m.Result)
		a.mark(&n.Result)
	}
	for i := range m.Params {
		if i >= len(n.Params) {
			break
		}
		if a.tagged[&m.Params[i].Type] || a.tagged[&n.Params[i].Type] {
			a.mark(&m.Params[i].Type)
			a.mark(&n.Params[i].Type)
		}
	}
}
func (a *numberStorage) stmt(m *hir.Method, s *hir.Stmt) {
	if s == nil {
		return
	}
	x, y := a.expr(m, s.X), a.expr(m, s.Y)
	switch s.Kind {
	case hir.VarDecl:
		if x {
			a.mark(a.locals[m][s.Name])
		}
	case hir.Assign:
		if y {
			a.mark(a.slot(m, s.X))
		}
	case hir.Return:
		if x {
			a.mark(&m.Result)
		}
	}
	a.stmt(m, s.Body)
	a.stmt(m, s.Else)
	for _, v := range s.List {
		a.stmt(m, v)
	}
}
func (a *numberStorage) expr(m *hir.Method, x *hir.Expr) bool {
	if x == nil {
		return false
	}
	left, right, third := a.expr(m, x.X), a.expr(m, x.Y), a.expr(m, x.Z)
	for _, v := range x.Args {
		a.expr(m, v)
	}
	a.stmt(m, x.Stmt)
	tagged := a.exprs[x] || a.tagged[a.slot(m, x)]
	switch x.Kind {
	case hir.RuntimeOp:
		tagged = tagged || nanStorageBoundary(x) || left && strings.HasPrefix(x.Op, "number.")
	case hir.Binary:
		tagged = tagged || left || right
	case hir.Unary, hir.NumericConvert, hir.CheckedNumericConvert:
		tagged = tagged || left
	case hir.Narrow:
		tagged = tagged || left
	case hir.Conditional:
		tagged = tagged || right || third
	case hir.Seq:
		tagged = tagged || right
	case hir.NumericMinMax:
		tagged = tagged || left || right
		for _, v := range x.Args {
			tagged = tagged || a.exprs[v]
		}
	case hir.New, hir.DirectCall, hir.VirtualCall, hir.SuperCall:
		owner, name := x.Owner, x.Name
		// Match body.call: instance receivers, not Expr.Owner, determine
		// dispatch. Super calls have neither Owner nor a receiver in HIR.
		if x.Kind == hir.New {
			owner, name = x.Type.Name, "constructor"
		} else if x.Kind == hir.SuperCall {
			if c := a.classes[a.owners[m]]; c != nil {
				owner = c.Super
			}
			if original, _, specialized := strings.Cut(name, "_instantiated_"); specialized {
				name = original
			}
		} else if x.X != nil {
			owner = x.X.Type.Name
		}
		callee := a.lookupMethod(owner, name)
		if callee != nil {
			for i, v := range x.Args {
				if i < len(callee.Params) && a.exprs[v] {
					a.mark(&callee.Params[i].Type)
				}
			}
			tagged = tagged || a.tagged[&callee.Result]
		}
	}
	if tagged && x.Type.Kind == hir.Number && !a.exprs[x] {
		a.exprs[x] = true
		a.changed = true
	}
	return a.exprs[x]
}

func copyNumberProgram(p *hir.Program) *hir.Program {
	q := *p
	copyMethod := func(m *hir.Method) *hir.Method {
		if m == nil {
			return nil
		}
		n := *m
		n.Params = append([]hir.Param(nil), m.Params...)
		n.Body = copyNumberStmt(m.Body)
		return &n
	}
	q.Classes = make([]*hir.Class, len(p.Classes))
	for i, c := range p.Classes {
		n := *c
		n.Fields = append([]hir.Field(nil), c.Fields...)
		n.Ctor = copyMethod(c.Ctor)
		n.Methods = make([]*hir.Method, len(c.Methods))
		for j, m := range c.Methods {
			n.Methods[j] = copyMethod(m)
		}
		q.Classes[i] = &n
	}
	q.Interfaces = make([]*hir.Interface, len(p.Interfaces))
	for i, c := range p.Interfaces {
		n := *c
		n.Methods = make([]*hir.Method, len(c.Methods))
		for j, m := range c.Methods {
			n.Methods[j] = copyMethod(m)
		}
		q.Interfaces[i] = &n
	}
	return &q
}
func copyNumberStmt(s *hir.Stmt) *hir.Stmt {
	if s == nil {
		return nil
	}
	n := *s
	n.X = copyNumberExpr(s.X)
	n.Y = copyNumberExpr(s.Y)
	n.Body = copyNumberStmt(s.Body)
	n.Else = copyNumberStmt(s.Else)
	n.List = make([]*hir.Stmt, len(s.List))
	for i, v := range s.List {
		n.List[i] = copyNumberStmt(v)
	}
	return &n
}
func copyNumberExpr(x *hir.Expr) *hir.Expr {
	if x == nil {
		return nil
	}
	n := *x
	n.X = copyNumberExpr(x.X)
	n.Y = copyNumberExpr(x.Y)
	n.Z = copyNumberExpr(x.Z)
	n.Stmt = copyNumberStmt(x.Stmt)
	n.Args = make([]*hir.Expr, len(x.Args))
	for i, v := range x.Args {
		n.Args[i] = copyNumberExpr(v)
	}
	return &n
}
