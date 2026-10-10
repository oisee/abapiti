package rewrite

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// ValueObjectRules is separate from the optimization rules: screening must not
// influence any emitter, inliner or existing optimization decision.
func ValueObjectRules() string {
	b, err := ruleFiles.ReadFile("rules/value_objects.grace")
	if err != nil {
		panic(err)
	}
	return string(b)
}

// ExtractValueObjects screens the whole verified, lowered closed world using
// Grace hierarchy, binding identities, alias, CFG and method write facts.
// Type sets are conservative: a use through a supertype screens every possible
// class. Dynamic uses are explicitly labelled uncertainty, never evidence of a
// definite comparison of the candidate. No readonly annotation is a proof.
func ExtractValueObjects(p *hir.Program, flow *DB) *DB {
	v := &valueScreen{db: NewDB(), flow: flow, classes: map[string]*hir.Class{}, types: map[string][]string{}, refs: map[string]string{}, own: map[string]bool{}, witnesses: map[string]Tuple{}, optionals: map[string]Tuple{}}
	for _, c := range p.Classes {
		v.classes[c.Name] = c
		v.add("vo_class", c.Name, c.Source)
	}
	for _, n := range []string{"1", "2", "3", "4", "5"} {
		v.add("vo_condition", n)
	}
	for _, r := range flow.Facts("subtype") {
		v.types[r[1]] = append(v.types[r[1]], r[0])
	}
	for _, r := range flow.Facts("local_ref") {
		v.refs[r[0]] = r[1]
	}
	for _, r := range flow.Facts("own_value") {
		v.own[r[1]] = true
	}
	for _, c := range p.Classes {
		if c.Abstract {
			v.block(c.Name, "3", c.Name, c.Source, "abstract class")
		}
		for _, child := range p.Classes {
			if child.Super == c.Name {
				v.block(c.Name, "3", child.Name, child.Source, "subclass "+child.Name)
			}
		}
		bytes, refs := 0, 0
		for cl := c; cl != nil; cl = v.classes[cl.Super] {
			for _, f := range cl.Fields {
				if f.Static {
					continue
				}
				size, ref := valueFieldSize(f.Type)
				bytes += size
				refs += ref
				v.add("vo_field", c.Name, cl.Name+"."+f.Name, f.Type.String(), strconv.Itoa(size), strconv.Itoa(ref), f.Source)
				v.optional(f.Type, f.Source, cl.Name+"."+f.Name)
				v.polymorphic(f.Type, f.Source, cl.Name+"."+f.Name)
				if f.Type.Kind == hir.OrderedMap && len(f.Type.Args) > 0 {
					v.collectionIdentity(f.Type.Args[0], cl.Name+"."+f.Name, f.Source, "map key type")
				}
				if f.Type.Kind == hir.OrderedSet {
					v.collectionIdentity(f.Type, cl.Name+"."+f.Name, f.Source, "set element type")
				}
			}
		}
		v.add("vo_size", c.Name, strconv.Itoa(bytes), strconv.Itoa(refs))
		// This is a declared copy-cost policy, not an ABAP layout prediction.
		if bytes > 64 || refs > 4 {
			v.block(c.Name, "5", c.Name, c.Source, "copy-cost policy: >64 inline bytes or >4 reference slots")
		}
		methods := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			methods = append(methods, c.Ctor)
		}
		for _, m := range methods {
			name := m.Name
			if m == c.Ctor {
				name = "constructor"
			}
			id := c.Name + "::" + name
			for _, par := range m.Params {
				v.optional(par.Type, m.Source, id+"/param/"+par.Name)
				v.polymorphic(par.Type, m.Source, id+"/param/"+par.Name)
			}
			v.optional(m.Result, m.Source, id+"/result")
			v.stmt(m.Body, id+"/body", 0, c, m)
		}
		if c.Ctor != nil {
			v.constructor(c)
		} else if c.Super == "" {
			for _, f := range c.Fields {
				if !f.Static && f.Type.Kind != hir.Optional {
					v.block(c.Name, "1", c.Name, f.Source, "unproven field initialization without constructor: "+f.Name)
				}
			}
		}
	}
	// Transitive/alias writes are evidence in their own right. All direct fields
	// are already witnessed above; retain summary uncertainty at the call site.
	nodes := v.nodes
	targets := map[string][]string{}
	for _, r := range flow.Facts("calls") {
		targets[r[2]] = append(targets[r[2]], r[1])
	}
	for path, n := range nodes {
		if !n.call {
			continue
		}
		unknown := len(targets[path]) == 0
		for _, callee := range targets[path] {
			if !flow.Has("defined", callee) {
				unknown = true
			}
		}
		if unknown {
			for _, arg := range n.operands {
				for _, cn := range v.possible(arg.e.Type) {
					v.block(cn, "1", path, n.source, "unproven field immutability at unresolved/missing-body call")
				}
			}
		}
	}
	for _, r := range flow.Facts("site_writes") {
		n, ok := nodes[r[0]]
		if !ok {
			continue
		}
		for _, arg := range n.operands {
			if v.refs[arg.path] != r[1] && arg.path != r[1] {
				continue
			}
			if n.ctor && v.own[r[1]] {
				continue
			} // checked by constructor/direct-write analysis
			for _, cn := range v.possible(arg.e.Type) {
				v.block(cn, "1", r[0], n.source, "potential transitive/alias write (Grace site_writes)")
			}
		}
	}

	keys := []string{}
	for k := range v.witnesses {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v.add("vo_violation", v.witnesses[k]...)
	}
	keys = nil
	for k := range v.optionals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v.add("vo_optional", v.optionals[k]...)
	}
	return v.db
}

type valueOperand struct {
	e    *hir.Expr
	path string
}
type valueNode struct {
	source   string
	ctor     bool
	operands []valueOperand
	call     bool
}
type valueScreen struct {
	db, flow  *DB
	classes   map[string]*hir.Class
	types     map[string][]string
	refs      map[string]string
	own       map[string]bool
	nodes     map[string]valueNode
	witnesses map[string]Tuple
	optionals map[string]Tuple
}

func (v *valueScreen) add(p string, a ...string) {
	if e := v.db.Add(p, a...); e != nil {
		panic(e)
	}
}
func (v *valueScreen) block(c, condition, path, source, reason string) {
	key := c + "\x00" + condition + "\x00" + reason
	r := Tuple{c, condition, path, source, reason}
	old := v.witnesses[key]
	if old == nil || valueSourceOrder(source) < valueSourceOrder(old[3]) || (source == old[3] && path < old[2]) {
		v.witnesses[key] = r
	}
}
func valueFieldSize(t hir.Type) (int, int) {
	switch t.Kind {
	case hir.Optional:
		n, r := valueFieldSize(t.Args[0])
		return n + 8, r
	case hir.Bool:
		return 1, 0
	case hir.I32:
		return 4, 0
	case hir.Number, hir.I64:
		return 8, 0
	case hir.String:
		return 16, 1 // descriptor/handle; backing UTF-16 is excluded
	default:
		return 8, 1 // handle, excluding referenced heap payload
	}
}
func (v *valueScreen) possible(t hir.Type) []string {
	if t.Kind == hir.Optional {
		return v.possible(t.Args[0])
	}
	if t.Kind == hir.Dynamic || (t.Kind == hir.ClassRef && t.Name == hir.RootObject) {
		if all, ok := v.types["$all"]; ok {
			return all
		}
		out := []string{}
		for c := range v.classes {
			out = append(out, c)
		}
		sort.Strings(out)
		v.types["$all"] = out
		return out
	}
	if t.Kind != hir.ClassRef && t.Kind != hir.InterfaceRef {
		return nil
	}
	return v.types[t.Name]
}
func (v *valueScreen) optional(t hir.Type, source, path string) {
	if t.Kind == hir.Optional {
		for _, c := range v.possible(t.Args[0]) {
			key := c + "\x00" + t.String()
			old := v.optionals[key]
			if old == nil || valueSourceOrder(source) < valueSourceOrder(old[2]) || (source == old[2] && path < old[1]) {
				v.optionals[key] = Tuple{c, path, source, t.String()}
			}
		}
	}
	// Optional<Array<C>> makes the container absent, not each C element. Only
	// an Optional directly around a compatible object reference flags that row.
	for _, a := range t.Args {
		v.optional(a, source, path)
	}
}
func (v *valueScreen) polymorphic(t hir.Type, source, path string) {
	choices := v.possible(t)
	for _, cn := range choices {
		if len(choices) > 1 || t.Kind == hir.Dynamic {
			v.block(cn, "3", path, source, "potential mixed polymorphic storage/use: "+t.String())
		}
	}
	for _, a := range t.Args {
		v.polymorphic(a, source, path)
	}
}
func (v *valueScreen) store(t hir.Type, e *hir.Expr, path, source string) {
	if e == nil {
		return
	}
	target := t
	if target.Kind == hir.Optional {
		target = target.Args[0]
	}
	if target.Kind == hir.Dynamic || target.Kind == hir.InterfaceRef || target.Kind == hir.ClassRef {
		choices := v.possible(target)
		if len(choices) > 1 {
			accepted := map[string]bool{}
			for _, cn := range choices {
				accepted[cn] = true
			}
			for _, cn := range v.possible(e.Type) {
				if accepted[cn] {
					reason := "potential assigned to mixed supertype/interface " + target.String()
					from := e.Type
					if from.Kind == hir.Optional {
						from = from.Args[0]
					}
					if from.Kind != hir.ClassRef || from.Name != cn {
						if !strings.HasPrefix(reason, "potential ") {
							reason = "potential " + reason
						}
					}
					v.block(cn, "3", path, source, reason)
				}
			}
		}
	}
}
func (v *valueScreen) stmt(s *hir.Stmt, path string, loop int, c *hir.Class, m *hir.Method) {
	if s == nil {
		return
	}
	source := s.Source
	if source == "" {
		if s.X != nil {
			source = s.X.Source
		}
		if source == "" {
			source = m.Source
		}
	}
	v.optional(s.Type, source, path)
	if s.Kind == hir.Assign && s.X != nil {
		v.store(s.X.Type, s.Y, path, source)
		if s.X.Kind == hir.StaticGet && !(m == c.Ctor && s.X.Owner == c.Name) && !(m.Name == "class_constructor" && s.X.Owner == c.Name) {
			if v.classes[s.X.Owner] != nil {
				v.block(s.X.Owner, "1", path, source, "static field write outside constructor/initializer: "+s.X.Owner+"."+s.X.Name)
			}
		}

		if s.X.Kind == hir.FieldGet && s.X.X != nil {
			for _, cn := range v.possible(s.X.X.Type) {
				isOwn := s.X.X.Kind == hir.This || v.own[v.refs[path+"/x/0"]]
				// Only writes to this inside the candidate's own constructor are exempt.
				if m == c.Ctor && isOwn {
					continue
				}
				reason := "field write outside own constructor: " + s.X.Owner + "." + s.X.Name
				if untypedValueType(s.X.X.Type) {
					reason = "potential " + reason
				}
				v.block(cn, "1", path, source, reason)
			}
		}
	}
	if s.Kind == hir.VarDecl {
		v.store(s.Type, s.X, path, source)
		v.polymorphic(s.Type, source, path)
	}
	if s.Kind == hir.Return {
		v.store(m.Result, s.X, path, source)
	}
	v.expr(s.X, path+"/x", loop, c, m)
	v.expr(s.Y, path+"/y", loop, c, m)
	for i, b := range s.List {
		v.stmt(b, fmt.Sprintf("%s/s%d", path, i), loop, c, m)
	}
	d := loop
	if s.Kind == hir.ForEach || s.Kind == hir.While {
		d++
	}
	v.stmt(s.Body, path+"/body", d, c, m)
	v.stmt(s.Else, path+"/else", loop, c, m)
}
func (v *valueScreen) expr(e *hir.Expr, path string, loop int, c *hir.Class, m *hir.Method) {
	if e == nil {
		return
	}
	source := e.Source
	if source == "" {
		source = m.Source
	}
	if source == "" {
		source = c.Source
	}
	v.optional(e.Type, source, path)
	operands := []valueOperand{}
	for i, x := range []*hir.Expr{e.X, e.Y, e.Z} {
		if x != nil {
			operands = append(operands, valueOperand{x, fmt.Sprintf("%s/%d", path, i)})
		}
	}
	for i, x := range e.Args {
		operands = append(operands, valueOperand{x, fmt.Sprintf("%s/arg%d", path, i)})
	}
	if v.nodes == nil {
		v.nodes = map[string]valueNode{}
	}
	v.nodes[path] = valueNode{source, m == c.Ctor, operands, e.Kind == hir.DirectCall || e.Kind == hir.VirtualCall || e.Kind == hir.SuperCall || (e.Kind == hir.New && e.Type.Kind == hir.ClassRef)}
	if e.Kind == hir.New && e.Type.Kind == hir.ClassRef {
		v.add("vo_new", e.Type.Name, strings.Split(path, "/body")[0], path, source, strconv.Itoa(loop))
	}
	if e.Kind == hir.Binary && (e.Op == "==" || e.Op == "!=" || e.Op == "===" || e.Op == "!==") && e.X != nil && e.Y != nil {
		a, b := v.possible(e.X.Type), v.possible(e.Y.Type)
		// A primitive or absent check does not observe object identity.
		absent := func(x *hir.Expr) bool { return x.Kind == hir.Lit && x.Value == nil }
		if len(a) > 0 && len(b) > 0 && !absent(e.X) && !absent(e.Y) {
			for _, cn := range append(a, b...) {
				reason := "reference equality"
				if untypedValueType(e.X.Type) || untypedValueType(e.Y.Type) {
					reason = "potential reference equality through dynamic type"
				}
				v.block(cn, "2", path, source, reason)
			}
		}
	}
	if e.Kind == hir.InstanceOf {
		if v.classes[e.Owner] != nil {
			v.block(e.Owner, "2", path, source, "instanceof/class test: "+e.Owner)
		}
		if e.X != nil {
			for _, cn := range v.possible(e.X.Type) {
				reason := "instanceof/class test through " + e.X.Type.String()
				if untypedValueType(e.X.Type) {
					reason = "potential " + reason
				}
				v.block(cn, "2", path, source, reason)
			}
		}
	}
	if (e.Kind == hir.Cast || e.Kind == hir.Narrow) && e.X != nil {
		target, fromType := e.Type, e.X.Type
		if target.Kind == hir.Optional {
			target = target.Args[0]
		}
		if fromType.Kind == hir.Optional {
			fromType = fromType.Args[0]
		}
		if (target.Kind == hir.ClassRef || target.Kind == hir.InterfaceRef) && !target.Equal(fromType) {
			accepted := map[string]bool{}
			for _, cn := range v.possible(target) {
				accepted[cn] = true
			}
			checked := false
			for _, cn := range v.possible(fromType) {
				if !accepted[cn] {
					checked = true
				}
			}
			if checked {
				for _, cn := range append(v.possible(fromType), v.possible(target)...) {
					reason := "checked reference downcast/class test: " + fromType.String() + " -> " + target.String()
					if fromType.Kind == hir.Dynamic || fromType.Name == hir.RootObject {
						if !strings.HasPrefix(reason, "potential ") {
							reason = "potential " + reason
						}
					}
					v.block(cn, "2", path, source, reason)
				}
			}
		}
	}
	if e.Kind == hir.RuntimeOp {
		_, known := RuntimeEffects[e.Op]
		if !known || e.Op == "dynamic.materialize" || e.Op == "classvalue.new" {
			for _, arg := range operands {
				for _, cn := range v.possible(arg.e.Type) {
					v.block(cn, "1", path, source, "unproven field immutability at dynamic/unknown runtime operation: "+e.Op)
				}
			}
		}
		if e.Op == "classvalue.new" {
			for _, cn := range v.possible(e.Type) {
				v.add("vo_dynamic_new", cn, c.Name+"::"+m.Name, path, source, strconv.Itoa(loop))
			}
		}
		if e.Op == "dynamic.strictEquals" {
			for _, arg := range operands {
				for _, cn := range v.possible(arg.e.Type) {
					v.block(cn, "2", path, source, "potential reference equality through dynamic.strictEquals")
				}
			}
		}
		if (e.Op == "array.includes" || e.Op == "array.indexOf") && len(e.Args) > 0 {
			for _, cn := range v.possible(e.Args[0].Type) {
				reason := "reference membership comparison: " + e.Op
				if untypedValueType(e.Args[0].Type) {
					reason = "potential " + reason
				}
				v.block(cn, "2", path, source, reason)
			}
		}
		key := []*hir.Expr{}
		switch {
		case strings.HasPrefix(e.Op, "map.") && len(e.Args) > 0 && (e.Op == "map.set" || e.Op == "map.get" || e.Op == "map.has" || e.Op == "map.delete"):
			key = append(key, e.Args[0])
		case strings.HasPrefix(e.Op, "set.") && len(e.Args) > 0:
			key = append(key, e.Args[0])
		}
		for _, k := range key {
			for _, cn := range v.possible(k.Type) {
				reason := "identity-keyed collection: " + e.Op
				if untypedValueType(k.Type) {
					reason = "potential " + reason
				}
				v.block(cn, "2", path, source, reason)
			}
		}
		// Constructor conversions such as set.fromArray preserve element identity.
		if e.Op == "set.fromArray" || strings.HasPrefix(e.Op, "weak") {
			for _, op := range operands {
				v.collectionIdentity(op.e.Type, path, source, e.Op)
			}
		}
		if strings.HasPrefix(e.Op, "classvalue.") || e.Op == "object.classOf" {
			for _, op := range operands {
				for _, cn := range v.possible(op.e.Type) {
					reason := "dynamic class operation: " + e.Op
					from := op.e.Type
					if from.Kind == hir.Optional {
						from = from.Args[0]
					}
					if from.Kind != hir.ClassRef || from.Name != cn {
						if !strings.HasPrefix(reason, "potential ") {
							reason = "potential " + reason
						}
					}
					v.block(cn, "2", path, source, reason)
				}
			}
		}
	}
	// Polymorphic fields/arrays/collections can be populated through aliases;
	// screen their declared element types rather than trusting a local assignment.
	if e.Kind == hir.FieldGet || e.Kind == hir.IndexGet {
		v.polymorphic(e.Type, source, path)
	}
	for _, op := range operands {
		v.expr(op.e, op.path, loop, c, m)
	}
	if e.Kind == hir.Seq && e.Stmt != nil {
		for i, s := range e.Stmt.List {
			v.stmt(s, fmt.Sprintf("%s/seq/s%d", path, i), loop, c, m)
		}
	}
}
func (v *valueScreen) collectionIdentity(t hir.Type, path, source, reason string) {
	for _, cn := range v.possible(t) {
		r := "identity-keyed collection: " + reason
		if untypedValueType(t) {
			r = "potential " + r
		}
		v.block(cn, "2", path, source, r)
	}
	for _, a := range t.Args {
		v.collectionIdentity(a, path, source, reason)
	}
}

// Constructor proof accepts a straight-line initialization prefix. Calls using
// this, aliases, stores and returns before completion are conservatively blocked.
// Branch/loop-dependent initialization and helper initialization are unproven.
func (v *valueScreen) constructor(c *hir.Class) {
	pending := map[string]bool{}
	for _, f := range c.Fields {
		if !f.Static && f.Type.Kind != hir.Optional {
			pending[f.Name] = true
		}
	}
	var inspect func(*hir.Expr, string, bool)
	inspect = func(e *hir.Expr, path string, receiverOnly bool) {
		if e == nil {
			return
		}
		own := e.Kind == hir.This || v.own[v.refs[path]]
		if own && !receiverOnly && len(pending) > 0 {
			v.block(c.Name, "1", path, e.Source, "this may leak/be observed before definite field initialization")
		}
		if e.Kind == hir.Seq {
			v.block(c.Name, "1", path, e.Source, "unproven constructor sequence initialization")
		}
		inspect(e.X, path+"/0", e.Kind == hir.FieldGet)
		inspect(e.Y, path+"/1", false)
		inspect(e.Z, path+"/2", false)
		for i, a := range e.Args {
			inspect(a, fmt.Sprintf("%s/arg%d", path, i), false)
		}
	}
	var scan func(*hir.Stmt, string, bool)
	scan = func(s *hir.Stmt, path string, definite bool) {
		if s == nil {
			return
		}
		if s.Kind == hir.Block {
			for i, b := range s.List {
				scan(b, fmt.Sprintf("%s/s%d", path, i), definite)
			}
			return
		}
		if len(pending) > 0 && s.Kind == hir.Assign && s.X != nil && s.Y != nil && (s.Y.Kind == hir.This || v.own[v.refs[path+"/y"]]) {
			if s.X.Kind == hir.StaticGet || s.X.Kind == hir.IndexGet || (s.X.Kind == hir.FieldGet && s.X.X != nil && s.X.X.Kind != hir.This && !v.own[v.refs[path+"/x/0"]]) {
				v.block(c.Name, "1", path, s.Source, "this leaked before definite field initialization")
			}
		}
		inspect(s.X, path+"/x", s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.FieldGet)
		inspect(s.Y, path+"/y", false)
		if definite && s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.FieldGet && s.X.X != nil && s.X.X.Kind == hir.This {
			delete(pending, s.X.Name)
		}
		scan(s.Body, path+"/body", false)
		scan(s.Else, path+"/else", false)
	}
	scan(c.Ctor.Body, c.Name+"::constructor/body", true)
	for f := range pending {
		v.block(c.Name, "1", c.Name+"::constructor", c.Ctor.Source, "unproven definite constructor initialization: "+f)
	}
}

// Retain the first numeric source location per blocker category, independently
// of declaration/traversal order. All allocation sites remain separate facts.
func valueSourceOrder(s string) string {
	parts := strings.Split(s, ":")
	if len(parts) >= 3 {
		line, _ := strconv.Atoi(parts[len(parts)-2])
		col, _ := strconv.Atoi(parts[len(parts)-1])
		return fmt.Sprintf("%s:%09d:%09d", strings.Join(parts[:len(parts)-2], ":"), line, col)
	}
	return s
}

func untypedValueType(t hir.Type) bool {
	if t.Kind == hir.Optional {
		return untypedValueType(t.Args[0])
	}
	return t.Kind == hir.Dynamic || (t.Kind == hir.ClassRef && t.Name == hir.RootObject)
}
