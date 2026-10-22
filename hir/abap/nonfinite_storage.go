package abap

import (
	"maps"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Native f remains the finite fast path. Values reaching a nonfinite-producing
// boundary use the existing tagged numeric box, including across scalar fields
// and call signatures. This is a storage lowering, not ToNumber(undefined)=0.
// Work on a copy: Go emission and the input HIR retain their Number semantics.
func retainNonfiniteStorage(p *hir.Program) (*hir.Program, map[*hir.Expr]bool, map[string]map[string]bool) {
	if !hasNaNBoundary(p) {
		return p, nil, nil
	}
	q := copyNumberProgram(p)
	a := &numberStorage{
		classes:    map[string]*hir.Class{},
		interfaces: map[string]*hir.Interface{},
		methods:    map[string]*hir.Method{},
		owners:     map[*hir.Method]string{},
		locals:     map[*hir.Expr]*hir.Type{},
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
	// Preserve audit/body identity when no storage actually needs lowering.
	if len(a.tagged) == 0 && len(a.exprs) == 0 {
		return p, nil, nil
	}
	// Keep the required-field proof separate from the optional numeric box.
	// Only fields widened by this pass qualify; source optional fields do not.
	fields := map[string]map[string]bool{}
	for _, c := range q.Classes {
		for i := range c.Fields {
			f := &c.Fields[i]
			if a.tagged[&f.Type] {
				if fields[c.Name] == nil {
					fields[c.Name] = map[string]bool{}
				}
				fields[c.Name][f.Name] = true
			}
		}
	}
	for t := range a.tagged {
		*t = hir.T(hir.Optional, hir.T(hir.Number))
	}
	for x := range a.exprs {
		x.Type = hir.T(hir.Optional, hir.T(hir.Number))
	}
	return q, a.exprs, fields
}

// The frontend's NaN and Infinity constants need tagged numeric storage.
// Other parser results acquire tagged storage only when a destination needs
// it; a finite scalar destination keeps its explicit domain checks.
func nanStorageBoundary(x *hir.Expr) bool {
	if x.Kind != hir.RuntimeOp || x.Op != "string.parseFloatNumber" || x.X == nil || x.X.Kind != hir.Lit {
		return false
	}
	return x.X.Value == "NaN" || x.X.Value == "Infinity" || x.X.Value == "-Infinity"
}

func hasNaNBoundary(p *hir.Program) bool {
	found := false
	var expr func(*hir.Expr)
	expr = func(x *hir.Expr) {
		if x == nil || found {
			return
		}
		// Existing Optional<Number> destinations need capture propagation even
		// without an explicit NaN seed elsewhere in the program.
		found = x.Kind == hir.RuntimeOp && (x.Op == "string.parseFloatNumber" || x.Op == "string.parseIntNumber")
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
	locals     map[*hir.Expr]*hir.Type
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
	// Bind each read to its lexical declaration. Sibling blocks and sequence
	// captures can reuse names while requiring different numeric storage.
	var stmt func(*hir.Stmt, map[string]*hir.Type)
	var expr func(*hir.Expr, map[string]*hir.Type)
	expr = func(x *hir.Expr, scope map[string]*hir.Type) {
		if x == nil {
			return
		}
		if x.Kind == hir.Local {
			a.locals[x] = scope[x.Name]
		}
		if x.Kind == hir.Seq {
			scope = maps.Clone(scope)
			for _, child := range x.Stmt.List {
				stmt(child, scope)
			}
		} else {
			stmt(x.Stmt, scope)
		}
		expr(x.X, scope)
		expr(x.Y, scope)
		expr(x.Z, scope)
		for _, v := range x.Args {
			expr(v, scope)
		}
	}
	stmt = func(s *hir.Stmt, scope map[string]*hir.Type) {
		if s == nil {
			return
		}
		if s.Kind == hir.Block {
			scope = maps.Clone(scope)
		}
		expr(s.X, scope)
		expr(s.Y, scope)
		switch s.Kind {
		case hir.VarDecl:
			scope[s.Name] = &s.Type
		case hir.ForEach:
			scope = maps.Clone(scope)
			scope[s.Name] = &s.Type
		case hir.If, hir.While:
			stmt(s.Body, maps.Clone(scope))
			stmt(s.Else, maps.Clone(scope))
			return
		case hir.Try:
			stmt(s.Body, maps.Clone(scope))
			scope = maps.Clone(scope)
			scope[s.Name] = &s.Type
			stmt(s.Else, scope)
			return
		}
		stmt(s.Body, scope)
		stmt(s.Else, scope)
		for _, child := range s.List {
			stmt(child, scope)
		}
	}
	stmt(m.Body, slots)
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
func (a *numberStorage) slot(x *hir.Expr) *hir.Type {
	if x == nil {
		return nil
	}
	if x.Kind == hir.Local {
		return a.locals[x]
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

func (a *numberStorage) taggedSlot(t *hir.Type) bool {
	return t != nil && (a.tagged[t] || taggedNumber(*t))
}

// A destination already using tagged storage also constrains parser captures.
// Keep that requirement across local temporaries introduced for evaluation order;
// evaluating a parser into a native f first would reject NaN before boxing it.
func (a *numberStorage) retainParserStorage(m *hir.Method, x *hir.Expr) {
	if x == nil || x.Type.Kind != hir.Number && !taggedNumber(x.Type) {
		return
	}
	switch x.Kind {
	case hir.Local:
		a.mark(a.slot(x))
	case hir.RuntimeOp:
		if x.Type.Kind == hir.Number && (x.Op == "string.parseIntNumber" || x.Op == "string.parseFloatNumber") && !a.exprs[x] {
			a.exprs[x] = true
			a.changed = true
		}
	case hir.Conditional:
		a.retainParserStorage(m, x.Y)
		a.retainParserStorage(m, x.Z)
	case hir.Seq:
		a.retainParserStorage(m, x.Y)
	}
}

func (a *numberStorage) stmt(m *hir.Method, s *hir.Stmt) {
	if s == nil {
		return
	}
	x, y := a.expr(m, s.X), a.expr(m, s.Y)
	switch s.Kind {
	case hir.VarDecl:
		if a.taggedSlot(&s.Type) {
			a.retainParserStorage(m, s.X)
		}
		if x {
			a.mark(&s.Type)
		}
	case hir.Assign:
		if a.taggedSlot(a.slot(s.X)) {
			a.retainParserStorage(m, s.Y)
		}
		if y {
			a.mark(a.slot(s.X))
		}
	case hir.Return:
		if a.taggedSlot(&m.Result) {
			a.retainParserStorage(m, s.X)
		}
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
	if x.Kind == hir.RuntimeOp {
		switch x.Op {
		case "string.relativeAt", "string.padEnd":
			if len(x.Args) > 0 {
				a.retainParserStorage(m, x.Args[0])
			}
		case "number.abs":
			a.retainParserStorage(m, x.X)
		case "number.isInteger", "number.isSafeInteger", "number.isFinite", "dynamic.values", "dynamic.isMap", "dynamic.isSet":
			if x.X != nil && x.X.Kind == hir.RuntimeOp && x.X.Op == "dynamic.of" {
				a.retainParserStorage(m, x.X.X)
			}
		}
	}
	left, right, third := a.expr(m, x.X), a.expr(m, x.Y), a.expr(m, x.Z)
	for _, v := range x.Args {
		a.expr(m, v)
	}
	a.stmt(m, x.Stmt)
	tagged := a.exprs[x] || a.tagged[a.slot(x)]
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
				if i < len(callee.Params) && a.taggedSlot(&callee.Params[i].Type) {
					a.retainParserStorage(m, v)
				}
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

// semanticFieldType preserves JavaScript requiredness after numeric storage
// lowering. Use the emitted field type for values, this type for presence proofs.
func (e *emitter) semanticFieldType(c *hir.Class, f hir.Field) hir.Type {
	if e.numberFields[c.Name][f.Name] && taggedNumber(f.Type) {
		return hir.T(hir.Number)
	}
	return f.Type
}
