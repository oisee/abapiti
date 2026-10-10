package hir

import "strings"

// Singleton-argument specialisation. abaplint's combinators call
// `runnable.run([input])` once per input: every call builds a one-element
// array and the callee loops over that single row. For a method m whose only
// parameter is an array that it only iterates (prologue; for x of r; epilogue,
// r used nowhere else, no break/continue of that loop), this pass adds
// m_one(x) = prologue; body; epilogue, run once for x, and rewrites
// `recv.m([e])` into `recv.m_one(e)`. Every other declaration of m with the
// same shape gets m_one(x) = m([x]), so dispatch stays exact whatever the
// receiver is; interfaces declaring m declare m_one too.

const singletonSuffix = "_one"

// SingletonStats is what the pass did.
type SingletonStats struct {
	Methods   int // names specialised
	Clones    int // m_one with the loop body
	Defaults  int // m_one forwarding to m([x])
	CallSites int // recv.m([e]) rewritten
	Names     []string
}

// Singleton runs the pass in place.
func Singleton(p *Program) SingletonStats {
	var st SingletonStats
	// family: method name + the array type of the singleton literal passed.
	families := map[string]bool{}
	walkProgram(p, nil, func(x *Expr) {
		if x.Kind == VirtualCall && len(x.Args) == 1 {
			if _, ok := singletonElem(x.Args[0]); ok {
				families[x.Name+"\x00"+x.Args[0].Type.String()] = true
			}
		}
	})
	for _, key := range sortedKeys(families) {
		cut := strings.IndexByte(key, 0)
		name, arr := key[:cut], key[cut+1:]
		member := func(m *Method) bool {
			return m.Name == name && arrayParam(m) && m.Params[0].Type.String() == arr
		}
		if !singletonFamily(p, name, member) {
			continue
		}
		st.Methods++
		st.Names = append(st.Names, name)
		// Rewrite the existing call sites first: the forwarding defaults added
		// below call m([x]) on purpose and must stay calls of m.
		walkProgram(p, nil, func(x *Expr) {
			if x.Kind != VirtualCall || x.Name != name || len(x.Args) != 1 || x.Args[0].Type.String() != arr {
				return
			}
			if e, ok := singletonElem(x.Args[0]); ok {
				x.Name = name + singletonSuffix
				x.Args = []*Expr{e}
				st.CallSites++
			}
		})
		for _, c := range p.Classes {
			for _, m := range c.Methods {
				if !member(m) {
					continue
				}
				one := &Method{Node: m.Node, Name: name + singletonSuffix, Params: []Param{{Name: "x", Type: m.Params[0].Type.Args[0]}},
					Result: m.Result, Virtual: m.Virtual, Abstract: m.Abstract, Internal: m.Internal}
				if !m.Abstract {
					if body, ok := singletonClone(m); ok {
						one.Body = body
						st.Clones++
					} else {
						one.Body = forwardOne(c, m)
						st.Defaults++
					}
				}
				c.Methods = append(c.Methods, one)
			}
		}
		for _, in := range p.Interfaces {
			for _, m := range in.Methods {
				if member(m) {
					in.Methods = append(in.Methods, &Method{Node: m.Node, Name: name + singletonSuffix,
						Params: []Param{{Name: "x", Type: m.Params[0].Type.Args[0]}}, Result: m.Result, Virtual: m.Virtual, Abstract: true})
				}
			}
		}
	}
	return st
}

// singletonFamily: the family's declarations are instance methods and no
// class or interface declares name_one yet.
func singletonFamily(p *Program, name string, member func(*Method) bool) bool {
	found := false
	check := func(m *Method) bool {
		if m.Name == name+singletonSuffix {
			return false
		}
		if member(m) {
			if m.Static {
				return false
			}
			found = true
		}
		return true
	}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if !check(m) {
				return false
			}
		}
	}
	for _, in := range p.Interfaces {
		for _, m := range in.Methods {
			if !check(m) {
				return false
			}
		}
	}
	return found
}

func arrayParam(m *Method) bool {
	return len(m.Params) == 1 && !m.Params[0].Variadic && m.Params[0].Type.Kind == Array && len(m.Params[0].Type.Args) == 1
}

// singletonElem matches the lowered literal `[e]`:
// seq { var a = new Array; array.push(a, e) } a.
func singletonElem(x *Expr) (*Expr, bool) {
	if x == nil || x.Kind != Seq || x.Stmt == nil || x.Y == nil || x.Y.Kind != Local {
		return nil, false
	}
	list := x.Stmt.List
	if x.Stmt.Kind != Block || len(list) != 2 {
		return nil, false
	}
	decl, push := list[0], list[1]
	if decl.Kind != VarDecl || decl.Name != x.Y.Name || decl.X == nil || decl.X.Kind != New || decl.Type.Kind != Array {
		return nil, false
	}
	if push.Kind != ExprStmt || push.X == nil || push.X.Kind != RuntimeOp || push.X.Op != "array.push" ||
		push.X.X == nil || push.X.X.Kind != Local || push.X.X.Name != decl.Name || len(push.X.Args) != 1 {
		return nil, false
	}
	return push.X.Args[0], true
}

// singletonClone builds m_one's body when m's body is
// prologue; foreach v in r { body }; epilogue with r used only there. The
// parameter is x in every declaration (ABAP keeps parameter names across
// redefinitions), so the loop body starts with `var v = x`.
func singletonClone(m *Method) (*Stmt, bool) {
	if m.Body == nil || m.Body.Kind != Block {
		return nil, false
	}
	r := m.Params[0].Name
	loop := -1
	for i, s := range m.Body.List {
		if s.Kind == ForEach && s.X != nil && s.X.Kind == Local && s.X.Name == r {
			if loop >= 0 {
				return nil, false
			}
			loop = i
		}
	}
	if loop < 0 {
		return nil, false
	}
	uses := 0
	walk(m.Body, func(*Stmt) {}, func(x *Expr) {
		if x.Kind == Local && (x.Name == r || x.Name == "x") {
			uses++
		}
	})
	each := m.Body.List[loop]
	elem := m.Params[0].Type.Args[0]
	if uses != 1 || r == "x" || !each.Type.Equal(elem) || breaksOuter(each.Body, 0) || declares(m.Body, each.Name, each) || declares(m.Body, "x", nil) {
		return nil, false
	}
	list := make([]*Stmt, 0, len(m.Body.List))
	for i, s := range m.Body.List {
		if i != loop {
			list = append(list, cloneStmt(s))
			continue
		}
		body := cloneStmt(each.Body)
		inner := []*Stmt{{Node: each.Node, Kind: VarDecl, Name: each.Name, Type: each.Type, X: V("x", elem)}}
		if body != nil && body.Kind == Block {
			inner = append(inner, body.List...)
		} else if body != nil {
			inner = append(inner, body)
		}
		list = append(list, &Stmt{Node: each.Node, Kind: Block, List: inner})
	}
	return &Stmt{Node: m.Body.Node, Kind: Block, List: list}, true
}

// breaksOuter reports a break/continue that targets the loop being unrolled
// (depth 0); nested loops own theirs.
func breaksOuter(s *Stmt, depth int) bool {
	if s == nil {
		return false
	}
	switch s.Kind {
	case Break, Continue:
		return depth == 0
	case While, ForEach:
		depth++
	}
	if breaksOuter(s.Body, depth) || breaksOuter(s.Else, depth) {
		return true
	}
	for _, x := range s.List {
		if breaksOuter(x, depth) {
			return true
		}
	}
	found := false
	for _, x := range []*Expr{s.X, s.Y} {
		walkExpr(x, func(t *Stmt) {
			if !found && breaksOuter(t, depth) {
				found = true
			}
		}, func(*Expr) {})
	}
	return found
}

// declares: another declaration of name in the method would clash with the
// loop variable once it becomes the parameter.
func declares(body *Stmt, name string, loop *Stmt) bool {
	clash := false
	walk(body, func(s *Stmt) {
		if s != loop && (s.Kind == VarDecl || s.Kind == ForEach) && s.Name == name {
			clash = true
		}
	}, func(*Expr) {})
	return clash
}

// forwardOne is m_one(x) = this.m([x]) for a declaration that does not qualify.
func forwardOne(c *Class, m *Method) *Stmt {
	arr := m.Params[0].Type
	tmp := "singleton_arg"
	lit := &Expr{Kind: Seq, Type: arr, Y: V(tmp, arr), Stmt: B(
		&Stmt{Kind: VarDecl, Name: tmp, Type: arr, X: &Expr{Kind: New, Type: arr}},
		&Stmt{Kind: ExprStmt, X: &Expr{Kind: RuntimeOp, Op: "array.push", Type: T(I32), X: V(tmp, arr), Args: []*Expr{V("x", arr.Args[0])}}},
	)}
	call := &Expr{Kind: VirtualCall, Type: m.Result, Name: m.Name, X: &Expr{Kind: This, Type: Ref(c.Name)}, Args: []*Expr{lit}}
	if m.Result.Kind == Void {
		return B(&Stmt{Kind: ExprStmt, X: call})
	}
	return B(&Stmt{Kind: Return, X: call})
}

func cloneStmt(s *Stmt) *Stmt {
	if s == nil {
		return nil
	}
	c := *s
	c.X, c.Y = cloneExpr(s.X), cloneExpr(s.Y)
	c.Body, c.Else = cloneStmt(s.Body), cloneStmt(s.Else)
	if s.List != nil {
		c.List = make([]*Stmt, len(s.List))
		for i, x := range s.List {
			c.List[i] = cloneStmt(x)
		}
	}
	return &c
}

func cloneExpr(x *Expr) *Expr {
	if x == nil {
		return nil
	}
	c := *x
	c.X, c.Y, c.Z = cloneExpr(x.X), cloneExpr(x.Y), cloneExpr(x.Z)
	if x.Args != nil {
		c.Args = make([]*Expr, len(x.Args))
		for i, a := range x.Args {
			c.Args[i] = cloneExpr(a)
		}
	}
	if x.Range != nil {
		r := *x.Range
		c.Range = &r
	}
	c.Stmt = cloneStmt(x.Stmt)
	return &c
}

func walkProgram(p *Program, fs func(*Stmt), fx func(*Expr)) {
	if fs == nil {
		fs = func(*Stmt) {}
	}
	for _, c := range p.Classes {
		if c.Ctor != nil {
			walk(c.Ctor.Body, fs, fx)
		}
		for _, m := range c.Methods {
			walk(m.Body, fs, fx)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
