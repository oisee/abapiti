// Test oracle only: mechanically relocated from 1f84049419b22f062573ebbe7f30c48cfcecfae2 hir/inline.go.
package inlineoracle

import (
	"fmt"
	. "github.com/oisee/abapiti/hir"
	"strings"
)

// InlineMaxStmts bounds a callee body (statements counted recursively,
// blocks excluded) for inlining.
const InlineMaxStmts = 12

// Inline replaces calls of small, non-overridden instance methods by their
// bodies (target independent; method calls are expensive on the ABAP kernel).
// A VirtualCall whose receiver is statically a class C is inlined when the
// method it resolves to on C is small, straight-line apart from guarded
// returns, not recursive and redefined by no class at or below C. The
// receiver and the arguments are evaluated once, left to right, into fresh
// locals before the body. It returns the number of inlined call sites.
func Inline(p *Program) int {
	n, _ := InlineStats(p)
	return n
}

// InlineStats is Inline that also reports inlined call sites per callee
// ("Class.method").
func InlineStats(p *Program) (int, map[string]int) {
	in := &inliner{p: p, classes: map[string]*Class{}, children: map[string][]*Class{}, owner: map[*Method]*Class{},
		candidate: map[*Method]bool{}, state: map[*Method]int{}, tmpl: map[*Method]*template{}, rewritten: map[*Method]bool{},
		overridden: map[string]bool{}, stats: map[string]int{}}
	in.init()
	for _, c := range p.Classes {
		if c.Ctor != nil {
			in.rewriteMethod(c.Ctor)
		}
		for _, m := range c.Methods {
			in.rewriteMethod(m)
		}
	}
	return in.count, in.stats
}

type template struct {
	owner *Class
	m     *Method
	value *Expr   // non-void result
	stmts []*Stmt // void method used as a statement
	// literal marks parameters a literal argument may replace (never assigned).
	literal map[string]bool
}

type inliner struct {
	p          *Program
	classes    map[string]*Class
	children   map[string][]*Class
	owner      map[*Method]*Class
	candidate  map[*Method]bool
	state      map[*Method]int
	tmpl       map[*Method]*template
	rewritten  map[*Method]bool
	overridden map[string]bool
	prefix     string
	serial     int
	count      int
	stats      map[string]int
}

func (in *inliner) init() {
	for _, c := range in.p.Classes {
		in.classes[c.Name] = c
	}
	for _, c := range in.p.Classes {
		if c.Super != "" {
			in.children[c.Super] = append(in.children[c.Super], c)
		}
		for _, m := range c.Methods {
			in.owner[m] = c
		}
	}
	// A prefix no existing local, parameter or binding starts with.
	in.prefix = "inl_"
	for in.prefixUsed() {
		in.prefix = "x" + in.prefix
	}
	for _, c := range in.p.Classes {
		for _, m := range c.Methods {
			in.candidate[m] = in.eligible(c, m)
		}
	}
	in.dropRecursive()
}

func (in *inliner) prefixUsed() bool {
	used := false
	visit := func(name string) {
		if strings.HasPrefix(name, in.prefix) {
			used = true
		}
	}
	for _, c := range in.p.Classes {
		ms := append([]*Method{}, c.Methods...)
		if c.Ctor != nil {
			ms = append(ms, c.Ctor)
		}
		for _, m := range ms {
			for _, p := range m.Params {
				visit(p.Name)
			}
			walk(m.Body, func(s *Stmt) {
				if s.Kind == VarDecl || s.Kind == ForEach || s.Kind == Try {
					visit(s.Name)
				}
			}, func(x *Expr) {
				if x.Kind == Local {
					visit(x.Name)
				}
			})
		}
	}
	return used
}

// walk visits every statement and expression, including Seq statements.
func walk(s *Stmt, fs func(*Stmt), fx func(*Expr)) {
	if s == nil {
		return
	}
	fs(s)
	walkExpr(s.X, fs, fx)
	walkExpr(s.Y, fs, fx)
	walk(s.Body, fs, fx)
	walk(s.Else, fs, fx)
	for _, x := range s.List {
		walk(x, fs, fx)
	}
}
func walkExpr(x *Expr, fs func(*Stmt), fx func(*Expr)) {
	if x == nil {
		return
	}
	fx(x)
	walkExpr(x.X, fs, fx)
	walkExpr(x.Y, fs, fx)
	walkExpr(x.Z, fs, fx)
	for _, a := range x.Args {
		walkExpr(a, fs, fx)
	}
	walk(x.Stmt, fs, fx)
}

// eligible checks the callee alone: shape, size and forbidden constructs.
func (in *inliner) eligible(c *Class, m *Method) bool {
	if m.Static || m.Abstract || !m.Virtual || m.Body == nil || m.Body.Kind != Block || m.Name == "class_constructor" || m.Name == "constructor" || c.Ctor == m || strings.Contains(m.Name, "_instantiated_") {
		return false
	}
	for _, p := range m.Params {
		if p.Variadic {
			return false
		}
	}
	ok, stmts := true, 0
	var inSeq func(x *Expr)
	inSeq = func(x *Expr) {
		// A return inside an expression's statements would leave the caller.
		walk(x.Stmt, func(s *Stmt) {
			if s.Kind == Return {
				ok = false
			}
		}, func(*Expr) {})
	}
	walk(m.Body, func(s *Stmt) {
		switch s.Kind {
		case Block:
		case Trap, Throw, Try, Finally, While, ForEach, Break, Continue:
			ok = false
		default:
			stmts++
		}
	}, func(x *Expr) {
		if x.Kind == SuperCall {
			ok = false
		}
		if x.Kind == Seq {
			inSeq(x)
		}
	})
	return ok && stmts <= InlineMaxStmts
}

// notOverridden: no class at or below c redefines name (closed world), and no
// checker-typed variant replaces the body between c and the declaring class.
func (in *inliner) notOverridden(c *Class, name string, m *Method) bool {
	key := c.Name + "\x00" + name
	if r, ok := in.overridden[key]; ok {
		return !r
	}
	found := false
	var down func(k *Class)
	down = func(k *Class) {
		for _, x := range k.Methods {
			if (x.Name == name && x != m) || strings.HasPrefix(x.Name, name+"_instantiated_") {
				found = true
			}
		}
		for _, ch := range in.children[k.Name] {
			down(ch)
		}
	}
	down(c)
	for k := in.classes[c.Super]; k != nil && !found && c != in.owner[m]; k = in.classes[k.Super] {
		for _, x := range k.Methods {
			if strings.HasPrefix(x.Name, name+"_instantiated_") {
				found = true
			}
		}
		if k == in.owner[m] {
			break
		}
	}
	in.overridden[key] = found
	return !found
}

// resolve returns the inlinable callee of a call, or nil.
func (in *inliner) resolve(x *Expr) *Method {
	if x == nil || x.Kind != VirtualCall || x.X == nil || x.X.Type.Kind != ClassRef {
		return nil
	}
	c := in.classes[x.X.Type.Name]
	if c == nil {
		return nil
	}
	var m *Method
	for k := c; k != nil && m == nil; k = in.classes[k.Super] {
		for _, f := range k.Methods {
			if f.Name == x.Name {
				m = f
				break
			}
		}
	}
	if m == nil || !in.candidate[m] || len(x.Args) != len(m.Params) || !in.notOverridden(c, x.Name, m) {
		return nil
	}
	return m
}

// dropRecursive removes candidates on a cycle of candidate calls (Tarjan).
func (in *inliner) dropRecursive() {
	edges := map[*Method][]*Method{}
	var nodes []*Method
	for _, c := range in.p.Classes {
		for _, m := range c.Methods {
			if !in.candidate[m] {
				continue
			}
			nodes = append(nodes, m)
			walk(m.Body, func(*Stmt) {}, func(x *Expr) {
				if f := in.resolve(x); f != nil {
					edges[m] = append(edges[m], f)
				}
			})
		}
	}
	index, low, on := map[*Method]int{}, map[*Method]int{}, map[*Method]bool{}
	var stack, drop []*Method
	next := 0
	var connect func(m *Method)
	connect = func(m *Method) {
		index[m], low[m] = next, next
		next++
		stack = append(stack, m)
		on[m] = true
		self := false
		for _, f := range edges[m] {
			if f == m {
				self = true
			}
			if _, seen := index[f]; !seen {
				connect(f)
				low[m] = min(low[m], low[f])
			} else if on[f] {
				low[m] = min(low[m], index[f])
			}
		}
		if low[m] == index[m] {
			var scc []*Method
			for {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[top] = false
				scc = append(scc, top)
				if top == m {
					break
				}
			}
			if len(scc) > 1 || self {
				drop = append(drop, scc...)
			}
		}
	}
	for _, m := range nodes {
		if _, seen := index[m]; !seen {
			connect(m)
		}
	}
	for _, m := range drop {
		in.candidate[m] = false
	}
}

func trapStub(m *Method) bool {
	return m.Body != nil && m.Body.Kind == Block && len(m.Body.List) == 1 && m.Body.List[0].Kind == Trap
}

// rewriteMethod inlines the calls in a body once (callees first).
func (in *inliner) rewriteMethod(m *Method) {
	if in.rewritten[m] || m.Body == nil || trapStub(m) {
		return
	}
	in.rewritten[m] = true
	m.Body = in.stmt(m.Body)
}

// prepare returns the callee's template (its own calls inlined first), or nil.
func (in *inliner) prepare(m *Method) *template {
	if in.state[m] != 0 {
		return in.tmpl[m]
	}
	in.state[m] = 1
	in.rewriteMethod(m)
	t := &template{owner: in.owner[m], m: m, literal: map[string]bool{}}
	b := &builder{result: m.Result, decls: map[string]int{}}
	for _, p := range m.Params {
		b.decls[p.Name]++
		t.literal[p.Name] = true
	}
	walk(m.Body, func(s *Stmt) {
		if s.Kind == VarDecl {
			b.decls[s.Name]++
		}
		if s.Kind == Assign && s.X != nil && s.X.Kind == Local {
			t.literal[s.X.Name] = false
		}
	}, func(*Expr) {})
	ok := false
	if m.Result.Kind == Void {
		t.stmts, ok = b.toStmts(m.Body.List)
	} else {
		t.value, ok = b.toExpr(m.Body.List)
	}
	if ok {
		in.tmpl[m] = t
	}
	in.state[m] = 2
	return in.tmpl[m]
}

func (in *inliner) stmt(s *Stmt) *Stmt {
	if s == nil {
		return nil
	}
	if s.Kind == ExprStmt && s.X != nil && s.X.Type.Kind == Void {
		in.operands(s.X)
		if m := in.resolve(s.X); m != nil {
			if t := in.prepare(m); t != nil && t.stmts != nil {
				return in.expandVoid(s, t)
			}
		}
		return s
	}
	s.X = in.expr(s.X)
	s.Y = in.expr(s.Y)
	s.Body = in.stmt(s.Body)
	s.Else = in.stmt(s.Else)
	for i, x := range s.List {
		s.List[i] = in.stmt(x)
	}
	return s
}

// operands rewrites the operands of x, not x itself.
func (in *inliner) operands(x *Expr) {
	x.X = in.expr(x.X)
	x.Y = in.expr(x.Y)
	x.Z = in.expr(x.Z)
	for i, a := range x.Args {
		x.Args[i] = in.expr(a)
	}
	x.Stmt = in.stmt(x.Stmt)
}

func (in *inliner) expr(x *Expr) *Expr {
	if x == nil {
		return nil
	}
	in.operands(x)
	if x.Type.Kind == Void {
		return x
	}
	if m := in.resolve(x); m != nil {
		if t := in.prepare(m); t != nil && t.value != nil {
			return in.expandValue(x, t)
		}
	}
	return x
}

// site is one inlined call: the bindings of the callee's names.
type site struct {
	in   *inliner
	n    int
	node Node
	self *Expr
	used map[string]bool
}

// bind evaluates the receiver and the arguments into fresh locals, left to
// right; `this` of the caller's own class and never-assigned parameters
// receiving a literal are substituted instead.
func (in *inliner) bind(call *Expr, t *template) (*site, []*Stmt, map[string]*Expr) {
	in.serial++
	in.count++
	in.stats[t.owner.Name+"."+t.m.Name]++
	st := &site{in: in, n: in.serial, used: map[string]bool{}}
	st.node = Node{ID: call.ID, Source: strings.TrimSpace(call.Source + " inlined " + t.owner.Name + "." + t.m.Name)}
	var decls []*Stmt
	self := Ref(t.owner.Name)
	if call.X.Kind == This && call.X.Type.Equal(self) {
		st.self = &Expr{Node: call.X.Node, Kind: This, Type: self}
	} else {
		name := st.fresh("self")
		decls = append(decls, &Stmt{Node: call.Node, Kind: VarDecl, Name: name, Type: self, X: call.X})
		st.self = V(name, self)
	}
	env := map[string]*Expr{}
	for i, p := range t.m.Params {
		a := call.Args[i]
		if a.Kind == Lit && t.literal[p.Name] && a.Type.Equal(p.Type) {
			env[p.Name] = a
			continue
		}
		name := st.fresh(p.Name)
		decls = append(decls, &Stmt{Node: call.Node, Kind: VarDecl, Name: name, Type: p.Type, X: a})
		env[p.Name] = V(name, p.Type)
	}
	return st, decls, env
}

func (in *inliner) expandValue(call *Expr, t *template) *Expr {
	st, decls, env := in.bind(call, t)
	y := st.expr(t.value, env)
	if len(decls) == 0 {
		return y
	}
	return &Expr{Node: st.node, Kind: Seq, Type: call.Type, Stmt: &Stmt{Node: st.node, Kind: Block, List: decls}, Y: y}
}

func (in *inliner) expandVoid(s *Stmt, t *template) *Stmt {
	st, decls, env := in.bind(s.X, t)
	list := decls
	for _, x := range t.stmts {
		list = append(list, st.stmt(x, env))
	}
	return &Stmt{Node: st.node, Kind: Block, List: list}
}

func (st *site) fresh(name string) string {
	base := fmt.Sprintf("%s%d_%s", st.in.prefix, st.n, name)
	n := base
	for k := 2; st.used[n]; k++ {
		n = fmt.Sprintf("%s_%d", base, k)
	}
	st.used[n] = true
	return n
}

func scope(env map[string]*Expr) map[string]*Expr {
	r := make(map[string]*Expr, len(env))
	for k, v := range env {
		r[k] = v
	}
	return r
}

// expr deep-copies a callee expression into the call site.
func (st *site) expr(x *Expr, env map[string]*Expr) *Expr {
	if x == nil {
		return nil
	}
	switch x.Kind {
	case This:
		c := *st.self
		c.Node = st.node
		return &c
	case Local:
		if r, ok := env[x.Name]; ok {
			return st.expr(r, nil)
		}
	}
	c := *x
	c.Node = st.node
	if x.Range != nil {
		r := *x.Range
		c.Range = &r
	}
	if x.Args != nil {
		c.Args = make([]*Expr, len(x.Args))
		for i, a := range x.Args {
			c.Args[i] = st.expr(a, env)
		}
	}
	if x.Kind == Seq {
		inner := scope(env)
		c.Stmt = st.list(x.Stmt, inner)
		c.Y = st.expr(x.Y, inner)
		c.X = st.expr(x.X, env)
		c.Z = st.expr(x.Z, env)
		return &c
	}
	c.X = st.expr(x.X, env)
	c.Y = st.expr(x.Y, env)
	c.Z = st.expr(x.Z, env)
	c.Stmt = st.stmt(x.Stmt, env)
	return &c
}

// list copies a block whose declarations stay visible in env (Seq).
func (st *site) list(s *Stmt, env map[string]*Expr) *Stmt {
	if s == nil {
		return nil
	}
	c := *s
	c.Node = st.node
	c.List = make([]*Stmt, len(s.List))
	for i, x := range s.List {
		c.List[i] = st.stmt(x, env)
	}
	return &c
}

// stmt deep-copies a statement; a declaration renames into env.
func (st *site) stmt(s *Stmt, env map[string]*Expr) *Stmt {
	if s == nil {
		return nil
	}
	if s.Kind == Block {
		return st.list(s, scope(env))
	}
	c := *s
	c.Node = st.node
	c.X = st.expr(s.X, env)
	c.Y = st.expr(s.Y, env)
	c.Body = st.stmt(s.Body, scope(env))
	c.Else = st.stmt(s.Else, scope(env))
	if s.List != nil {
		c.List = make([]*Stmt, len(s.List))
		inner := scope(env)
		for i, x := range s.List {
			c.List[i] = st.stmt(x, inner)
		}
	}
	if s.Kind == VarDecl {
		c.Name = st.fresh(s.Name)
		env[s.Name] = V(c.Name, s.Type)
	}
	return &c
}

// builder turns a callee body into an expression (or a return-free statement
// list for void methods).
type builder struct {
	result Type
	decls  map[string]int // declarations per name in the callee (params too)
}

func stmtsOf(s *Stmt) []*Stmt {
	if s == nil {
		return nil
	}
	if s.Kind == Block {
		return s.List
	}
	return []*Stmt{s}
}

// flat reports whether a block's declarations may join the enclosing list:
// each is the only declaration of its name in the callee.
func (b *builder) flat(s *Stmt) bool {
	for _, x := range stmtsOf(s) {
		if x.Kind == VarDecl && b.decls[x.Name] != 1 {
			return false
		}
	}
	return true
}

func hasReturn(s *Stmt) bool {
	found := false
	walk(s, func(x *Stmt) {
		if x.Kind == Return {
			found = true
		}
	}, func(*Expr) {})
	return found
}

// returnsAll: every path through s ends in a return.
func returnsAll(s *Stmt) bool {
	if s == nil {
		return false
	}
	switch s.Kind {
	case Return:
		return true
	case Block:
		for _, x := range s.List {
			if returnsAll(x) {
				return true
			}
		}
	case If:
		return returnsAll(s.Body) && returnsAll(s.Else)
	}
	return false
}

func concat(a, b []*Stmt) []*Stmt {
	return append(append([]*Stmt{}, a...), b...)
}

func (b *builder) seq(pre []*Stmt, y *Expr) *Expr {
	if len(pre) == 0 {
		return y
	}
	return &Expr{Node: y.Node, Kind: Seq, Type: b.result, Stmt: B(pre...), Y: y}
}

func (b *builder) toExpr(list []*Stmt) (*Expr, bool) {
	var pre []*Stmt
	for i, s := range list {
		rest := list[i+1:]
		switch s.Kind {
		case Return:
			if s.X == nil || !s.X.Type.Equal(b.result) {
				return nil, false
			}
			return b.seq(pre, s.X), true
		case VarDecl, Assign, ExprStmt:
			pre = append(pre, s)
		case Block:
			if !b.flat(s) {
				return nil, false
			}
			y, ok := b.toExpr(concat(s.List, rest))
			if !ok {
				return nil, false
			}
			return b.seq(pre, y), true
		case If:
			if !hasReturn(s.Body) && !hasReturn(s.Else) {
				pre = append(pre, s)
				continue
			}
			var y, z *Expr
			okY, okZ := false, false
			switch {
			case returnsAll(s.Body):
				if !b.flat(s.Else) {
					return nil, false
				}
				y, okY = b.toExpr(stmtsOf(s.Body))
				z, okZ = b.toExpr(concat(stmtsOf(s.Else), rest))
			case returnsAll(s.Else):
				if !b.flat(s.Body) {
					return nil, false
				}
				y, okY = b.toExpr(concat(stmtsOf(s.Body), rest))
				z, okZ = b.toExpr(stmtsOf(s.Else))
			}
			if !okY || !okZ {
				return nil, false
			}
			return b.seq(pre, &Expr{Node: s.Node, Kind: Conditional, Type: b.result, X: s.X, Y: y, Z: z}), true
		default:
			return nil, false
		}
	}
	return nil, false
}

// toStmts lowers a void body to statements without returns.
func (b *builder) toStmts(list []*Stmt) ([]*Stmt, bool) {
	out := []*Stmt{}
	for i, s := range list {
		rest := list[i+1:]
		switch s.Kind {
		case Return:
			if s.X != nil {
				if s.X.Type.Kind != Void {
					return nil, false
				}
				out = append(out, &Stmt{Node: s.Node, Kind: ExprStmt, X: s.X})
			}
			return out, true
		case VarDecl, Assign, ExprStmt:
			out = append(out, s)
		case Block:
			if !b.flat(s) {
				return nil, false
			}
			tail, ok := b.toStmts(concat(s.List, rest))
			return append(out, tail...), ok
		case If:
			if !hasReturn(s.Body) && !hasReturn(s.Else) {
				out = append(out, s)
				continue
			}
			var y, z []*Stmt
			okY, okZ := false, false
			switch {
			case returnsAll(s.Body):
				if !b.flat(s.Else) {
					return nil, false
				}
				y, okY = b.toStmts(stmtsOf(s.Body))
				z, okZ = b.toStmts(concat(stmtsOf(s.Else), rest))
			case returnsAll(s.Else):
				if !b.flat(s.Body) {
					return nil, false
				}
				y, okY = b.toStmts(concat(stmtsOf(s.Body), rest))
				z, okZ = b.toStmts(stmtsOf(s.Else))
			}
			if !okY || !okZ {
				return nil, false
			}
			return append(out, &Stmt{Node: s.Node, Kind: If, X: s.X, Body: B(y...), Else: B(z...)}), true
		default:
			return nil, false
		}
	}
	return out, true
}
