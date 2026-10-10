package rewrite

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

type binding struct {
	id      string
	classes []string
	fresh   bool
}
type extractor struct {
	db            *DB
	classes       map[string]*hir.Class
	owner, method string
	assigned      map[string]bool
	constants     map[FieldKey]bool
	initializers  map[string]bool
}
type scope map[string]binding

func copyScope(s scope) scope {
	out := scope{}
	for k, v := range s {
		out[k] = v
	}
	return out
}
func (x *extractor) add(p string, a ...string) {
	if err := x.db.Add(p, a...); err != nil {
		panic(err)
	}
}
func methodID(c, m string) string { return c + "::" + m }
func (x *extractor) resolve(c, m string) string {
	for cl := x.classes[c]; cl != nil; cl = x.classes[cl.Super] {
		if m == "constructor" && cl.Ctor != nil {
			return methodID(cl.Name, m)
		}
		for _, mm := range cl.Methods {
			if mm.Name == m {
				return methodID(cl.Name, m)
			}
		}
	}
	return ""
}
func (x *extractor) fieldOwner(c, f string, static bool) string {
	for cl := x.classes[c]; cl != nil; cl = x.classes[cl.Super] {
		for _, ff := range cl.Fields {
			if ff.Name == f && ff.Static == static {
				return cl.Name
			}
		}
	}
	return c
}
func (x *extractor) init(c, site string) {
	if !x.initializers[c] {
		return
	}
	x.add("ensure_init", x.method, site, c)
	x.add("calls", x.method, methodID(c, "class_constructor"), site+"/init/"+c)
}

// Extract expects verified HIR. Site identities are method-qualified structural
// paths, independent of optional Node IDs. Abstract declarations have no body.
func Extract(p *hir.Program) *DB {
	return extractDemanded(p, nil)
}

func extractDemanded(p *hir.Program, demanded map[string]bool) *DB {
	db := NewDB()
	db.demanded = demanded
	x := &extractor{db: db, classes: map[string]*hir.Class{}}
	for _, c := range p.Classes {
		x.classes[c.Name] = c
	}
	syntaxOnly := demanded != nil
	for pred := range demanded {
		if pred != "dispatch" && pred != "defined" && pred != "static" && (len(pred) < 7 || pred[:7] != "inline_") && pred != "node" {
			syntaxOnly = false
		}
	}
	if !syntaxOnly {
		x.findInitializers(p)
		x.addEffects()
		for _, a := range p.Interfaces {
			for _, b := range p.Interfaces {
				if interfaceAccepts(a, b) {
					x.add("interface_subtype", a.Name, b.Name)
				}
			}
		}
	}
	for _, c := range p.Classes {
		x.add("class", c.Name)
		if !c.Abstract {
			x.add("concrete", c.Name)
		}
		if c.Super != "" {
			x.add("extends", c.Name, c.Super)
		}
		for _, i := range c.Implements {
			x.add("implements", c.Name, i)
		}
		leaf := true
		if !syntaxOnly {
			for _, d := range p.Classes {
				if d.Super == c.Name {
					leaf = false
				}
			}
			if leaf {
				x.add("final", c.Name)
			}
		}
		for cl := c; cl != nil; cl = x.classes[cl.Super] {
			for _, m := range cl.Methods {
				target := x.resolve(c.Name, m.Name)
				if target != "" {
					x.add("dispatch", c.Name, m.Name, target)
				}
			}
		}
	}
	for _, c := range p.Classes {
		x.owner = c.Name
		methods := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			methods = append(methods, c.Ctor)
		}
		for _, m := range methods {
			name := m.Name
			if m == c.Ctor {
				name = "constructor"
			}
			x.method = methodID(c.Name, name)
			x.add("method", c.Name, x.method)
			if m.Static {
				x.add("static", c.Name, x.method)
			}
			if m.Abstract {
				continue
			}
			x.add("defined", x.method)
			if syntaxOnly {
				continue
			}
			env := scope{}
			if !m.Static {
				env["this"] = binding{id: x.method + "/this"}
				x.add("param", x.method, "this", env["this"].id)
			}
			for i, p := range m.Params {
				b := binding{id: x.method + "/param/" + p.Name}
				env[p.Name] = b
				x.add("param", x.method, strconv.Itoa(i), b.id)
				x.add("local", x.method, b.id, p.Type.String())
			}
			if (m.Static || m == c.Ctor || name == "constructor") && name != "class_constructor" {
				x.init(c.Name, x.method+"/entry")
			}
			x.assigned = map[string]bool{}
			assignedNames(m.Body, x.assigned)
			x.flowFacts(m, env)
			x.stmt(m.Body, x.method+"/body", env)
			x.shapes(m.Body)
			if name == "class_constructor" {
				x.add("implicit_init", c.Name)
			}
		}
	}
	if !syntaxOnly {
		x.recursiveCalls()
	}
	return x.db
}
func assignedNames(s *hir.Stmt, out map[string]bool) {
	if s == nil {
		return
	}
	if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.Local {
		out[s.X.Name] = true
	}
	var expr func(*hir.Expr)
	expr = func(e *hir.Expr) {
		if e == nil {
			return
		}
		assignedNames(e.Stmt, out)
		expr(e.X)
		expr(e.Y)
		expr(e.Z)
		for _, a := range e.Args {
			expr(a)
		}
	}
	expr(s.X)
	expr(s.Y)
	for _, b := range s.List {
		assignedNames(b, out)
	}
	assignedNames(s.Body, out)
	assignedNames(s.Else, out)
}
func (x *extractor) proof(e *hir.Expr, env scope) ([]string, bool) {
	if e == nil {
		return nil, false
	}
	switch e.Kind {
	case hir.New:
		if e.Type.Kind == hir.ClassRef {
			return []string{e.Type.Name}, true
		}
		return nil, true
	case hir.RuntimeOp:
		// Fresh containers may contain existing element references. This proves
		// container identity only; value-dependence escape remains conservative.
		switch e.Op {
		case "array.concat", "array.slice0", "array.slice1", "array.slice2", "array.splice1", "array.splice2", "array.splice3", "string.split", "map.keys", "map.values", "set.values", "set.copy", "set.fromArray":
			return nil, true
		}
	case hir.Local:
		if !x.assigned[e.Name] {
			b := env[e.Name]
			return b.classes, b.fresh
		}
	case hir.Narrow, hir.Cast:
		return x.proof(e.X, env)
	case hir.Conditional:
		a, af := x.proof(e.Y, env)
		b, bf := x.proof(e.Z, env)
		if len(a) > 0 && len(b) > 0 {
			return append(append([]string{}, a...), b...), af && bf
		}
	}
	return nil, false
}
func (x *extractor) expr(e *hir.Expr, path string, env scope) string {
	if e == nil {
		return ""
	}
	if e.Kind == hir.Local {
		return env[e.Name].id
	}
	if e.Kind == hir.This {
		return env["this"].id
	}
	x.add("expr", x.method, path, string(e.Kind), e.Type.String())
	if e.Kind == hir.Seq {
		env = copyScope(env)
		for i, s := range e.Stmt.List {
			x.stmt(s, fmt.Sprintf("%s/seq/s%d", path, i), env)
		}
	}
	children := []*hir.Expr{e.X, e.Y, e.Z}
	ids := make([]string, 3)
	for i, c := range children {
		ids[i] = x.expr(c, fmt.Sprintf("%s/%d", path, i), env)
		if ids[i] != "" {
			x.add("flow", x.method, path, ids[i])
		}
	}
	for i, a := range e.Args {
		id := x.expr(a, fmt.Sprintf("%s/arg%d", path, i), env)
		x.add("argument", path, strconv.Itoa(i), id)
		x.add("flow", x.method, path, id)
	}
	if _, fresh := x.proof(e, env); fresh {
		x.add("fresh", x.method, path)
	}
	owner := e.Owner
	if e.X != nil {
		owner = e.X.Type.Name
	}
	switch e.Kind {
	case hir.FieldGet:
		owner = x.fieldOwner(owner, e.Name, false)
		x.add("reads_field", x.method, owner, e.Name)
		x.add("field_origin", path, owner, e.Name)
	case hir.StaticGet:
		owner = x.fieldOwner(e.Owner, e.Name, true)
		x.add("reads_static", x.method, owner, e.Name)
		x.add("static_origin", path, owner, e.Name)
		if !x.constants[FieldKey{e.Owner, e.Name}] {
			x.init(e.Owner, path)
		}
	case hir.VirtualCall:
		x.add("virtual_call", x.method, ids[0], e.Name, path)
		x.add("site_type", path, owner)
		if cs, _ := x.proof(e.X, env); len(cs) > 0 {
			x.add("narrowed", path)
			for _, c := range cs {
				x.add("exact_receiver", path, c)
			}
		}
		x.add("argument", path, "this", ids[0])
	case hir.DirectCall, hir.SuperCall:
		if e.Kind == hir.SuperCall {
			owner = x.classes[x.owner].Super
		}
		if target := x.resolve(owner, e.Name); target != "" {
			x.add("calls", x.method, target, path)
		} else {
			x.add("unknown_effect", x.method)
			x.add("unknown_summary", x.method)
			for i := range e.Args {
				x.add("sink", x.method, fmt.Sprintf("%s/arg%d", path, i))
			}
			if ids[0] != "" {
				x.add("sink", x.method, ids[0])
			}
			x.add("raises", x.method, path)
		}
		if e.X != nil {
			x.add("argument", path, "this", ids[0])
		} else if e.Kind == hir.SuperCall {
			x.add("argument", path, "this", env["this"].id)
		}
	case hir.New:
		if e.Type.Kind == hir.ClassRef {
			x.add("new", path, e.Type.Name)
			x.init(e.Type.Name, path)
			if target := x.resolve(e.Type.Name, "constructor"); target != "" {
				x.add("calls", x.method, target, path)
			}
		}
	case hir.RuntimeOp:
		x.add("runtime_op", path, e.Op)
		x.add("site_method", path, x.method)
		effect, known := RuntimeEffects[e.Op]
		if effect.Writes == "Receiver" {
			x.add("mutation", x.method, ids[0])
			for i := range e.Args {
				x.add("sink", x.method, fmt.Sprintf("%s/arg%d", path, i))
			}
		} else if strings.HasPrefix(effect.Writes, "Arg(") {
			i, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(effect.Writes, "Arg("), ")"))
			if err == nil && i < len(e.Args) {
				x.add("mutation", x.method, fmt.Sprintf("%s/arg%d", path, i))
			}
		}
		if effect.Aliases == "Receiver" && referenceType(e.Type) {
			x.add("alias", x.method, path, ids[0])
		}
		// Allocation may be only an Optional wrapper around an existing ref.
		// op_allocates alone does not prove ownership or confinement.
		if !known || e.Op == "classvalue.new" || e.Op == "dynamic.materialize" {
			x.add("unknown_summary", x.method)
		}
		if !known || effect.Reads.Global || e.Op == "classvalue.new" || e.Op == "dynamic.materialize" {
			x.add("unknown_effect", x.method)
			for _, id := range ids {
				if id != "" {
					x.add("sink", x.method, id)
				}
			}
			for i := range e.Args {
				x.add("sink", x.method, fmt.Sprintf("%s/arg%d", path, i))
			}
		}
		if !known {
			x.add("raises", x.method, path)
		}

	case hir.Conditional:
		if referenceType(e.Type) {
			for _, id := range ids[1:] {
				if id != "" {
					x.add("alias", x.method, path, id)
				}
			}
		}
	case hir.Seq:
		if referenceType(e.Type) && ids[1] != "" {
			x.add("alias", x.method, path, ids[1])
		}
	case hir.Cast, hir.CheckedNumericConvert:
		if e.Kind == hir.Cast {
			x.add("alias", x.method, path, ids[0])
		}
		x.add("raises", x.method, path)
	case hir.Narrow:
		if e.Type.IsRef() {
			x.add("alias", x.method, path, ids[0])
		}
		if e.Type.IsRef() && e.X != nil && !e.Type.Equal(e.X.Type) {
			x.add("raises", x.method, path)
		}
	case hir.Unary:
		if e.CheckIntegerOverflow {
			x.add("raises", x.method, path)
		}
	case hir.Binary:
		if e.Type.Kind == hir.Number && (e.Op == "+" || e.Op == "-" || e.Op == "*" || e.Op == "/") || e.Op == "/" || e.Op == "%" || e.CheckIntegerOverflow {
			x.add("raises", x.method, path)
		}
	}
	return path
}

func (x *extractor) stmt(s *hir.Stmt, path string, env scope) {
	if s == nil {
		return
	}
	if s.Kind == hir.Block {
		env = copyScope(env)
		for i, b := range s.List {
			x.stmt(b, fmt.Sprintf("%s/s%d", path, i), env)
		}
		return
	}
	if s.Kind == hir.VarDecl {
		id := x.expr(s.X, path+"/x", env)
		cs, fresh := x.proof(s.X, env)
		b := binding{path + "/local/" + s.Name, cs, fresh && !x.assigned[s.Name]}
		env[s.Name] = b
		x.add("local", x.method, b.id, s.Type.String())
		if id != "" {
			x.add("assign", x.method, b.id, id)
			x.add("flow", x.method, b.id, id)
			if referenceType(s.Type) {
				x.add("alias", x.method, b.id, id)
				if !x.assigned[s.Name] && s.X != nil && (s.X.Kind == hir.This || s.X.Kind == hir.Local && !x.assigned[s.X.Name]) {
					x.add("must_alias", x.method, b.id, id)
				}
			}
		}
		if b.fresh {
			x.add("fresh", x.method, b.id)
		}
		return
	}
	a := ""
	if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.StaticGet {
		// The emitter guards the write; its LHS is not a static read.
		a = path + "/x"
		x.add("expr", x.method, a, string(s.X.Kind), s.X.Type.String())
	} else {
		a = x.expr(s.X, path+"/x", env)
	}
	b := x.expr(s.Y, path+"/y", env)
	switch s.Kind {
	case hir.Assign:
		switch s.X.Kind {
		case hir.Local:
			x.add("assign", x.method, a, b)
			x.add("flow", x.method, a, b)
			if referenceType(s.X.Type) {
				x.add("alias", x.method, a, b)
			}
		case hir.StaticGet:
			owner := x.fieldOwner(s.X.Owner, s.X.Name, true)
			if !x.constants[FieldKey{s.X.Owner, s.X.Name}] || x.method != methodID(s.X.Owner, "class_constructor") {
				x.init(s.X.Owner, path)
			}
			x.add("writes_static", x.method, owner, s.X.Name)
			x.add("sink", x.method, b)
			pred := "noncounter_write"
			if s.Y != nil && s.Y.Kind == hir.Binary && s.Y.Op == "+" && same(s.X, s.Y.X) && s.Y.Y != nil && s.Y.Y.Kind == hir.Lit && s.X.Type.Kind != hir.String {
				pred = "counter_shape"
			}
			x.add(pred, x.method, owner, s.X.Name)
		case hir.FieldGet:
			x.add("writes_field", x.method, x.fieldOwner(s.X.X.Type.Name, s.X.Name, false), s.X.Name)
			x.add("sink", x.method, b)
		case hir.IndexGet:
			recv := x.expr(s.X.X, path+"/index_receiver", env)
			x.add("mutation", x.method, recv)
			x.add("sink", x.method, b)
		}
	case hir.Return:
		if a != "" {
			x.add("returns", x.method, a)
			x.add("sink", x.method, a)
		}
	case hir.Throw:
		x.add("throws", x.method, path)
		x.add("sink", x.method, a)
	case hir.Trap:
		x.add("trap", x.method)
	case hir.Try:
		x.add("try", x.method, path)
	}
	bodyEnv := copyScope(env)
	if s.Kind == hir.ForEach {
		id := path + "/local/" + s.Name
		bodyEnv[s.Name] = binding{id: id}
		x.add("local", x.method, id, s.Type.String())
		if s.Kind == hir.ForEach {
			x.add("flow", x.method, id, a)
		}
	}
	x.stmt(s.Body, path+"/body", bodyEnv)
	catchEnv := copyScope(env)
	if s.Kind == hir.Try {
		id := path + "/catch/" + s.Name
		catchEnv[s.Name] = binding{id: id}
		x.add("local", x.method, id, s.Type.String())
	}
	x.stmt(s.Else, path+"/else", catchEnv)
}

// Interface-to-interface assignment is structural in hir.Verify.
func interfaceAccepts(src, dst *hir.Interface) bool {
	for _, want := range dst.Methods {
		found := false
		for _, got := range src.Methods {
			if got.Name != want.Name || !got.Result.Equal(want.Result) || len(got.Params) != len(want.Params) {
				continue
			}
			match := true
			for i, p := range want.Params {
				if p.Name != got.Params[i].Name || !p.Type.Equal(got.Params[i].Type) {
					match = false
				}
			}
			if match {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func referenceType(t hir.Type) bool {
	if t.Kind == hir.Optional && len(t.Args) == 1 {
		return referenceType(t.Args[0])
	}
	return t.IsRef()
}
