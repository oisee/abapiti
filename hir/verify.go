package hir

import (
	"fmt"
	"math"
	"reflect"
)

type Error struct {
	Node
	Message string
}

func (e Error) Error() string { return fmt.Sprintf("node %d (%s): %s", e.ID, e.Source, e.Message) }

type verifier struct {
	p          *Program
	errors     []error
	classes    map[string]*Class
	interfaces map[string]*Interface
}

func Verify(p *Program) []error {
	v := &verifier{p: p, classes: map[string]*Class{}, interfaces: map[string]*Interface{}}
	if p == nil {
		v.fail(Node{}, "nil program")
		return v.errors
	}
	for _, c := range p.Classes {
		if c == nil {
			v.fail(Node{}, "nil class")
			continue
		}
		if _, ok := v.classes[c.Name]; ok || c.Name == "" {
			v.fail(c.Node, "duplicate or empty class "+c.Name)
		}
		v.classes[c.Name] = c
	}
	for _, i := range p.Interfaces {
		if i == nil {
			v.fail(Node{}, "nil interface")
			continue
		}
		if _, ok := v.interfaces[i.Name]; ok || i.Name == "" || v.classes[i.Name] != nil {
			v.fail(i.Node, "duplicate or empty interface "+i.Name)
		}
		v.interfaces[i.Name] = i
	}
	for _, c := range p.Classes {
		if c == nil {
			continue
		}
		seen := map[string]bool{}
		for n := c.Name; n != ""; {
			if seen[n] {
				v.fail(c.Node, "inheritance cycle")
				break
			}
			seen[n] = true
			b := v.classes[n]
			if b == nil {
				v.fail(c.Node, "unresolved base "+n)
				break
			}
			n = b.Super
		}
	}
	if len(v.errors) > 0 {
		return v.errors
	}
	// Reject nil declarations before any signature or member lookup uses them.
	for _, i := range p.Interfaces {
		for _, m := range i.Methods {
			if m == nil {
				v.fail(i.Node, "nil interface method")
			}
		}
	}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if m == nil {
				v.fail(c.Node, "nil class method")
			}
		}
	}
	if len(v.errors) > 0 {
		return v.errors
	}
	for _, i := range p.Interfaces {
		names := map[string]bool{}
		for _, m := range i.Methods {
			v.signature(m)
			if names[m.Name] || m.Name == "" || m.Static {
				v.fail(m.Node, "duplicate or static interface method")
			}
			names[m.Name] = true
		}
	}
	for _, c := range p.Classes {
		names := map[string]bool{}
		for _, f := range c.Fields {
			v.typ(f.Node, f.Type)
			if names[f.Name] || f.Name == "" || v.field(c.Super, f.Name) != nil || v.method(c.Super, f.Name) != nil {
				v.fail(f.Node, "duplicate or empty member")
			}
			names[f.Name] = true
		}
		for _, m := range c.Methods {
			v.signature(m)
			if m.Name == classConstructor {
				// The implicit static class constructor: no parameters, no result,
				// never virtual or abstract, exactly one per class, and it may not
				// be called explicitly.
				if !m.Static || m.Virtual || m.Abstract || len(m.Params) != 0 || m.Result.Kind != Void {
					v.fail(m.Node, "invalid class constructor")
				}
				if names[m.Name] {
					v.fail(m.Node, "duplicate class constructor")
				}
			}
			if names[m.Name] || m.Name == "" || v.field(c.Super, m.Name) != nil {
				v.fail(m.Node, "duplicate member "+m.Name)
			}
			names[m.Name] = true
			if m.Abstract && (!c.Abstract || !m.Virtual || m.Static) {
				v.fail(m.Node, "invalid abstract method")
			}
			if base := v.method(c.Super, m.Name); base != nil && m.Name != classConstructor {
				if !base.Virtual || m.Static || !sameSignature(base, m) {
					v.fail(m.Node, "invalid override "+m.Name)
				}
			}
			v.body(c, m)
		}
		if c.Ctor != nil {
			v.signature(c.Ctor)
			if c.Ctor.Static || c.Ctor.Abstract || c.Ctor.Result.Kind != Void {
				v.fail(c.Ctor.Node, "invalid constructor")
			}
			v.body(c, c.Ctor)
		}
		if !c.Abstract {
			for b := c; b != nil; b = v.classes[b.Super] {
				for _, m := range b.Methods {
					if m.Abstract {
						impl := v.method(c.Name, m.Name)
						if impl == nil || impl.Abstract {
							v.fail(c.Node, "missing abstract implementation "+m.Name)
						}
					}
				}
			}
		}
		for _, name := range c.Implements {
			i := v.interfaces[name]
			if i == nil {
				v.fail(c.Node, "unresolved interface "+name)
				continue
			}
			for _, m := range i.Methods {
				impl := v.method(c.Name, m.Name)
				if impl == nil || impl.Static || !sameSignature(m, impl) || (!c.Abstract && impl.Abstract) {
					v.fail(c.Node, "missing interface implementation "+m.Name)
				}
			}
		}
	}
	return v.errors
}
func (v *verifier) fail(n Node, s string) { v.errors = append(v.errors, Error{n, s}) }

// classConstructor names the implicit static class constructor method. It is
// emitted as ABAP's CLASS-METHODS class_constructor and runs implicitly.
const classConstructor = "class_constructor"

func (v *verifier) typ(n Node, t Type) {
	arity := 0
	switch t.Kind {
	case Bool, Number, I32, I64, String, Void:
	case ClassRef, ClassValue:
		if v.classes[t.Name] == nil {
			v.fail(n, "unresolved class "+t.Name)
		}
	case InterfaceRef:
		if v.interfaces[t.Name] == nil {
			v.fail(n, "unresolved interface "+t.Name)
		}
	case Optional, Array, OrderedSet:
		arity = 1
	case OrderedMap:
		arity = 2
	default:
		v.fail(n, "unknown type "+t.String())
	}
	if len(t.Args) != arity {
		v.fail(n, "invalid type arguments "+t.String())
		return
	}
	for _, a := range t.Args {
		v.typ(n, a)
		if a.Kind == Void {
			v.fail(n, "void type argument")
		}
	}
	if t.Kind == Optional && t.Args[0].Kind == Optional {
		v.fail(n, "nested optional")
	}
	if t.Kind == OrderedMap || t.Kind == OrderedSet {
		k := t.Args[0]
		if k.Kind != String && k.Kind != I32 && k.Kind != I64 && k.Kind != ClassRef && k.Kind != InterfaceRef {
			v.fail(n, "unsupported collection key "+k.String())
		}
	}
}
func (v *verifier) signature(m *Method) {
	v.typ(m.Node, m.Result)
	seen := map[string]bool{}
	for _, p := range m.Params {
		v.typ(m.Node, p.Type)
		if seen[p.Name] || p.Name == "" || p.Type.Kind == Void {
			v.fail(m.Node, "invalid parameter "+p.Name)
		}
		seen[p.Name] = true
	}
	if m.Static && m.Virtual {
		v.fail(m.Node, "static virtual method")
	}
}
func sameSignature(a, b *Method) bool {
	if !a.Result.Equal(b.Result) || len(a.Params) != len(b.Params) {
		return false
	}
	for i, p := range a.Params {
		if p.Name != b.Params[i].Name || !p.Type.Equal(b.Params[i].Type) {
			return false
		}
	}
	return true
}
func (v *verifier) method(owner, name string) *Method {
	if i := v.interfaces[owner]; i != nil {
		for _, m := range i.Methods {
			if m.Name == name {
				return m
			}
		}
		return nil
	}
	for c := v.classes[owner]; c != nil; c = v.classes[c.Super] {
		for _, m := range c.Methods {
			if m.Name == name {
				return m
			}
		}
	}
	return nil
}
func (v *verifier) field(owner, name string) *Field {
	for c := v.classes[owner]; c != nil; c = v.classes[c.Super] {
		for i := range c.Fields {
			if c.Fields[i].Name == name {
				return &c.Fields[i]
			}
		}
	}
	return nil
}
func (v *verifier) accepts(dst, src Type) bool {
	if dst.Equal(src) {
		return true
	}
	if dst.Kind == Optional && len(dst.Args) == 1 {
		return v.accepts(dst.Args[0], src)
	}
	if src.Kind == ClassRef {
		for c := v.classes[src.Name]; c != nil; c = v.classes[c.Super] {
			if dst.Kind == ClassRef && c.Name == dst.Name {
				return true
			}
			for _, i := range c.Implements {
				if dst.Kind == InterfaceRef && dst.Name == i {
					return true
				}
			}
		}
	}
	return false
}
func (v *verifier) body(c *Class, m *Method) {
	if m.Abstract {
		if m.Body != nil {
			v.fail(m.Node, "abstract method has body")
		}
		return
	}
	if m.Body == nil {
		v.fail(m.Node, "missing method body")
		return
	}
	env := map[string]Type{}
	for _, p := range m.Params {
		env[p.Name] = p.Type
	}
	v.stmt(c, m, m.Body, env, 0)
	if m.Result.Kind != Void && !returns(m.Body) {
		v.fail(m.Node, "method may fall through without return")
	}
}
func returns(s *Stmt) bool {
	if s == nil {
		return false
	}
	switch s.Kind {
	case Return, Throw:
		return true
	case Block:
		for _, x := range s.List {
			if returns(x) {
				return true
			}
		}
	case If, Try:
		return returns(s.Body) && returns(s.Else)
	}
	return false
}
func clone(env map[string]Type) map[string]Type {
	r := map[string]Type{}
	for k, t := range env {
		r[k] = t
	}
	return r
}
func (v *verifier) stmt(c *Class, m *Method, s *Stmt, env map[string]Type, loops int) {
	if s == nil {
		return
	}
	check := func(e *Expr) Type { return v.expr(c, m, e, env) }
	assign := func(t Type, e *Expr) {
		if !v.accepts(t, check(e)) {
			v.fail(s.Node, "assignment type mismatch")
		}
	}
	switch s.Kind {
	case Block:
		scope := clone(env)
		for _, x := range s.List {
			v.stmt(c, m, x, scope, loops)
		}
	case VarDecl:
		v.typ(s.Node, s.Type)
		if _, ok := env[s.Name]; ok || s.Name == "" || s.Type.Kind == Void {
			v.fail(s.Node, "invalid local "+s.Name)
		}
		if s.X != nil {
			assign(s.Type, s.X)
		}
		env[s.Name] = s.Type
	case Assign:
		t := check(s.X)
		if s.X == nil || (s.X.Kind != Local && s.X.Kind != FieldGet && s.X.Kind != StaticGet && s.X.Kind != IndexGet) {
			v.fail(s.Node, "invalid assignment target")
		}
		assign(t, s.Y)
	case ExprStmt:
		check(s.X)
	case If, While:
		if check(s.X).Kind != Bool {
			v.fail(s.Node, "condition must be bool")
		}
		n := loops
		if s.Kind == While {
			n++
		}
		v.stmt(c, m, s.Body, clone(env), n)
		v.stmt(c, m, s.Else, clone(env), loops)
	case ForEach:
		t := check(s.X)
		if t.Kind != Array || len(t.Args) != 1 || !s.Type.Equal(t.Args[0]) {
			v.fail(s.Node, "foreach requires matching array")
		}
		e := clone(env)
		e[s.Name] = s.Type
		v.stmt(c, m, s.Body, e, loops+1)
	case Break, Continue:
		if loops == 0 {
			v.fail(s.Node, "loop control outside loop")
		}
	case Return:
		if s.X == nil {
			if m.Result.Kind != Void {
				v.fail(s.Node, "missing return value")
			}
		} else {
			assign(m.Result, s.X)
		}
	case Throw:
		if check(s.X).Kind == Void {
			v.fail(s.Node, "void throw")
		}
	case Try:
		v.typ(s.Node, s.Type)
		v.stmt(c, m, s.Body, clone(env), loops)
		e := clone(env)
		e[s.Name] = s.Type
		v.stmt(c, m, s.Else, e, loops)
	default:
		v.fail(s.Node, "unknown statement "+string(s.Kind))
	}
}
func (v *verifier) expr(c *Class, m *Method, e *Expr, env map[string]Type) Type {
	if e == nil {
		v.fail(m.Node, "missing expression")
		return T(Void)
	}
	v.typ(e.Node, e.Type)
	t := e.Type
	check := func(x *Expr) Type { return v.expr(c, m, x, env) }
	eq := func(a, b Type) {
		if !a.Equal(b) {
			v.fail(e.Node, "expression type mismatch: "+a.String()+" / "+b.String())
		}
	}
	args := func(params []Type) {
		if len(params) != len(e.Args) {
			v.fail(e.Node, "call arity mismatch")
		}
		for i, x := range e.Args {
			a := check(x)
			if i < len(params) && !v.accepts(params[i], a) {
				v.fail(e.Node, "call argument type mismatch")
			}
		}
	}
	switch e.Kind {
	case Lit:
		lt := t
		if t.Kind == Optional && len(t.Args) == 1 {
			lt = t.Args[0]
		}
		valid := false
		if e.Value == nil {
			valid = t.Kind == Optional
		} else {
			switch lt.Kind {
			case Bool:
				_, valid = e.Value.(bool)
			case String:
				_, valid = e.Value.(string)
			case Number, I32, I64:
				r := reflect.ValueOf(e.Value)
				valid = r.Kind() >= reflect.Int && r.Kind() <= reflect.Float64
				if valid {
					var f float64
					if r.Kind() <= reflect.Int64 {
						f = float64(r.Int())
					} else if r.Kind() <= reflect.Uintptr {
						f = float64(r.Uint())
					} else {
						f = r.Float()
					}
					if lt.Kind != Number {
						valid = f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f)
						if lt.Kind == I32 {
							valid = valid && f >= -2147483648 && f <= 2147483647
						} else if r.Kind() >= reflect.Uint && r.Kind() <= reflect.Uintptr {
							valid = valid && r.Uint() <= math.MaxInt64
						} else if r.Kind() == reflect.Float32 || r.Kind() == reflect.Float64 {
							valid = valid && f >= -9223372036854775808.0 && f < 9223372036854775808.0
						}
					}
				}
			}
		}
		if !valid {
			v.fail(e.Node, "invalid literal")
		}
	case Local:
		a, ok := env[e.Name]
		if !ok {
			v.fail(e.Node, "unresolved local "+e.Name)
		} else {
			eq(t, a)
		}
	case This:
		if m.Static {
			v.fail(e.Node, "this in static method")
		}
		eq(t, Ref(c.Name))
	case FieldGet, StaticGet:
		owner := e.Owner
		if e.Kind == FieldGet {
			a := check(e.X)
			owner = a.Name
			if a.Kind != ClassRef {
				v.fail(e.Node, "field on non-class")
			}
		}
		f := v.field(owner, e.Name)
		if f == nil {
			v.fail(e.Node, "unresolved field "+e.Name)
		} else {
			eq(t, f.Type)
			if f.Static != (e.Kind == StaticGet) {
				v.fail(e.Node, "field static mismatch")
			}
		}
	case IndexGet:
		a := check(e.X)
		eq(check(e.Y), T(I32))
		if a.Kind != Array || len(a.Args) != 1 {
			v.fail(e.Node, "index on non-array")
		} else {
			eq(t, a.Args[0])
		}
	case DirectCall, VirtualCall, SuperCall:
		if e.Kind == VirtualCall && e.X == nil {
			v.fail(e.Node, "virtual call requires object receiver")
		}
		owner := e.Owner
		if e.Kind == SuperCall {
			owner = c.Super
			if m.Static {
				v.fail(e.Node, "super in static method")
			}
		} else if e.X != nil {
			a := check(e.X)
			owner = a.Name
			if a.Kind != ClassRef && a.Kind != InterfaceRef {
				v.fail(e.Node, "call on non-object")
			}
		}
		f := v.method(owner, e.Name)
		if e.Kind == SuperCall && e.Name == "constructor" {
			f = v.p.Constructor(owner)
			if m != c.Ctor {
				v.fail(e.Node, "constructor call outside constructor")
			}
		}
		if f != nil && f.Name == classConstructor && e.Kind != SuperCall {
			v.fail(e.Node, "class constructor is implicit")
			f = nil
		}
		if f == nil {
			v.fail(e.Node, "unresolved method "+e.Name)
		} else {
			eq(t, f.Result)
			ps := []Type{}
			for _, p := range f.Params {
				ps = append(ps, p.Type)
			}
			args(ps)
			if e.Kind == SuperCall && (f.Static || f.Abstract) {
				v.fail(e.Node, "invalid super call")
			}
			if e.Kind == VirtualCall && !f.Virtual && v.interfaces[owner] == nil {
				v.fail(e.Node, "virtual call on non-virtual method")
			}
			if e.Kind == DirectCall && ((e.X == nil) != f.Static || f.Virtual) {
				v.fail(e.Node, "invalid direct call")
			}
		}
	case New:
		if t.Kind == ClassRef {
			cl := v.classes[t.Name]
			if cl != nil {
				if cl.Abstract {
					v.fail(e.Node, "new abstract class")
				}
				ps := []Type{}
				if ctor := v.p.Constructor(cl.Name); ctor != nil {
					for _, p := range ctor.Params {
						ps = append(ps, p.Type)
					}
				}
				args(ps)
			}
		} else if t.Kind == Array || t.Kind == OrderedMap || t.Kind == OrderedSet {
			args(nil)
		} else {
			v.fail(e.Node, "unsupported allocation")
		}
	case Binary:
		a, b := check(e.X), check(e.Y)
		eq(a, b)
		switch e.Op {
		case "==", "!=":
			eq(t, T(Bool))
			if a.Kind == Void || b.Kind == Void {
				v.fail(e.Node, "void equality operand")
			}
		case "<", "<=", ">", ">=":
			eq(t, T(Bool))
			if !numeric(a) {
				v.fail(e.Node, "non-numeric comparison")
			}
		case "&&", "||":
			eq(a, T(Bool))
			eq(t, T(Bool))
		case "+", "-", "*", "/", "%":
			eq(t, a)
			if !numeric(a) {
				v.fail(e.Node, "non-numeric arithmetic")
			}
		default:
			v.fail(e.Node, "unknown binary operator")
		}
	case Unary:
		a := check(e.X)
		eq(a, t)
		if e.Op == "!" {
			eq(t, T(Bool))
		} else if e.Op != "-" || !numeric(t) {
			v.fail(e.Node, "invalid unary operator")
		}
	case Conditional:
		if t.Kind == Void {
			v.fail(e.Node, "void conditional")
		}
		eq(check(e.X), T(Bool))
		if !v.accepts(t, check(e.Y)) || !v.accepts(t, check(e.Z)) {
			v.fail(e.Node, "conditional type mismatch")
		}
	case InstanceOf:
		a := check(e.X)
		eq(t, T(Bool))
		if (a.Kind != ClassRef && a.Kind != InterfaceRef && a.Kind != Optional) || v.classes[e.Owner] == nil {
			v.fail(e.Node, "invalid instanceof")
		}
	case IsUndefined:
		a := check(e.X)
		eq(t, T(Bool))
		if a.Kind != Optional {
			v.fail(e.Node, "undefined test needs optional")
		}
	case ToBoolean:
		a := check(e.X)
		eq(t, T(Bool))
		if a.Kind == Void || a.Kind == ClassValue {
			v.fail(e.Node, "invalid boolean conversion")
		}
	case Narrow:
		// A checker-proven typed view: unwrapping an Optional or downcasting a
		// reference to a proven subtype. The emitter never invents the proof; a
		// failed runtime cast raises instead of returning a wrong reference.
		a := check(e.X)
		base := a
		if base.Kind == Optional {
			base = base.Args[0]
		}
		if (t.Kind != ClassRef && t.Kind != InterfaceRef) || !v.accepts(base, t) {
			v.fail(e.Node, "invalid narrowing "+a.String()+" to "+t.String())
		}
	case RuntimeOp:
		a := check(e.X)
		if len(v.errors) > 0 && len(a.Args) == 0 && a.Kind != String {
			break
		}
		ps, r, ok := RuntimeSignature(e.Op, a)
		if !ok {
			v.fail(e.Node, "unsupported runtime op "+e.Op)
		} else {
			eq(t, r)
			args(ps)
		}
	default:
		v.fail(e.Node, "unknown expression "+string(e.Kind))
	}
	return t
}
func numeric(t Type) bool { return t.Kind == Number || t.Kind == I32 || t.Kind == I64 }
