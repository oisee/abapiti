// Package abap lowers verified object HIR to ABAP 7.50.
package abap

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/oisee/abapiti/hir"
)

type emitter struct {
	p            *hir.Program
	names        *hir.Names
	files        map[string]string
	types        map[string]bool
	err          error
	valueSlots   map[string]bool
	materialized map[string]bool
	// Usage gates keep the emitted file set of programs that do not use the
	// phase-2 machinery unchanged.
	descriptors, regexpUsed, dynamicUsed, errorUsed bool
	isArrayUsed                                     bool
	descIndex                                       map[string]descRef
	descOrder                                       map[string][]string
	descCount                                       map[string]int
}

// Emit returns one source per global declaration, including all runtime dependencies.
func Emit(p *hir.Program) (map[string]string, error) {
	files, _, err := EmitNamed(p)
	return files, err
}

// EmitNamed additionally returns the name table used for emitted identities.
func EmitNamed(p *hir.Program) (map[string]string, *hir.Names, error) {
	if errors := hir.Verify(p); len(errors) > 0 {
		return nil, nil, errors[0]
	}
	e := &emitter{p: p, names: hir.NewNames(), files: map[string]string{}, types: map[string]bool{}, descCount: map[string]int{}}
	e.scanUsage()
	e.promoteValueSlots()
	if e.descriptors {
		n := e.name("runtime.described")
		e.files[n+".intf.abap"] = "INTERFACE " + n + " PUBLIC.\nMETHODS " + e.name("builtin.classOf") + " RETURNING VALUE(result) TYPE REF TO " + e.name("runtime.classvalue") + ".\nENDINTERFACE.\n"
	}
	for _, i := range p.Interfaces {
		var b strings.Builder
		fmt.Fprintf(&b, "INTERFACE %s PUBLIC.\n", e.name(i.Name))
		for _, m := range i.Methods {
			b.WriteString(e.signature(m, false))
		}
		if e.descriptors {
			fmt.Fprintf(&b, "METHODS %s RETURNING VALUE(result) TYPE REF TO %s.\n", e.name("builtin.classOf"), e.name("runtime.classvalue"))
		}
		b.WriteString("ENDINTERFACE.\n")
		e.files[e.name(i.Name)+".intf.abap"] = b.String()
	}
	for _, c := range p.Classes {
		e.class(c)
	}
	e.support()
	e.markArrays()
	if castTrace {
		e.traceCasts()
	}
	for file, src := range e.files {
		out, err := wrap(src)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", file, err)
		}
		e.files[file] = out
	}
	if e.err != nil {
		return nil, nil, e.err
	}
	return e.files, e.names, nil
}
func (e *emitter) name(s string) string   { return e.names.Get(s) }
func (e *emitter) member(s string) string { return e.name("member." + s) }
func (e *emitter) param(s string) string  { return e.name("param." + s) }
func (e *emitter) classBy(n string) *hir.Class {
	for _, c := range e.p.Classes {
		if c.Name == n {
			return c
		}
	}
	return nil
}
func (e *emitter) method(c *hir.Class, n string) (*hir.Method, *hir.Class) {
	for ; c != nil; c = e.classBy(c.Super) {
		for _, m := range c.Methods {
			if m.Name == n {
				return m, c
			}
		}
	}
	return nil, nil
}

// Reference arrays share one object table. Typed views cast elements when read,
// preserving the array object and mutations across covariant/generic views.
func arrayStorage(t hir.Type) hir.Type {
	if t.Kind == hir.Array && len(t.Args) == 1 && t.Args[0].IsRef() {
		return hir.T(hir.Array, hir.Ref(hir.RootObject))
	}
	return t
}
func (e *emitter) typ(t hir.Type) string {
	t = arrayStorage(t)
	switch t.Kind {
	case hir.Bool:
		return "abap_bool"
	case hir.Number:
		return "f"
	case hir.I32:
		return "i"
	case hir.I64:
		return "int8"
	case hir.String:
		return "string"
	case hir.ClassRef:
		if t.Name == hir.RootObject {
			return "REF TO object"
		}
		if t.Name == "builtin.Error" {
			e.errorUsed = true
		}
		return "REF TO " + e.name(t.Name)
	case hir.InterfaceRef:
		return "REF TO " + e.name(t.Name)
	case hir.ClassValue:
		e.descriptors = true
		return "REF TO " + e.name("runtime.classvalue")
	case hir.RegExp:
		e.regexpUsed = true
		return "REF TO " + e.name("runtime.regexp")
	case hir.Dynamic:
		e.dynamicUsed = true
		return "REF TO " + e.name("runtime.dynamic")
	case hir.Optional:
		if t.Args[0].IsRef() {
			return e.typ(t.Args[0])
		}
		fallthrough
	case hir.Array, hir.OrderedMap, hir.OrderedSet:
		e.runtime(t)
		return "REF TO " + e.name("runtime."+t.String())
	default:
		e.err = fmt.Errorf("unsupported ABAP type %s", t)
		return "i"
	}
}
func (e *emitter) signature(m *hir.Method, ctor bool) string {
	if m.Name == "class_constructor" && m.Static {
		return "CLASS-METHODS class_constructor.\n"
	}
	kw, name := "METHODS", e.member(m.Name)
	if ctor {
		name = "constructor"
	}
	if m.Static {
		kw = "CLASS-METHODS"
	}
	s := kw + " " + name
	if m.Abstract {
		s += " ABSTRACT"
	}
	if len(m.Params) > 0 {
		s += " IMPORTING"
		for _, p := range m.Params {
			s += " " + e.param(p.Name) + " TYPE " + e.typ(p.Type)
			if ctor && p.Type.Kind == hir.Optional {
				s += " OPTIONAL"
			}
		}
	}
	if m.Result.Kind != hir.Void {
		s += " RETURNING VALUE(result) TYPE " + e.typ(m.Result)
	}
	return s + ".\n"
}

// scanUsage pre-sets the phase-2 gates so the hidden classOf method is
// present in every class whenever descriptors could be needed.
func (e *emitter) scanUsage() {
	need := false
	var walk func(x *hir.Expr)
	walk = func(x *hir.Expr) {
		if x == nil {
			return
		}
		if x.Kind == hir.ClassOf || x.Type.Kind == hir.ClassValue {
			need = true
		}
		if x.Kind == hir.RuntimeOp && (x.Op == "object.classOf" || x.Op == "classvalue.new" || x.Op == "classvalue.name" || x.Op == "classvalue.has") {
			need = true
		}
		if x.Kind == hir.InstanceOf && x.Y != nil {
			need = true
		}
		if x.Kind == hir.RuntimeOp && x.Op == "dynamic.materialize" {
			e.materializer(x.Type)
		}
		if x.Type.Kind == hir.RegExp {
			e.regexpUsed = true
		}
		if x.Type.Kind == hir.Dynamic {
			e.dynamicUsed = true
		}
		walk(x.X)
		walk(x.Y)
		walk(x.Z)
		for _, a := range x.Args {
			walk(a)
		}
	}
	for _, c := range e.p.Classes {
		if c.Ctor != nil && c.Ctor.Body != nil {
			e.walkStmt(c.Ctor.Body, walk)
		}
		for _, m := range c.Methods {
			if m.Body != nil {
				e.walkStmt(m.Body, walk)
			}
		}
	}
	if need {
		e.descriptors = true
	}
}

func (e *emitter) walkStmt(s *hir.Stmt, walk func(*hir.Expr)) {
	if s == nil {
		return
	}
	walk(s.X)
	walk(s.Y)
	e.walkStmt(s.Body, walk)
	e.walkStmt(s.Else, walk)
	for _, x := range s.List {
		e.walkStmt(x, walk)
	}
}

// slotKey identifies the first declaration of an inherited virtual slot.
func (e *emitter) slotKey(c *hir.Class, name string) string {
	owner := c
	for base := e.classBy(c.Super); base != nil; base = e.classBy(base.Super) {
		if _, found := e.method(base, name); found != nil {
			owner = found
		}
	}
	return owner.Name + "." + name
}

// TS permits a value-returning override of a void method. ABAP needs one
// return signature for the entire inheritance slot so the redefinition can
// retain that value. Void callers and interface wrappers still discard it.
func (e *emitter) promoteValueSlots() {
	e.valueSlots = map[string]bool{}
	for _, c := range e.p.Classes {
		for _, m := range c.Methods {
			original, _, specialized := strings.Cut(m.Name, "_instantiated_")
			if !specialized || !m.Result.IsRef() {
				continue
			}
			slot, _ := e.method(c, original)
			base, _ := e.method(e.classBy(c.Super), original)
			if slot != nil && base != nil && slot.Result.Kind == hir.Void {
				e.valueSlots[e.slotKey(c, original)] = true
			}
		}
	}
}
func (e *emitter) emittedMethod(c *hir.Class, m *hir.Method) *hir.Method {
	if m.Result.Kind != hir.Void || !e.valueSlots[e.slotKey(c, m.Name)] {
		return m
	}
	clone := *m
	clone.Result = hir.Ref(hir.RootObject)
	return &clone
}

// overrideBody finds the checker-typed implementation behind an erased slot.
// ABAP requires SUPER->m to occur in METHOD m, so the implementation body
// belongs to the inherited slot; the checker-typed variant forwards to it.
func (e *emitter) overrideBody(c *hir.Class, slot *hir.Method) *hir.Method {
	if base, _ := e.method(e.classBy(c.Super), slot.Name); base == nil {
		return nil
	}
	for _, m := range c.Methods {
		if m.Name == slot.Name+"_instantiated_"+c.Name {
			return m
		}
	}
	return nil
}

func (e *emitter) narrowedBridge(c *hir.Class, m, slot *hir.Method) string {
	slot = e.emittedMethod(c, slot)
	b := &body{e: e, c: c, m: m, implemented: e.member(m.Name), locals: map[string]string{}}
	for _, p := range m.Params {
		n := b.temp(p.Type)
		b.locals[p.Name] = n
		b.line(n + " = " + e.param(p.Name) + ".")
	}
	args := []string{}
	for j, p := range slot.Params {
		if j >= len(m.Params) {
			// This implementation declares no binding for the trailing argument.
			// The inherited slot body is this same implementation; it cannot
			// observe the ABI filler (JS arguments/rest are not erased here).
			args = append(args, e.param(p.Name)+" = "+b.temp(p.Type))
			continue
		}
		actual := m.Params[j]
		args = append(args, e.param(p.Name)+" = "+b.value(hir.V(actual.Name, actual.Type), p.Type))
	}
	call := "me->" + e.member(slot.Name) + "( " + strings.Join(args, " ") + " )"
	if m.Result.Kind == hir.Void {
		b.line(call + ".")
	} else {
		if slot.Result.Kind == hir.Void {
			e.err = fmt.Errorf("class %s method %s: value-returning override of void superclass slot is unsupported", c.Name, slot.Name)
			return ""
		}
		result := b.temp(slot.Result)
		b.line(result + " = " + call + ".")
		op := " = "
		if !m.Result.Equal(slot.Result) {
			op = " ?= "
		}
		b.line("result" + op + result + ".")
	}
	b.line("RETURN.")
	return "METHOD " + b.implemented + ".\n" + b.code.String() + "ENDMETHOD.\n"
}

func (e *emitter) class(c *hir.Class) {
	var b strings.Builder
	s := "CLASS " + e.name(c.Name) + " DEFINITION PUBLIC"
	if c.Super != "" {
		s += " INHERITING FROM " + e.name(c.Super)
	}
	if c.Abstract {
		s += " ABSTRACT"
	}
	s += " CREATE PUBLIC.\nPUBLIC SECTION.\n"
	b.WriteString(s)
	if e.materialized[c.Name] {
		fmt.Fprintf(&b, "DATA %s TYPE REF TO %s.\n", e.name("builtin.materializedSource"), e.name("runtime.dynamic"))
	}
	for _, i := range c.Implements {
		fmt.Fprintf(&b, "INTERFACES %s.\n", e.name(i))
	}
	for _, f := range c.Fields {
		kw := "DATA"
		if f.Static {
			kw = "CLASS-DATA"
		}
		fmt.Fprintf(&b, "%s %s TYPE %s.\n", kw, e.member(f.Name), e.typ(f.Type))
	}
	if c.Ctor != nil && !e.wideShape(c) {
		b.WriteString(e.signature(c.Ctor, true))
	}
	for _, m := range c.Methods {
		if m.Name == "class_constructor" && m.Static {
			fmt.Fprintf(&b, "CLASS-DATA %s TYPE abap_bool.\nCLASS-METHODS %s.\n", e.name("builtin.initialized."+c.Name), e.name("builtin.initialize."+c.Name))
			continue
		}
		base, _ := e.method(e.classBy(c.Super), m.Name)
		if base != nil && m.Abstract {
			// An abstract redeclaration of an inherited (abstract) method adds
			// nothing ABAP can express.
			continue
		}
		if base != nil {
			fmt.Fprintf(&b, "METHODS %s REDEFINITION.\n", e.member(m.Name))
		} else {
			b.WriteString(e.signature(e.emittedMethod(c, m), false))
		}
	}
	if e.descriptors {
		if c.Super != "" {
			fmt.Fprintf(&b, "METHODS %s REDEFINITION.\n", e.name("builtin.classOf"))
		} else {
			fmt.Fprintf(&b, "INTERFACES %s.\n", e.name("runtime.described"))
			fmt.Fprintf(&b, "METHODS %s RETURNING VALUE(result) TYPE REF TO %s.\n", e.name("builtin.classOf"), e.name("runtime.classvalue"))
		}
	}
	b.WriteString("PROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + e.name(c.Name) + " IMPLEMENTATION.\n")
	if c.Ctor != nil && !e.wideShape(c) {
		b.WriteString(e.body(c, c.Ctor, "constructor"))
	}
	for _, m := range c.Methods {
		if m.Abstract {
			continue
		}
		name := e.member(m.Name)
		if m.Name == "class_constructor" && m.Static {
			name = e.name("builtin.initialize." + c.Name)
		}
		if impl := e.overrideBody(c, m); impl != nil {
			if e.forwardsToImplementation(impl, m) {
				b.WriteString(e.slotForwarder(c, m, impl))
			} else {
				b.WriteString(e.overrideImplementation(c, impl, m))
			}
		} else if original, _, specialized := strings.Cut(m.Name, "_instantiated_"); specialized {
			slot, _ := e.method(c, original)
			if slot != nil && e.overrideBody(c, slot) == m && !e.forwardsToImplementation(m, slot) {
				b.WriteString(e.narrowedBridge(c, m, slot))
			} else {
				b.WriteString(e.body(c, m, name))
			}
		} else {
			b.WriteString(e.body(c, m, name))
		}
	}
	if e.descriptors {
		name := e.name("builtin.classOf")
		if c.Super == "" {
			fmt.Fprintf(&b, "METHOD %s~%s.\nresult = me->%s( ).\nENDMETHOD.\n", e.name("runtime.described"), name, name)
		}
		fmt.Fprintf(&b, "METHOD %s.\nresult = %s.\nENDMETHOD.\n", name, e.descriptorOf(c.Name))
	}
	for _, n := range c.Implements {
		for _, i := range e.p.Interfaces {
			if i.Name != n {
				continue
			}
			if e.descriptors {
				fmt.Fprintf(&b, "METHOD %s~%s.\nresult = me->%s( ).\nENDMETHOD.\n", e.name(n), e.name("builtin.classOf"), e.name("builtin.classOf"))
			}
			for _, m := range i.Methods {
				fmt.Fprintf(&b, "METHOD %s~%s.\n", e.name(n), e.member(m.Name))
				s := "me->" + e.member(m.Name) + "( "
				if len(m.Params) > 0 {
					for _, p := range m.Params {
						s += " " + e.param(p.Name) + " = " + e.param(p.Name)
					}
				}
				if m.Result.Kind != hir.Void {
					s = "result = " + s
				}
				b.WriteString(s + " ).\nENDMETHOD.\n")
			}
		}
	}
	b.WriteString("ENDCLASS.\n")
	e.files[e.name(c.Name)+".clas.abap"] = b.String()
}

type body struct {
	e                            *emitter
	c                            *hir.Class
	m                            *hir.Method
	implemented                  string
	code                         strings.Builder
	locals                       map[string]string
	serial                       int
	lastInit, lastName, lastType string
}

func (e *emitter) body(c *hir.Class, m *hir.Method, name string) string {
	m = e.emittedMethod(c, m)
	b := &body{e: e, c: c, m: m, implemented: name, locals: map[string]string{}}
	if m.Name == "class_constructor" && m.Static {
		flag := e.name("builtin.initialized." + c.Name)
		b.line("IF " + flag + " = abap_true.")
		b.line("RETURN.")
		b.line("ENDIF.")
		b.line(flag + " = abap_true.")
	} else if m.Static || m.Name == "constructor" {
		b.initialize(c.Name)
	}
	for _, p := range m.Params {
		n := b.temp(p.Type)
		b.locals[p.Name] = n
		b.line(n + " = " + e.param(p.Name) + ".")
	}
	b.stmt(m.Body)
	return "METHOD " + name + ".\n" + b.code.String() + "ENDMETHOD.\n"
}

func (e *emitter) overrideImplementation(c *hir.Class, impl, slot *hir.Method) string {
	clone := *impl
	clone.Result = e.emittedMethod(c, slot).Result
	b := &body{e: e, c: c, m: &clone, implemented: e.member(slot.Name), locals: map[string]string{}}
	for j, p := range impl.Params {
		inherited := slot.Params[j]
		// A required primitive cannot represent undefined passed through the
		// inherited optional slot. Reading only ->value would silently replace
		// undefined with an initial primitive (or dereference an unbound ref).
		if inherited.Type.Kind == hir.Optional && !inherited.Type.Args[0].IsRef() && p.Type.Kind != hir.Optional {
			e.err = fmt.Errorf("class %s method %s parameter %s: narrowing an inherited optional primitive cannot preserve undefined", c.Name, slot.Name, p.Name)
			return ""
		}
		n := b.temp(p.Type)
		b.locals[p.Name] = n
		b.line(n + " = " + b.convert(e.param(slot.Params[j].Name), slot.Params[j].Type, p.Type) + ".")
	}
	b.stmt(impl.Body)
	return "METHOD " + b.implemented + ".\n" + b.code.String() + "ENDMETHOD.\n"
}

// Explicit lazy initialization avoids eager module constructors calling a
// registry module before the runtime has registered all translated classes.
func (b *body) initialize(owner string) {
	c := b.e.classBy(owner)
	if c == nil {
		return
	}
	for _, m := range c.Methods {
		if m.Name == "class_constructor" && m.Static {
			b.line("CALL METHOD " + b.e.name(owner) + "=>" + b.e.name("builtin.initialize."+c.Name) + ".")
			return
		}
	}
}

// Temporaries are initialized at their evaluation point, including each loop
// iteration. Fold an immediately following assignment into its declaration;
// explicit conversions retain the HIR type rather than ABAP literal inference.
func (b *body) line(s string) {
	prefix := b.lastName + " = "
	if b.lastInit != "" && b.lastType != "REF TO object" && strings.HasPrefix(s, prefix) {
		code := b.code.String()
		b.code.Reset()
		b.code.WriteString(strings.TrimSuffix(code, b.lastInit))
		rhs := strings.TrimSuffix(strings.TrimPrefix(s, prefix), ".")
		if strings.HasPrefix(b.lastType, "REF TO ") {
			typ := strings.TrimPrefix(b.lastType, "REF TO ")
			if strings.HasPrefix(rhs, "NEW #( ") {
				rhs = strings.Replace(rhs, "NEW #", "NEW "+typ, 1)
			}
			if strings.HasPrefix(rhs, "NEW "+typ+"(") || strings.HasPrefix(rhs, "NEW "+typ+" (") {
				s = "DATA(" + b.lastName + ") = " + rhs + "."
			} else {
				s = "DATA(" + b.lastName + ") = CAST " + typ + "( " + rhs + " )."
			}
		} else {
			s = "DATA(" + b.lastName + ") = CONV " + b.lastType + "( " + rhs + " )."
		}
	}
	b.lastInit, b.lastName, b.lastType = "", "", ""
	b.code.WriteString(s + "\n")
}
func (b *body) temp(t hir.Type) string { return b.rawTemp(b.e.typ(t)) }
func (b *body) rawTemp(typ string) string {
	b.serial++
	n := fmt.Sprintf("t%d", b.serial)
	var init string
	if strings.HasPrefix(typ, "REF TO ") || strings.Contains(typ, " LENGTH ") {
		init = "DATA " + n + " TYPE " + typ + ".\nCLEAR " + n + ".\n"
	} else {
		init = "DATA(" + n + ") = VALUE " + typ + "( ).\n"
	}
	b.line(strings.TrimSuffix(init, "\n"))
	if !strings.Contains(typ, " LENGTH ") {
		b.lastInit, b.lastName, b.lastType = init, n, typ
	}
	return n
}
func (b *body) convert(value string, src, dst hir.Type) string {
	if src.Equal(dst) {
		return value
	}
	if dst.Kind == hir.Dynamic {
		// A typed value into an erased `any` slot: box it.
		n := b.temp(dst)
		b.boxDynamic(n, value, src)
		return n
	}
	if src.Kind == hir.Dynamic && (dst.IsRef() || dst.Kind == hir.String || dst.Kind == hir.Number || dst.Kind == hir.Bool || dst.Kind == hir.ClassValue) {
		// An erased `any` into the implementation's type: checked unboxing.
		return b.unbox(value, dst)
	}
	if src.Kind == hir.InterfaceRef && dst.Kind == hir.InterfaceRef && !src.Equal(dst) {
		n := b.temp(dst)
		b.line(n + " ?= " + value + ".")
		return n
	}
	if (src.IsRef() || (src.Kind == hir.Optional && src.Args[0].IsRef())) && (dst.IsRef() || (dst.Kind == hir.Optional && dst.Args[0].IsRef())) {
		from, to := src, dst
		if from.Kind == hir.Optional {
			from = from.Args[0]
		}
		if to.Kind == hir.Optional {
			to = to.Args[0]
		}
		if (from.Kind == hir.ClassRef || from.Kind == hir.InterfaceRef) && (to.Kind == hir.ClassRef || to.Kind == hir.InterfaceRef) && !b.e.upcast(from, to) {
			// A checked cast through the object root (class/interface relation unknown statically).
			root := b.rawTemp("REF TO object")
			b.line(root + " = " + value + ".")
			n := b.temp(dst)
			b.line(n + " ?= " + root + ".")
			return n
		}
	}
	if dst.Kind == hir.Optional && src.Kind != hir.Optional && !dst.Args[0].IsRef() {
		n := b.temp(dst)
		b.line(n + " = NEW #( ).")
		b.line(n + "->has = abap_true.")
		b.line(n + "->value = " + value + ".")
		return n
	}
	return value
}
func (b *body) value(x *hir.Expr, dst hir.Type) string { return b.convert(b.expr(x), x.Type, dst) }
func (b *body) expr(x *hir.Expr) string {
	if x == nil {
		return ""
	}
	e, t := b.e, x.Type
	if t.Kind == hir.Void {
		if x.Kind == hir.RuntimeOp {
			// A void runtime operation at statement level (dynamic.put).
			b.runtimeOp(x, "")
			return ""
		}
		if x.Kind == hir.Seq {
			// A void sequence: its statements, then its (void) value if any.
			old := b.locals
			b.locals = map[string]string{}
			for k, v := range old {
				b.locals[k] = v
			}
			for _, stmt := range x.Stmt.List {
				b.stmt(stmt)
			}
			b.expr(x.Y)
			b.locals = old
			return ""
		}
		b.call(x, "")
		return ""
	}
	n := b.temp(t)
	switch x.Kind {
	case hir.Lit:
		if x.Value == nil {
			break
		}
		lt := t
		target := n
		if t.Kind == hir.Optional {
			lt = t.Args[0]
			b.line(n + " = NEW #( ).")
			b.line(n + "->has = abap_true.")
			target = n + "->value"
		}
		switch lt.Kind {
		case hir.String:
			value, ok := b.literalString(x)
			if ok {
				b.stringLit(target, value)
			}
		case hir.Bool:
			s := "abap_false"
			value, ok := x.Value.(bool)
			if !ok {
				e.err = fmt.Errorf("node %d (%s): expected bool literal, got %T", x.ID, x.Source, x.Value)
				break
			}
			if value {
				s = "abap_true"
			}
			b.line(target + " = " + s + ".")
		case hir.I64:
			value, err := strconv.ParseInt(fmt.Sprint(x.Value), 10, 64)
			if err != nil {
				e.err = fmt.Errorf("node %d (%s): invalid I64 literal: %w", x.ID, x.Source, err)
				break
			}
			b.int8Lit(target, value)
		default:
			if f, err := strconv.ParseFloat(fmt.Sprint(x.Value), 64); err == nil && (math.IsNaN(f) || math.IsInf(f, 0)) {
				e.err = fmt.Errorf("node %d (%s): ABAP f cannot represent non-finite Number", x.ID, x.Source)
				break
			}
			b.line(target + " = '" + fmt.Sprint(x.Value) + "'.")
		}
	case hir.Local:
		b.line(n + " = " + b.locals[x.Name] + ".")
	case hir.This:
		b.line(n + " = me.")
	case hir.FieldGet:
		b.line(n + " = " + b.expr(x.X) + "->" + e.member(x.Name) + ".")
	case hir.StaticGet:
		b.initialize(x.Owner)
		b.line(n + " = " + e.name(x.Owner) + "=>" + e.member(x.Name) + ".")
	case hir.IndexGet:
		a, i := b.expr(x.X), b.expr(x.Y)
		b.line(i + " = " + i + " + 1.")
		row := n
		if t.IsRef() {
			row = b.temp(hir.Ref(hir.RootObject))
		}
		b.line("READ TABLE " + a + "->items INDEX " + i + " INTO " + row + ".")
		if row != n {
			b.line(n + " ?= " + row + ".")
		}
	case hir.New:
		if t.Kind == hir.Array {
			// new Array<T>(n): n still-undefined slots.
			b.line("CREATE OBJECT " + n + ".")
			if len(x.Args) == 1 {
				cnt := b.expr(x.Args[0])
				row := b.temp(arrayStorage(t).Args[0])
				b.line("CLEAR " + row + ".")
				b.line("WHILE " + cnt + " > 0.")
				b.line("APPEND " + row + " TO " + n + "->items.")
				b.line(cnt + " = " + cnt + " - 1.")
				b.line("ENDWHILE.")
			}
			break
		}
		if t.Kind == hir.RegExp {
			e.regexpUsed = true
			original := b.value(x.Args[0], hir.T(hir.String))
			pattern, reject := original, ""
			if raw, ok := x.Args[0].Value.(string); x.Args[0].Kind == hir.Lit && ok {
				translated, excluded := regexPattern(raw)
				if translated != raw || excluded != "" {
					pattern = b.temp(hir.T(hir.String))
					b.stringLit(pattern, translated)
					if excluded != "" {
						reject = b.temp(hir.T(hir.String))
						b.stringLit(reject, excluded)
					}
				}
			}
			s := "CREATE OBJECT " + n + " TYPE " + e.name("runtime.regexp")
			s += " EXPORTING pattern = " + pattern
			if len(x.Args) > 1 {
				s += " flags = " + b.value(x.Args[1], hir.T(hir.String))
			} else {
				s += " flags = ``"
			}
			if reject != "" {
				s += " excluded_pattern = " + reject
			}
			b.line(s + ".")
			if pattern != original {
				b.line(n + "->source = " + original + ".")
			}
			break
		}
		b.initialize(t.Name)
		if c := e.classBy(t.Name); c != nil && e.wideShape(c) {
			// A wide shape has no constructor: its fields are set one by one.
			values := make([]string, len(x.Args))
			for i, a := range x.Args {
				field := c.Ctor.Body.List[i].X.Type
				if a.Type.Kind == hir.Optional && !a.Type.Equal(field) && c.Ctor.Body.List[i].Y.Kind == hir.Narrow {
					// The constructor narrowed (checked) the optional parameter.
					values[i] = b.expr(&hir.Expr{Kind: hir.Narrow, Node: a.Node, Type: field, X: a})
				} else {
					values[i] = b.value(a, field)
				}
			}
			b.line(n + " = NEW " + strings.TrimPrefix(e.typ(t), "REF TO ") + "( ).")
			for i := range values {
				b.line(n + "->" + e.member(c.Ctor.Params[i].Name) + " = " + values[i] + ".")
			}
			break
		}
		s := n + " = NEW " + strings.TrimPrefix(e.typ(t), "REF TO ") + "( "
		args := []string{}
		ctor := e.p.Constructor(t.Name)
		for i, a := range x.Args {
			args = append(args, e.param(ctor.Params[i].Name)+" = "+b.value(a, ctor.Params[i].Type))
		}
		if len(args) > 0 {
			s += strings.Join(args, " ")
		}
		b.line(s + " ).")
	case hir.DirectCall, hir.VirtualCall, hir.SuperCall:
		b.call(x, n)
	case hir.Binary:
		a := b.expr(x.X)
		if x.Op == "&&" || x.Op == "||" {
			b.line(n + " = " + a + ".")
			test := "abap_true"
			if x.Op == "||" {
				test = "abap_false"
			}
			b.line("IF " + a + " = " + test + ".")
			b.line(n + " = " + b.expr(x.Y) + ".")
			b.line("ENDIF.")
			break
		}

		z := b.expr(x.Y)
		if (x.Op == "/" || x.Op == "%") && (t.Kind == hir.I32 || t.Kind == hir.I64) {
			rem := b.temp(t)
			b.line(rem + " = " + a + " MOD " + z + ".")
			if x.Op == "/" {
				b.line(n + " = " + a + " DIV " + z + ".")
			} else {
				b.line(n + " = " + rem + ".")
			}
			b.line("IF " + a + " < 0 AND " + rem + " <> 0.")
			b.line("IF " + z + " > 0.")
			if x.Op == "/" {
				b.line(n + " = " + n + " + 1.")
			} else {
				b.line(n + " = " + n + " - " + z + ".")
			}
			b.line("ELSE.")
			if x.Op == "/" {
				b.line(n + " = " + n + " - 1.")
			} else {
				b.line(n + " = " + n + " + " + z + ".")
			}
			b.line("ENDIF.")
			b.line("ENDIF.")
			break
		}
		if x.Op == "%" && t.Kind == hir.Number {
			e.err = fmt.Errorf("node %d (%s): Number remainder requires an IEEE runtime", x.ID, x.Source)
			break
		}
		if x.Op == "/" && t.Kind == hir.Number {
			divisor, err := strconv.ParseFloat(fmt.Sprint(x.Y.Value), 64)
			if x.Y.Kind != hir.Lit || err != nil || divisor == 0 || math.IsNaN(divisor) || math.IsInf(divisor, 0) {
				e.err = fmt.Errorf("node %d (%s): Number division requires a non-zero literal divisor", x.ID, x.Source)
				break
			}
		}
		if (x.Op == "==" || x.Op == "!=") && x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			// Presence is independent of box identity; guard both dereferences.
			ah, zh := b.temp(hir.T(hir.Bool)), b.temp(hir.T(hir.Bool))
			for _, pair := range [][2]string{{a, ah}, {z, zh}} {
				b.line("IF " + pair[0] + " IS BOUND.")
				b.line(pair[1] + " = " + pair[0] + "->has.")
				b.line("ENDIF.")
			}
			b.line("IF " + ah + " = " + zh + ".")
			b.line("IF " + ah + " = abap_false.")
			b.line(n + " = abap_true.")
			b.line("ELSEIF " + a + "->value = " + z + "->value.")
			b.line(n + " = abap_true.")
			b.line("ENDIF.")
			b.line("ENDIF.")
			if x.Op == "!=" {
				b.line("IF " + n + " = abap_true.")
				b.line(n + " = abap_false.")
				b.line("ELSE.")
				b.line(n + " = abap_true.")
				b.line("ENDIF.")
			}
			break
		}
		op := map[string]string{"==": "=", "!=": "<>", "%": "MOD"}[x.Op]
		if op == "" {
			op = x.Op
		}
		if t.Kind == hir.Bool {
			b.line(n + " = xsdbool( " + a + " " + op + " " + z + " ).")
		} else {
			b.line(n + " = " + a + " " + op + " " + z + ".")
		}
	case hir.Unary:
		a := b.expr(x.X)
		if x.Op == "!" {
			b.line(n + " = xsdbool( " + a + " = abap_false ).")
		} else {
			b.line(n + " = 0 - " + a + ".")
		}
	case hir.Conditional:
		a := b.expr(x.X)
		b.line("IF " + a + " = abap_true.")
		b.line(n + " = " + b.value(x.Y, t) + ".")
		b.line("ELSE.")
		b.line(n + " = " + b.value(x.Z, t) + ".")
		b.line("ENDIF.")
	case hir.InstanceOf:
		a := b.expr(x.X)
		if x.Y != nil {
			// A dynamic class-value operand: walk the descriptor chain.
			cv := b.expr(x.Y)
			d := b.temp(hir.T(hir.ClassValue))
			b.line("IF " + a + " IS BOUND.")
			if x.X.Type.Name == hir.RootObject {
				ref := b.rawTemp("REF TO " + e.name("runtime.described"))
				b.line(ref + " ?= " + a + ".")
				a = ref
			}
			b.line(d + " = " + a + "->" + b.e.name("builtin.classOf") + "( ).")
			b.line("WHILE " + d + " IS BOUND.")
			b.line("IF " + d + " = " + cv + ".")
			b.line(n + " = abap_true.")
			b.line("EXIT.")
			b.line("ENDIF.")
			b.line(d + " = " + d + "->parent.")
			b.line("ENDWHILE.")
			b.line("ENDIF.")
			break
		}
		b.line(n + " = xsdbool( " + a + " IS BOUND AND " + a + " IS INSTANCE OF " + e.name(x.Owner) + " ).")
	case hir.ClassOf:
		e.descriptors = true
		b.line(n + " = " + e.descriptorOf(x.Owner) + ".")
	case hir.Cast:
		a := b.expr(x.X)
		b.line(n + " ?= " + a + ".")
	case hir.Seq:
		old := b.locals
		b.locals = map[string]string{}
		for k, v := range old {
			b.locals[k] = v
		}
		for _, stmt := range x.Stmt.List {
			b.stmt(stmt)
		}
		b.line(n + " = " + b.expr(x.Y) + ".")
		b.locals = old
	case hir.Narrow:
		a := b.expr(x.X)
		source := x.X.Type
		if source.Kind == hir.Optional {
			source = source.Args[0]
		}
		target := x.Type
		if target.Kind == hir.Optional {
			target = target.Args[0]
		}
		if (source.Kind == hir.ClassRef && target.Kind == hir.InterfaceRef) || (source.Kind == hir.InterfaceRef && target.Kind == hir.ClassRef) {
			// A class/interface cross cast: widen to the object root first so
			// the checked `?=` is valid whatever the static relation.
			root := b.temp(hir.Ref(hir.RootObject))
			b.line(root + " = " + a + ".")
			a = root
		}
		if x.X.Type.Kind == hir.Optional {
			base := x.X.Type.Args[0]
			if !base.IsRef() {
				a += "->value"
			}
			if base.Equal(x.Type) {
				b.line(n + " = " + a + ".")
			} else {
				b.line(n + " ?= " + a + ".")
			}
		} else {
			b.line(n + " ?= " + a + ".")
		}
	case hir.IsUndefined:
		a := b.expr(x.X)
		b.line("IF " + a + " IS INITIAL.")
		b.line(n + " = abap_true.")
		if x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			b.line("ELSEIF " + a + "->has = abap_false.")
			b.line(n + " = abap_true.")
		}
		b.line("ENDIF.")
	case hir.ToBoolean:
		a := b.expr(x.X)
		at := x.X.Type
		if at.Kind == hir.Optional && !at.Args[0].IsRef() {
			b.line("IF " + a + " IS BOUND.")
			b.line("IF " + a + "->has = abap_true.")
			b.truth(n, a+"->value", at.Args[0])
			b.line("ENDIF.")
			b.line("ENDIF.")
		} else {
			b.truth(n, a, at)
		}
	case hir.RuntimeOp:
		b.runtimeOp(x, n)
	default:
		e.err = fmt.Errorf("node %d: unsupported expression %s", x.ID, x.Kind)
	}
	return n
}

// jsWhiteSpace are the Unicode code points ECMAScript trim treats as white
// space: the singletons plus the U+2000..U+200A range.
var jsWhiteSpace = []int{9, 10, 11, 12, 13, 32, 160, 5760, 8232, 8233, 8239, 8287, 12288, 65279}

// trimLoop advances lo (or retreats hi) over JavaScript white space in a.
// The one-character section is materialized first: string offsets are not
// allowed as functional-method arguments.
func (b *body) trimLoop(a, lo, hi string, leading bool) {
	code := b.temp(hir.T(hir.I32))
	idx := b.temp(hir.T(hir.I32))
	ch := b.temp(hir.T(hir.String))
	cond := lo + " < " + hi
	init := ""
	step := lo + " = " + lo + " + 1."
	if leading {
		init = idx + " = " + lo + "."
	} else {
		cond = hi + " > " + lo
		init = idx + " = " + hi + " - 1."
		step = hi + " = " + hi + " - 1."
	}
	b.line("WHILE " + cond + ".")
	b.line(init)
	b.line(ch + " = " + a + "+" + idx + "(1).")
	b.codeUnit(code, ch)
	var tests []string
	for _, cp := range jsWhiteSpace {
		tests = append(tests, fmt.Sprintf("%s = %d", code, cp))
	}
	tests = append(tests, fmt.Sprintf("%s >= 8192 AND %s <= 8202", code, code))
	b.line("IF " + strings.Join(tests, " OR ") + ".")
	b.line(step)
	b.line("ELSE.")
	b.line("EXIT.")
	b.line("ENDIF.")
	b.line("ENDWHILE.")
}

// codeUnit reads one UTF-16 unit directly. The pinned library's uccpi
// uses high-byte * 255 on OSG-JS; that is not a correct Unicode code point.
// parseInt10 implements JavaScript parseInt(s, 10): leading ECMAScript white
// space is skipped, an optional sign is read, then decimal digits up to the
// first non-digit. No digit at all yields the absent box (NaN).
func (b *body) parseInt10(n, a, length string) {
	lo := b.temp(hir.T(hir.I32))
	hi := b.temp(hir.T(hir.I32))
	b.line(lo + " = 0.")
	b.line(hi + " = " + length + ".")
	b.trimLoop(a, lo, hi, true)
	sign := b.temp(hir.T(hir.Number))
	value := b.temp(hir.T(hir.Number))
	digits := b.temp(hir.T(hir.I32))
	ch := b.temp(hir.T(hir.String))
	b.line(sign + " = 1.")
	b.line(value + " = 0.")
	b.line(digits + " = 0.")
	b.line("IF " + lo + " < " + hi + ".")
	b.line(ch + " = " + a + "+" + lo + "(1).")
	b.line("IF " + ch + " = '-'.")
	b.line(sign + " = -1.")
	b.line(lo + " = " + lo + " + 1.")
	b.line("ELSEIF " + ch + " = '+'.")
	b.line(lo + " = " + lo + " + 1.")
	b.line("ENDIF.")
	b.line("ENDIF.")
	b.line("WHILE " + lo + " < " + hi + ".")
	b.line(ch + " = " + a + "+" + lo + "(1).")
	b.line("IF " + ch + " CA '0123456789' AND " + ch + " <> ` `.")
	b.line(value + " = " + value + " * 10 + ( " + ch + " ).")
	b.line(digits + " = " + digits + " + 1.")
	b.line(lo + " = " + lo + " + 1.")
	b.line("ELSE.")
	b.line("EXIT.")
	b.line("ENDIF.")
	b.line("ENDWHILE.")
	b.line("IF " + digits + " = 0.")
	b.line("CLEAR " + n + ".")
	b.line("ELSE.")
	b.line(n + " = NEW #( ).")
	b.line(n + "->has = abap_true.")
	b.line(n + "->value = " + sign + " * " + value + ".")
	b.line("ENDIF.")
}

// localeCompareNames implements a.localeCompare(b) for strings over the
// ABAP object-name alphabet. ICU root collation orders these code units as
// "_/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ" (verified against Node's
// Intl.Collator); a shorter prefix sorts first. Any other code unit in
// either operand raises cx_sy_range_out_of_bounds instead of guessing.
func (b *body) localeCompareNames(n, a, other, length string) {
	const alphabet = "_/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rank := func(s, out string) {
		b.line("IF " + s + " = ``.")
		b.line(out + " = -1.")
		b.line("ELSE.")
		b.line("FIND " + s + " IN `" + alphabet + "` MATCH OFFSET " + out + ".")
		b.line("IF sy-subrc <> 0.")
		b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
		b.line("ENDIF.")
		b.line("ENDIF.")
	}
	olen := b.temp(hir.T(hir.I32))
	idx := b.temp(hir.T(hir.I32))
	ca := b.temp(hir.T(hir.String))
	cb := b.temp(hir.T(hir.String))
	ra := b.temp(hir.T(hir.I32))
	rb := b.temp(hir.T(hir.I32))
	b.line(olen + " = strlen( " + other + " ).")
	b.line(idx + " = 0.")
	b.line(n + " = 0.")
	b.line("WHILE " + n + " = 0 AND ( " + idx + " < " + length + " OR " + idx + " < " + olen + " ).")
	b.line("CLEAR " + ca + ".")
	b.line("CLEAR " + cb + ".")
	b.line("IF " + idx + " < " + length + ".")
	b.line(ca + " = " + a + "+" + idx + "(1).")
	b.line("ENDIF.")
	b.line("IF " + idx + " < " + olen + ".")
	b.line(cb + " = " + other + "+" + idx + "(1).")
	b.line("ENDIF.")
	rank(ca, ra)
	rank(cb, rb)
	b.line("IF " + ra + " < " + rb + ".")
	b.line(n + " = -1.")
	b.line("ELSEIF " + ra + " > " + rb + ".")
	b.line(n + " = 1.")
	b.line("ENDIF.")
	b.line(idx + " = " + idx + " + 1.")
	b.line("ENDWHILE.")
}

func (b *body) codeUnit(target, ch string) {
	conv := b.rawTemp("REF TO cl_abap_conv_out_ce")
	bytes := b.rawTemp("xstring")
	low := b.rawTemp("x LENGTH 1")
	high := b.rawTemp("x LENGTH 1")
	highInt := b.temp(hir.T(hir.I32))
	b.line(conv + " = cl_abap_conv_out_ce=>create( encoding = '4103' ).")
	b.line(conv + "->convert( EXPORTING data = " + ch + " IMPORTING buffer = " + bytes + " ).")
	b.line(low + " = " + bytes + "(1).")
	b.line(high + " = " + bytes + "+1(1).")
	b.line(target + " = " + low + ".")
	b.line(highInt + " = " + high + ".")
	b.line(target + " = " + target + " + " + highInt + " * 256.")
}

func (b *body) truth(n, a string, t hir.Type) {
	test := a + " IS NOT INITIAL"
	if t.Kind == hir.Dynamic {
		b.line("IF " + a + " IS BOUND.")
		b.line(n + " = " + a + "->truth( ).")
		b.line("ENDIF.")
		return
	}
	if t.Kind == hir.String {
		test = "strlen( " + a + " ) > 0"
	}
	if t.IsRef() || t.Kind == hir.Optional {
		test = a + " IS BOUND"
	}
	b.line(n + " = xsdbool( " + test + " ).")
}
func (b *body) stringLit(n, s string) {
	// String templates preserve trailing blanks. Chunk before escaping so source
	// tokens always fit the 255-byte envelope (also bounds UTF-16 source length).
	if s == "" {
		b.line(n + " = ||.")
		return
	}
	first := true
	for s != "" {
		r := []rune(s)
		if r[0] < 32 || r[0] == 127 || r[0] == 0xfeff {
			if first {
				b.line(n + " = ||.")
			}
			char := b.rawTemp("c LENGTH 1")
			b.line(fmt.Sprintf("%s = cl_abap_conv_in_ce=>uccpi( %d ).", char, r[0]))
			b.line("CONCATENATE " + n + " " + char + " INTO " + n + " RESPECTING BLANKS.")
			s = string(r[1:])
			first = false
			continue
		}
		k := 0
		size := 0
		for k < len(r) && r[k] >= 32 && r[k] != 127 && r[k] != 0xfeff && size+len(string(r[k]))*2 <= 120 {
			size += len(string(r[k])) * 2
			k++
		}
		chunk := string(r[:k])
		s = string(r[k:])
		literal := "|" + strings.NewReplacer("\\", "\\\\", "{", "\\{", "}", "\\}", "|", "\\|").Replace(chunk) + "|"
		if strings.Contains(chunk, "\\") {
			// abaplint's lexer misreads an escaped backslash before an
			// escaped template delimiter (open-steamgate inbox 040); a
			// backquoted literal has no escapes and keeps trailing blanks.
			literal = "`" + strings.ReplaceAll(chunk, "`", "``") + "`"
		}
		if first {
			b.line(n + " = " + literal + ".")
		} else {
			b.line(n + " = " + n + " && " + literal + ".")
		}
		first = false
	}
}

// int8Lit uses only i-range literals, avoiding character-to-int8 conversion.
// Signed remainders also handle MinInt64 without negating it. Each intermediate
// is a prefix of the final value and fits int8; the target keeps arithmetic exact.
func (b *body) int8Lit(target string, value int64) {
	const base int64 = 1_000_000_000
	var parts []int64
	for value < math.MinInt32 || value > math.MaxInt32 {
		parts = append(parts, value%base)
		value /= base
	}
	b.line(fmt.Sprintf("%s = %d.", target, value))
	for i := len(parts) - 1; i >= 0; i-- {
		b.line(fmt.Sprintf("%s = %s * %d.", target, target, base))
		// Subtraction avoids a binary plus followed by a unary minus in ABAP.
		if parts[i] < 0 {
			b.line(fmt.Sprintf("%s = %s - %d.", target, target, -parts[i]))
		} else {
			b.line(fmt.Sprintf("%s = %s + %d.", target, target, parts[i]))
		}
	}
}

func (b *body) literalString(x *hir.Expr) (string, bool) {
	value, ok := x.Value.(string)
	if !ok {
		b.e.err = fmt.Errorf("node %d (%s): expected string literal, got %T", x.ID, x.Source, x.Value)
	}
	return value, ok
}

func (b *body) call(x *hir.Expr, n string) {
	e := b.e
	owner := x.Owner
	recv := ""
	var m *hir.Method
	if x.Kind == hir.SuperCall {
		owner = b.c.Super
		recv = "super->"
	} else if x.X != nil {
		recv = b.expr(x.X) + "->"
		owner = x.X.Type.Name
	} else {
		recv = e.name(owner) + "=>"
	}
	callName := x.Name
	if x.Kind == hir.SuperCall {
		if original, _, specialized := strings.Cut(callName, "_instantiated_"); specialized {
			callName = original
		}
	}
	m, _ = e.method(e.classBy(owner), callName)
	if m != nil {
		m = e.emittedMethod(e.classBy(owner), m)
	}
	member := e.member(callName)
	if x.Kind == hir.SuperCall && x.Name == "constructor" {
		m = e.p.Constructor(owner)
		member = "constructor"
	}
	if x.Kind == hir.SuperCall && member != b.implemented && !e.overriddenBelow(b.c.Name, callName) {
		// No class from here down redefines the member: virtual dispatch on
		// me reaches exactly the inherited implementation super names.
		recv = "me->"
	} else if x.Kind == hir.SuperCall && member != b.implemented {
		e.err = fmt.Errorf("node %d (%s): unsupported super call to %s from %s: SUPER-> can only call the previous implementation of the same method", x.ID, x.Source, callName, b.m.Name)
		return
	}
	if m == nil {
		for _, i := range e.p.Interfaces {
			if i.Name == owner {
				for _, f := range i.Methods {
					if f.Name == x.Name {
						m = f
					}
				}
				// Interface-typed receivers use the unqualified interface method.
			}
		}
	}
	args := []string{}
	for i, a := range x.Args {
		args = append(args, e.param(m.Params[i].Name)+" = "+b.value(a, m.Params[i].Type))
	}
	target := n
	if x.Kind == hir.SuperCall && n != "" && !x.Type.Equal(m.Result) {
		target = b.temp(m.Result)
	}
	s := recv + member + "( " + strings.Join(args, " ") + " )"
	if n != "" {
		s = target + " = " + s
	}
	b.line(s + ".")
	if target != n {
		b.line(n + " ?= " + target + ".")
	}
}
func (b *body) runtimeOp(x *hir.Expr, n string) {
	a := b.expr(x.X)
	args := []string{}
	ps, _, _ := hir.RuntimeSignature(x.Op, x.X.Type)
	for i, v := range x.Args {
		args = append(args, b.value(v, ps[i]))
	}
	if x.Op == "number.remainder2" {
		// Division/multiplication by two are exact for finite binary64.
		// Truncating the quotient and subtracting gives JS signed remainder.
		q := b.temp(hir.T(hir.Number))
		b.line(q + " = " + a + " / 2.")
		b.line(q + " = trunc( " + q + " ).")
		b.line(q + " = " + q + " * 2.")
		b.line(n + " = " + a + " - " + q + ".")
		return
	}
	if x.Op == "number.fromI32" {
		b.line(n + " = " + a + ".")
		return
	}
	if x.Op == "number.index" {
		// ToIntegerOrInfinity: saturating indices preserves clamping/bounds
		// semantics while preventing target i32 conversion overflow.
		b.line("IF " + a + " > 2147483647.")
		b.line(n + " = 2147483647.")
		b.line("ELSEIF " + a + " < -2147483648.")
		b.line(n + " = -2147483648.")
		b.line("ELSE.")
		b.line(n + " = trunc( " + a + " ).")
		b.line("ENDIF.")
		return
	}
	if x.Op == "number.toString" {
		// This phase supports decimal rendering of safe integers. Other
		// dynamic values trap rather than silently adopting ABAP formatting.
		limit := b.temp(hir.T(hir.Number))
		b.line(limit + " = '9007199254740991'.")
		b.line("IF " + a + " <> trunc( " + a + " ) OR " + a + " > " + limit + " OR " + a + " < 0 - " + limit + ".")
		b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
		b.line("ENDIF.")
		integer := b.temp(hir.T(hir.I64))
		b.line(integer + " = " + a + ".")
		b.line(n + " = |{ " + integer + " }|.")
		return
	}
	if x.X.Type.Kind == hir.String && strings.HasPrefix(x.Op, "string.") {
		if x.Op == "string.compareRegistryKey" || x.Op == "string.compareObjectName" {
			b.e.orderingSubsetRuntime()
			method := "rule_key"
			if x.Op == "string.compareObjectName" {
				method = "object_name"
			}
			b.line(n + " = " + b.e.name("runtime.orderingSubset") + "=>" + method + "( p0 = " + a + " p1 = " + args[0] + " ).")
			return
		}
		length := b.temp(hir.T(hir.I32))
		b.line(length + " = strlen( " + a + " ).")
		// ABAP strlen and sections count UTF-16 code units, like JavaScript.
		// Runtime implementations that count runes are a runtime gap, not a
		// reason to convert the entire receiver on every access.
		switch x.Op {
		case "string.length":
			b.line(n + " = " + length + ".")
		case "string.concat":
			b.line(n + " = |{ " + a + " }{ " + args[0] + " }|.")
		case "string.slice":
			lo, hi := args[0], args[1]
			for _, v := range []string{lo, hi} {
				b.line("IF " + v + " < 0.")
				b.line(v + " = " + length + " + " + v + ".")
				b.line("ENDIF.")
				b.line(v + " = COND i( WHEN " + v + " < 0 THEN 0 WHEN " + v + " > " + length + " THEN " + length + " ELSE " + v + " ).")
			}
			b.line("IF " + hi + " > " + lo + ".")
			b.line(length + " = " + hi + " - " + lo + ".")
			b.line(n + " = " + a + "+" + lo + "(" + length + ").")
			b.line("ELSE.")
			b.line("CLEAR " + n + ".")
			b.line("ENDIF.")
		case "string.repeatIndent":
			// The enclosing source certificate proves non-negative integer indentation.
			// The helper remains fail-closed if called without that domain proof.
			b.line("IF " + args[0] + " < 0 OR " + args[0] + " > 2147483647 OR " + args[0] + " <> trunc( " + args[0] + " ).")
			b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
			b.line("ENDIF.")
			index := b.temp(hir.T(hir.I32))
			b.line(index + " = 0.")
			b.line("CLEAR " + n + ".")
			b.line("WHILE " + index + " < " + args[0] + ".")
			b.line(n + " = |{ " + n + " }{ " + a + " }|.")
			b.line(index + " = " + index + " + 1.")
			b.line("ENDWHILE.")

		case "string.substring":
			// JavaScript substring clamps both indices into [0, length] and
			// swaps them when start is past end. Indices count UTF-16 units.
			lo, hi := args[0], args[1]
			for _, v := range []string{lo, hi} {
				b.line(v + " = COND i( WHEN " + v + " < 0 THEN 0 WHEN " + v + " > " + length + " THEN " + length + " ELSE " + v + " ).")
			}
			b.line("IF " + lo + " > " + hi + ".")
			b.line(length + " = " + lo + ".")
			b.line(lo + " = " + hi + ".")
			b.line(hi + " = " + length + ".")
			b.line("ENDIF.")
			b.line(length + " = " + hi + " - " + lo + ".")
			b.line(n + " = " + a + "+" + lo + "(" + length + ").")
		case "string.charAt":
			// charAt never raises: out of range is the empty string.
			b.line("IF " + args[0] + " < 0 OR " + args[0] + " >= " + length + ".")
			b.line("CLEAR " + n + ".")
			b.line("ELSE.")
			b.line(n + " = " + a + "+" + args[0] + "(1).")
			b.line("ENDIF.")
		case "string.charCodeAt":
			if x.X.Kind == hir.Lit && x.Args[0].Kind == hir.Lit {
				value, ok := b.literalString(x.X)
				if !ok {
					return
				}
				units := utf16.Encode([]rune(value))
				index, err := strconv.Atoi(fmt.Sprint(x.Args[0].Value))
				if err == nil && index >= 0 && index < len(units) {
					b.line(fmt.Sprintf("%s = %d.", n, units[index]))
					return
				}
			}
			// Out of range JavaScript returns NaN, which the Number domain
			// does not carry; raise instead of returning a wrong code unit
			// (a documented divergence, like Number division by zero).
			b.line("IF " + args[0] + " < 0 OR " + args[0] + " >= " + length + ".")
			b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
			b.line("ENDIF.")
			// The section is materialized first: string offsets are not
			// allowed as functional-method arguments.
			ch := b.temp(hir.T(hir.String))
			b.line(ch + " = " + a + "+" + args[0] + "(1).")
			b.codeUnit(n, ch)
		case "string.substr":
			// Legacy substr: a negative start counts from the end, a
			// non-positive length is the empty string, the result is cut at
			// the end of the receiver.
			start, count := args[0], args[1]
			b.line("IF " + start + " < 0.")
			b.line(start + " = " + start + " + " + length + ".")
			b.line("IF " + start + " < 0.")
			b.line(start + " = 0.")
			b.line("ENDIF.")
			b.line("ENDIF.")
			b.line("IF " + start + " > " + length + " OR " + count + " <= 0.")
			b.line("CLEAR " + n + ".")
			b.line("ELSE.")
			b.line("IF " + count + " > " + length + " - " + start + ".")
			b.line(count + " = " + length + " - " + start + ".")
			b.line("ENDIF.")
			b.line(n + " = " + a + "+" + start + "(" + count + ").")
			b.line("ENDIF.")
		case "string.trim":
			lo := b.temp(hir.T(hir.I32))
			hi := b.temp(hir.T(hir.I32))
			b.line(lo + " = 0.")
			b.line(hi + " = " + length + ".")
			b.trimLoop(a, lo, hi, true)
			b.trimLoop(a, lo, hi, false)
			b.line("IF " + hi + " > " + lo + ".")
			b.line(length + " = " + hi + " - " + lo + ".")
			b.line(n + " = " + a + "+" + lo + "(" + length + ").")
			b.line("ELSE.")
			b.line("CLEAR " + n + ".")
			b.line("ENDIF.")
		case "string.toUpperCase":
			b.line(n + " = " + a + ".")
			for _, mapping := range fullUpperMappings {
				b.line("REPLACE ALL OCCURRENCES OF `" + mapping[0] + "` IN " + n + " WITH `" + mapping[1] + "`.")
			}
			b.line("TRANSLATE " + n + " TO UPPER CASE.")
		case "string.toLowerCase":
			b.line(n + " = " + a + ".")
			b.line("TRANSLATE " + n + " TO LOWER CASE.")
		case "string.startsWith", "string.endsWith":
			count := b.temp(hir.T(hir.I32))
			offset := b.temp(hir.T(hir.I32))
			section := b.temp(hir.T(hir.String))
			b.line(count + " = strlen( " + args[0] + " ).")
			b.line("IF " + count + " = 0.")
			b.line(n + " = abap_true.")
			b.line("ELSEIF " + count + " <= " + length + ".")
			b.line(offset + " = 0.")
			if x.Op == "string.endsWith" {
				b.line(offset + " = " + length + " - " + count + ".")
			}
			b.line(section + " = " + a + "+" + offset + "(" + count + ").")
			b.line("IF " + section + " = " + args[0] + ".")
			b.line(n + " = abap_true.")
			b.line("ENDIF.")
			b.line("ENDIF.")
		case "string.indexOf":
			b.line("IF " + args[0] + " IS INITIAL.")
			b.line(n + " = 0.")
			b.line("ELSE.")
			b.line("FIND " + args[0] + " IN " + a + " MATCH OFFSET " + n + ".")
			b.line("IF sy-subrc <> 0.")
			b.line(n + " = -1.")
			b.line("ENDIF.")
			b.line("ENDIF.")
		case "string.split":
			b.stringSplit(n, a, args[0], length)
		case "string.replaceRegex":
			b.line("CALL METHOD " + args[0] + "->replace EXPORTING p0 = " + a + " p1 = " + args[1] + " RECEIVING result = " + n + ".")
		case "string.replaceAll":
			b.line(n + " = " + a + ".")
			b.line("REPLACE ALL OCCURRENCES OF " + args[0] + " IN " + n + " WITH " + args[1] + ".")
		case "string.replaceFirst":
			// JavaScript replace with a string pattern: the first occurrence
			// only; an empty needle inserts before the first code unit.
			b.line(n + " = " + a + ".")
			b.line("IF " + args[0] + " = ``.")
			b.line(n + " = |{ " + args[1] + " }{ " + a + " }|.")
			b.line("ELSE.")
			b.line("REPLACE FIRST OCCURRENCE OF " + args[0] + " IN " + n + " WITH " + args[1] + ".")
			b.line("ENDIF.")
		case "string.at":
			// s[i]: absent outside [0, length), otherwise one UTF-16 unit.
			b.line("IF " + args[0] + " < 0 OR " + args[0] + " >= " + length + ".")
			b.line("CLEAR " + n + ".")
			b.line("ELSE.")
			b.line(n + " = NEW #( ).")
			b.line(n + "->has = abap_true.")
			b.line(n + "->value = " + a + "+" + args[0] + "(1).")
			b.line("ENDIF.")
		case "string.parseInt10":
			b.parseInt10(n, a, length)
		case "string.localeCompareNames":
			b.localeCompareNames(n, a, args[0], length)
		}
		return
	}
	if x.Op == "i32.toString" {
		// A string template renders an i exactly like JavaScript String(int32).
		b.line(n + " = |{ " + a + " }|.")
		return
	}
	switch x.Op {
	case "dynamic.materialize":
		b.line(n + " = " + b.e.materializer(x.Type) + "=>project( " + a + " ).")
		return
	case "json.parseSubset":
		b.e.jsonSubsetRuntime()
		b.line(n + " = " + b.e.name("runtime.jsonSubset") + "=>parse( " + a + " ).")
		return
	case "xml.parseSubset":
		b.e.xmlSubsetRuntime()
		b.line(n + " = " + b.e.name("runtime.xmlSubset") + "=>parse( " + a + " ).")
		return
	case "dynamic.isNullish":
		b.line("IF " + a + " IS NOT BOUND.")
		b.line(n + " = abap_true.")
		b.line("ELSE.")
		b.line(n + " = xsdbool( " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_null ).")
		b.line("ENDIF.")
		return
	case "dynamic.null":
		b.line("CREATE OBJECT " + n + ".")
		b.line(n + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_null.")
		return
	case "dynamic.get", "dynamic.put", "dynamic.strictEquals", "dynamic.asBoolean":
		if x.Op == "dynamic.strictEquals" {
			b.line("IF " + a + " IS NOT BOUND.")
			b.line(n + " = xsdbool( " + args[0] + " IS NOT BOUND ).")
			b.line("ELSE.")
			b.line(n + " = " + a + "->strict_equals( " + args[0] + " ).")
			b.line("ENDIF.")
			return
		}
		method := map[string]string{"dynamic.get": "get", "dynamic.put": "put", "dynamic.asBoolean": "as_boolean"}[x.Op]
		params := []string{}
		for i, arg := range args {
			params = append(params, fmt.Sprintf("p%d = %s", i, arg))
		}
		if x.Type.Kind == hir.Void {
			b.line("CALL METHOD " + a + "->" + method + " EXPORTING " + strings.Join(params, " ") + ".")
		} else {
			call := "CALL METHOD " + a + "->" + method
			if len(params) > 0 {
				call += " EXPORTING " + strings.Join(params, " ")
			}
			b.line(call + " RECEIVING result = " + n + ".")
		}
		return
	case "clock.telemetry":
		b.e.telemetryRuntime()
		b.line(n + " = " + b.e.name("runtime.telemetry") + "=>now( ).")
		return
	case "object.classOf":
		b.e.descriptors = true
		if x.X.Type.Kind == hir.Optional {
			b.line("IF " + a + " IS BOUND.")
			b.line(n + " = " + a + "->" + b.e.name("builtin.classOf") + "( ).")
			b.line("ENDIF.")
			return
		}
		if x.X.Type.Kind == hir.ClassRef && x.X.Type.Name == hir.RootObject {
			b.e.err = fmt.Errorf("node %d (%s): classOf on the object root is not emitted", x.ID, x.Source)
			return
		}
		b.line(n + " = " + a + "->" + b.e.name("builtin.classOf") + "( ).")
		return
	case "classvalue.name":
		b.line(n + " = " + a + "->name.")
		return
	case "classvalue.has":
		b.line("CALL METHOD " + a + "->has EXPORTING p0 = " + args[0] + " RECEIVING result = " + n + ".")
		return
	case "classvalue.new":
		b.e.descriptors = true
		obj := b.rawTemp("REF TO object")
		b.line("CALL METHOD " + b.e.name("runtime.classvalue.factory") + "=>new EXPORTING p0 = " + a + " RECEIVING result = " + obj + ".")
		b.line(n + " ?= " + obj + ".")
		return
	case "dynamic.of":
		if x.X.Kind == hir.StaticGet && strings.HasSuffix(x.X.Name, "_namespace") && x.X.Type.Kind == hir.OrderedMap && x.X.Type.Args[0].Kind == hir.String {
			// An enum namespace is immutable: boxing it as a plain object (a
			// snapshot of its entries) reads like the JavaScript object does.
			dyn := b.e.name("runtime.dynamic")
			b.serial++
			row := fmt.Sprintf("t%d", b.serial)
			v := b.temp(hir.T(hir.Dynamic))
			b.line("CREATE OBJECT " + n + ".")
			b.line(n + "->tag = " + dyn + "=>tag_object.")
			b.line("LOOP AT " + a + "->entries INTO DATA(" + row + ").")
			b.boxDynamic(v, row+"-v", x.X.Type.Args[1])
			b.line("APPEND VALUE #( k = " + row + "-k v = " + v + " ) TO " + n + "->entries.")
			b.line("ENDLOOP.")
			return
		}
		b.boxDynamic(n, a, x.X.Type)
		return
	case "dynamic.typeof", "dynamic.toString":
		b.line("IF " + a + " IS BOUND.")
		method := "type_of"
		if x.Op == "dynamic.toString" {
			method = "to_string"
		}
		b.line(n + " = " + a + "->" + method + "( ).")
		b.line("ELSE.")
		b.line(n + " = `undefined`.")
		b.line("ENDIF.")
		return
	case "dynamic.isArray":
		// An unbound box is undefined, which is not an array.
		b.line(n + " = abap_false.")
		b.line("IF " + a + " IS BOUND.")
		b.e.isArrayUsed = true
		b.line("IF " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_array.")
		b.line(n + " = abap_true.")
		b.line("ELSEIF " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_ref AND " + a + "->oval IS BOUND.")
		mark := b.rawTemp("REF TO " + b.e.name("runtime.arraymark"))
		b.line("TRY.")
		b.line(mark + " ?= " + a + "->oval.")
		b.line(n + " = abap_true.")
		b.line("CATCH cx_sy_move_cast_error.")
		b.line("ENDTRY.")
		b.line("ENDIF.")
		b.line("ENDIF.")
		return
	case "dynamic.isNumber", "dynamic.asNumber", "dynamic.isString", "dynamic.isFunction", "dynamic.asString", "dynamic.asClassValue", "dynamic.asRef":
		// The box methods use snake_case ABAP names.
		op := map[string]string{"dynamic.isNumber": "is_number", "dynamic.asNumber": "as_number", "dynamic.isString": "is_string", "dynamic.isFunction": "is_function", "dynamic.asString": "as_string", "dynamic.asClassValue": "as_classvalue", "dynamic.asRef": "as_ref"}[x.Op]
		target := n
		if x.Op == "dynamic.asRef" {
			target = b.temp(hir.Ref(hir.RootObject))
		}
		b.line("CALL METHOD " + a + "->" + op + " RECEIVING result = " + target + ".")
		if target != n {
			b.line(n + " ?= " + target + ".")
		}
		return
	case "regexp.source":
		b.line(n + " = " + a + "->source.")
		return
	case "regexp.toString":
		// JavaScript prints the flags in canonical order: g before i.
		b.line(n + " = |/{ " + a + "->source }/|.")
		b.line("IF " + a + "->global = abap_true.")
		b.line(n + " = " + n + " && `g`.")
		b.line("ENDIF.")
		b.line("IF " + a + "->ignore_case = abap_true.")
		b.line(n + " = " + n + " && `i`.")
		b.line("ENDIF.")
		return
	}
	if x.Op == "record.delete" {
		b.line("DELETE " + a + "->entries WHERE k = " + args[0] + ".")
		b.line(n + " = abap_true.")
		return
	}
	op := strings.Split(x.Op, ".")[1]
	params := []string{}
	for i, arg := range args {
		params = append(params, fmt.Sprintf("p%d = %s", i, arg))
	}
	target := n
	if (x.Op == "array.get" || x.Op == "array.pop" || x.Op == "array.shift") && (x.X.Type.Args[0].IsRef() || (x.X.Type.Args[0].Kind == hir.Optional && x.X.Type.Args[0].Args[0].IsRef())) {
		target = b.temp(hir.Ref(hir.RootObject))
	}
	s := a + "->" + op + "( " + strings.Join(params, " ") + " )"
	if target != "" {
		s = target + " = " + s
	}
	b.line(s + ".")
	if target != n {
		b.line(n + " ?= " + target + ".")
	}
}
func (b *body) stmt(s *hir.Stmt) {
	if s == nil {
		return
	}
	if castTrace && s.Node.Source != "" {
		b.code.WriteString("*@src " + s.Node.Source + "\n")
		b.lastInit = ""
	}
	e := b.e
	switch s.Kind {
	case hir.Block:
		old := b.locals
		b.locals = map[string]string{}
		for k, v := range old {
			b.locals[k] = v
		}
		for _, x := range s.List {
			b.stmt(x)
		}
		b.locals = old
	case hir.VarDecl:
		n := b.temp(s.Type)
		if s.X != nil {
			b.line(n + " = " + b.value(s.X, s.Type) + ".")
		}
		b.locals[s.Name] = n
	case hir.Assign:
		target := ""
		switch s.X.Kind {
		case hir.Local:
			target = b.locals[s.X.Name]
		case hir.FieldGet:
			target = b.expr(s.X.X) + "->" + e.member(s.X.Name)
		case hir.StaticGet:
			b.initialize(s.X.Owner)
			target = e.name(s.X.Owner) + "=>" + e.member(s.X.Name)
		case hir.IndexGet:
			a, i := b.expr(s.X.X), b.expr(s.X.Y)
			v := b.value(s.Y, s.X.Type)
			b.line(i + " = " + i + " + 1.")
			row := b.temp(arrayStorage(s.X.X.Type).Args[0])
			b.line("CLEAR " + row + ".")
			b.line("WHILE lines( " + a + "->items ) < " + i + ".")
			b.line("APPEND " + row + " TO " + a + "->items.")
			b.line("ENDWHILE.")
			b.line("MODIFY " + a + "->items FROM " + v + " INDEX " + i + ".")
			return
		}
		b.line(target + " = " + b.value(s.Y, s.X.Type) + ".")
	case hir.ExprStmt:
		b.expr(s.X)
	case hir.If:
		a := b.expr(s.X)
		b.line("IF " + a + " = abap_true.")
		b.stmt(s.Body)
		if s.Else != nil {
			b.line("ELSE.")
			b.stmt(s.Else)
		}
		b.line("ENDIF.")
	case hir.While:
		b.line("DO.")
		a := b.expr(s.X)
		b.line("IF " + a + " = abap_false.")
		b.line("EXIT.")
		b.line("ENDIF.")
		b.stmt(s.Body)
		b.line("ENDDO.")
	case hir.ForEach:
		a := b.expr(s.X)
		n := b.temp(s.Type)
		old, ok := b.locals[s.Name]
		b.locals[s.Name] = n
		row := n
		if s.Type.IsRef() {
			row = b.temp(hir.Ref(hir.RootObject))
		}
		b.line("LOOP AT " + a + "->items INTO " + row + ".")
		if row != n {
			b.line(n + " ?= " + row + ".")
		}
		b.stmt(s.Body)
		b.line("ENDLOOP.")
		if ok {
			b.locals[s.Name] = old
		} else {
			delete(b.locals, s.Name)
		}
	case hir.Break:
		b.line("EXIT.")
	case hir.Continue:
		b.line("CONTINUE.")
	case hir.Return:
		if s.X != nil && (b.m.Result.Kind == hir.Void || s.X.Type.Kind == hir.Void) {
			b.expr(s.X)
		} else if s.X != nil {
			b.line("result = " + b.value(s.X, b.m.Result) + ".")
		}
		b.line("RETURN.")
	case hir.Trap:
		name := e.name("exception.unexecuted")
		e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nDATA source_location TYPE string.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nENDCLASS.\n"
		n := b.rawTemp("REF TO " + name)
		b.line(n + " = NEW #( ).")
		b.line(n + "->source_location = " + b.expr(hir.L(hir.T(hir.String), s.Name)) + ".")
		b.line("RAISE EXCEPTION " + n + ".")
	case hir.Throw:
		name := e.exception(s.X.Type)
		n := b.rawTemp("REF TO " + name)
		v := b.expr(s.X)
		b.line(n + " = NEW #( ).")
		b.line(n + "->payload = " + v + ".")
		b.line("RAISE EXCEPTION " + n + ".")
	case hir.Finally:
		// Normal completion runs the finally block after the TRY; anything
		// leaving the body is caught, the block runs, and the same exception
		// object is raised again.
		n := b.rawTemp("REF TO cx_root")
		b.line("TRY.")
		b.stmt(s.Body)
		b.line("CATCH cx_root INTO " + n + ".")
		b.stmt(s.Else)
		b.line("RAISE EXCEPTION " + n + ".")
		b.line("ENDTRY.")
		b.stmt(s.Else)
	case hir.Try:
		name := e.exception(s.Type)
		n := b.rawTemp("REF TO " + name)
		b.line("TRY.")
		b.stmt(s.Body)
		b.line("CATCH " + name + " INTO " + n + ".")
		v := b.temp(s.Type)
		b.line(v + " = " + n + "->payload.")
		old, ok := b.locals[s.Name]
		b.locals[s.Name] = v
		b.stmt(s.Else)
		if ok {
			b.locals[s.Name] = old
		} else {
			delete(b.locals, s.Name)
		}
		b.line("ENDTRY.")
	}
}

// wrap only splits outside literal tokens. Long strings are already chunked.
func wrap(src string) (string, error) {
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(src, "\n"), "\n") {
		for len(line) > 255 {
			quote := byte(0)
			cut := -1
			for i := 0; i < len(line) && i <= 240; i++ {
				c := line[i]
				if quote != 0 {
					if quote == '|' && c == '\\' {
						i++
					} else if c == quote {
						if quote != '|' && i+1 < len(line) && line[i+1] == quote {
							i++
						} else {
							quote = 0
						}
					}
				} else if c == '`' || c == '\'' || c == '|' {
					quote = c
				} else if c == ' ' {
					cut = i
				}
			}
			if cut <= 0 {
				return "", fmt.Errorf("unbreakable source line (%s bytes)", strconv.Itoa(len(line)))
			}
			out.WriteString(line[:cut] + "\n")
			line = strings.TrimLeft(line[cut:], " ")
		}
		out.WriteString(line + "\n")
	}
	return out.String(), nil
}

// boxDynamic preserves identity and absence instead of boxing the ABI wrapper
// itself. Objects already represented as graph nodes flow through unchanged.
func (b *body) boxDynamic(n, a string, t hir.Type) {
	if t.Kind == hir.Dynamic {
		b.line(n + " = " + a + ".")
		return
	}
	if t.Kind == hir.Optional {
		test := a + " IS BOUND"
		base := t.Args[0]
		if !base.IsRef() {
			test += " AND " + a + "->has = abap_true"
		}
		b.line("IF " + test + ".")
		if !base.IsRef() {
			a += "->value"
		}
		b.boxDynamic(n, a, base)
		b.line("ENDIF.")
		return
	}
	if t.IsRef() {
		b.line("IF " + a + " IS BOUND.")
	}
	tag := "tag_ref"
	field := "oval"
	switch t.Kind {
	case hir.String:
		tag, field = "tag_string", "sval"
	case hir.Number, hir.I32:
		tag, field = "tag_number", "nval"
	case hir.Bool:
		tag, field = "tag_boolean", "bval"
	case hir.ClassValue:
		tag, field = "tag_class", "cval"
	default:
		if !t.IsRef() {
			b.e.err = fmt.Errorf("dynamic boxing of %s is not supported", t)
			return
		}
	}
	b.line("CREATE OBJECT " + n + ".")
	b.line(n + "->tag = " + b.e.name("runtime.dynamic") + "=>" + tag + ".")
	if field == "oval" {
		converted := b.rawTemp("REF TO object")
		b.line(converted + " = " + a + ".")
		a = converted
	}
	b.line(n + "->" + field + " = " + a + ".")
	if t.IsRef() {
		b.line("ENDIF.")
	}
}

// overriddenBelow reports whether class or any of its subclasses declares
// method name itself.
func (e *emitter) overriddenBelow(class, name string) bool {
	for _, c := range e.p.Classes {
		for k := c; k != nil; k = e.classBy(k.Super) {
			if k.Name == class {
				for _, m := range c.Methods {
					if m.Name == name || strings.HasPrefix(m.Name, name+"_instantiated_") {
						return true
					}
				}
				break
			}
			if k.Super == "" {
				break
			}
		}
	}
	return false
}

// wideShape: a class whose constructor would exceed the kernel's statement
// length (hundreds of parameters, e.g. the all-rules config shape) and whose
// constructor body only copies each parameter into the same-named field.
// Such a class is emitted without a constructor; NEW sets the fields.
func (e *emitter) wideShape(c *hir.Class) bool {
	if c.Ctor == nil || len(c.Ctor.Params) <= 40 || c.Super != "" || c.Ctor.Body == nil {
		return false
	}
	if len(c.Ctor.Body.List) != len(c.Ctor.Params) {
		return false
	}
	for i, s := range c.Ctor.Body.List {
		p := c.Ctor.Params[i]
		y := s.Y
		if y != nil && y.Kind == hir.Narrow {
			y = y.X
		}
		if s.Kind != hir.Assign || s.X == nil || y == nil || s.X.Kind != hir.FieldGet || s.X.Name != p.Name || y.Kind != hir.Local || y.Name != p.Name {
			return false
		}
	}
	return true
}
