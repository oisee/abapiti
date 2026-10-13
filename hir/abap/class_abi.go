package abap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Class/constructor ABI refusals are native traps, never fabricated values.
func (e *emitter) classRefusal() string {
	name := e.name("class.refusal")
	e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nENDCLASS.\n"
	return name
}

func (b *body) freshClassDescriptor(x *hir.Expr, n string) {
	e := b.e
	c := e.classBy(x.Owner)
	// Captured lexical state, inherited layouts and argument-bearing local
	// constructors need a per-evaluation prototype ABI, not a singleton alias.
	if c == nil || !c.Scoped || c.Abstract || c.Super != "" || (c.Ctor != nil && len(c.Ctor.Params) != 0) {
		e.err = fmt.Errorf("node %d: fresh lexical constructor descriptor requires a concrete scoped zero-argument class without ancestry", x.ID)
		return
	}
	e.descriptors = true
	parent := b.temp(hir.T(hir.ClassValue))
	statics := b.temp(hir.T(hir.Array, hir.T(hir.String)))
	b.line("CREATE OBJECT " + statics + ".")
	b.line(n + " = NEW " + e.name("runtime.classvalue") + "( name = `" + strings.ReplaceAll(tsName(c.Name), "`", "``") + "` emitted = `" + e.name(c.Name) + "` parent = " + parent + " static_names = " + statics + " ).")
	b.line(n + "->lexical = abap_true.")
}

func constructorArgumentSupported(t hir.Type) bool {
	switch t.Kind {
	case hir.String, hir.Bool, hir.Number, hir.Dynamic, hir.ClassValue, hir.ClassRef, hir.InterfaceRef:
		return true
	case hir.Optional:
		return constructorArgumentSupported(t.Args[0])
	}
	return false
}

// Matches the Go argument factory's supported storage domains. Missing slots
// are undefined; optional slots retain absence, and surplus slots are ignored
// only after evaluation at the call site. No conversion supplies a zero value.
func (b *body) constructorArgument(n, d string, t hir.Type) {
	if t.Kind == hir.Optional {
		b.line("IF " + d + " IS BOUND.")
		if t.Args[0].IsRef() {
			b.constructorArgument(n, d, t.Args[0])
		} else {
			b.line(n + " = NEW #( ).\n" + n + "->has = abap_true.")
			b.constructorArgument(n+"->value", d, t.Args[0])
		}
		b.line("ENDIF.")
		return
	}
	if t.Kind == hir.Dynamic {
		b.line(n + " = " + d + ".")
		return
	}
	b.line("IF " + d + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + b.e.classRefusal() + ".\nENDIF.")
	switch t.Kind {
	case hir.String:
		b.line(n + " = " + d + "->as_string( ).")
	case hir.Bool:
		b.line(n + " = " + d + "->as_boolean( ).")
	case hir.Number:
		b.line(n + " = " + d + "->as_number( ).")
	case hir.ClassValue:
		b.line(n + " = " + d + "->as_classvalue( ).")
	case hir.ClassRef, hir.InterfaceRef:
		v := b.rawTemp("REF TO object")
		b.line(v + " = " + d + "->as_ref( ).")
		b.line(n + " ?= " + v + ".")
	}
}

func (e *emitter) argumentFactoryABAP() ([]factoryCase, string, string) {
	var cases []factoryCase
	var defs, impl strings.Builder
	trap := e.classRefusal()
	serial := 0

	for index, c := range e.p.Classes {
		entry := factoryCase{name: strings.ToUpper(e.name(c.Name))}
		ctor := e.p.Constructor(c.Name)
		owner := c
		for owner != nil && owner.Ctor == nil {
			owner = e.classBy(owner.Super)
		}
		wide := owner != nil && e.wideShape(owner)
		ok := !c.Abstract && (ctor == nil || len(ctor.Params) <= 40 || wide)
		if ctor != nil {
			for _, p := range ctor.Params {
				ok = ok && !p.Variadic && constructorArgumentSupported(p.Type)
			}
		}
		if !ok {
			entry.code = "RAISE EXCEPTION TYPE " + trap + ".\n"
			cases = append(cases, entry)
			continue
		}
		if wide && len(ctor.Params) > 40 {
			code, declarations, methods := e.wideArgumentFactory(c, ctor, index)
			entry.code = code
			defs.WriteString(declarations)
			impl.WriteString(methods)
			cases = append(cases, entry)
			continue
		}
		// CASE entries can share a leaf method: retain distinct local names.
		b := &body{e: e, c: c, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, serial: serial}
		args := []string{}
		if ctor != nil {
			for i, p := range ctor.Params {
				d := b.temp(hir.T(hir.Dynamic))
				b.line("IF p1 IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
				// Reference arrays are erased to REF TO object; read the
				// entry erased and narrow it with a checked cast.
				raw := b.rawTemp("REF TO object")
				b.line(fmt.Sprintf("READ TABLE p1->items INDEX %d INTO %s.", i+1, raw))
				b.line(d + " ?= " + raw + ".")
				v := b.temp(p.Type)
				b.constructorArgument(v, d, p.Type)
				b.locals[p.Name] = v
				args = append(args, e.param(p.Name)+" = "+v)
			}
		}
		b.initialize(c.Name)
		call := "CREATE OBJECT result TYPE " + e.name(c.Name)
		if len(args) > 0 && !wide {
			call += " EXPORTING " + strings.Join(args, " ")
		}
		b.line(call + ".")
		if wide {
			// The normal ABAP NEW path elides a wide field-copy constructor.
			// Use that same ABI, including the constructor's checked narrowing.
			v := b.temp(hir.Ref(c.Name))
			b.line(v + " ?= result.")
			for _, s := range ctor.Body.List {
				if s.Y.Kind == hir.Narrow && s.Y.X.Type.Kind == hir.Optional && !s.X.Type.IsRef() {
					b.line("IF " + b.locals[s.Y.X.Name] + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
				}
				value := b.value(s.Y, s.X.Type)
				b.line(v + "->" + e.member(s.X.Name) + " = " + value + ".")
			}
		}
		if c.Scoped {
			v := b.temp(hir.Ref(c.Name))
			b.line("IF p0->lexical = abap_true.\n" + v + " ?= result.\n" + v + "->" + e.name("class.lexicalDescriptor") + " = p0.\nENDIF.")
		}
		serial = b.serial
		entry.code = b.constantDeclarations() + foldTempInits(b.code.String())
		cases = append(cases, entry)
	}
	return cases, defs.String(), impl.String()
}

// factoryCase keeps a constructor body intact: decoding, initialization,
// allocation and lexical attachment must execute in that order.
type factoryCase struct{ name, code string }

// splitFactory dispatches through a balanced tree of name comparisons. Leaves
// contain at most 64 cases / 800 source lines; internal methods have two calls.
// Unlike a hash bucket, the bound does not depend on the distribution of names.
func (e *emitter) splitFactory(prefix, params, actuals, fallback string, cases []factoryCase) (string, string, string) {
	sort.Slice(cases, func(i, j int) bool { return cases[i].name < cases[j].name })
	var defs, impl strings.Builder
	serial := 0
	var build func([]factoryCase) string
	build = func(entries []factoryCase) string {
		name := fmt.Sprintf("%s_%d", prefix, serial)
		serial++
		fmt.Fprintf(&defs, "CLASS-METHODS %s IMPORTING class_name TYPE string %s RETURNING VALUE(result) TYPE REF TO object.\n", name, params)
		lines := 0
		for _, entry := range entries {
			lines += strings.Count(entry.code, "\n") + 1
		}
		var code strings.Builder
		if len(entries) > 64 || (lines > 800 && len(entries) > 1) {
			mid := len(entries) / 2
			left, right := build(entries[:mid]), build(entries[mid:])
			fmt.Fprintf(&code, "IF class_name <= `%s`.\nresult = %s( class_name = class_name %s ).\nELSE.\nresult = %s( class_name = class_name %s ).\nENDIF.\n", entries[mid-1].name, left, actuals, right, actuals)
		} else {
			code.WriteString("CASE class_name.\n")
			for _, entry := range entries {
				fmt.Fprintf(&code, "WHEN `%s`.\n%s", entry.name, entry.code)
			}
			code.WriteString("WHEN OTHERS.\n" + fallback + "ENDCASE.\n")
		}
		fmt.Fprintf(&impl, "METHOD %s.\n%sENDMETHOD.\n", name, code.String())
		return name
	}
	root := build(cases)
	return defs.String(), impl.String(), "result = " + root + "( class_name = class_name " + actuals + " ).\n"
}

// Wide field-copy constructors have no parameter-count envelope. Keep decoded
// values in a call-local structure, and decode every slot before initialization
// or allocation. Write helpers run only after that allocation, in field order.
// Nothing is shared through CLASS-DATA, so nested/reentrant factories are safe.
func (e *emitter) wideArgumentFactory(c *hir.Class, ctor *hir.Method, index int) (string, string, string) {
	frameType := fmt.Sprintf("args_frame_%d", index)
	var defs, impl strings.Builder
	fmt.Fprintf(&defs, "TYPES: BEGIN OF %s,\n", frameType)
	locals := map[string]string{}
	for i, p := range ctor.Params {
		fmt.Fprintf(&defs, "a%d TYPE %s,\n", i, e.typ(p.Type))
		locals[p.Name] = fmt.Sprintf("frame-a%d", i)
	}
	fmt.Fprintf(&defs, "END OF %s.\n", frameType)
	b := &body{e: e, c: c, m: &hir.Method{Result: hir.T(hir.Void)}, locals: locals}
	frame := fmt.Sprintf("frame%d", index)
	b.line("DATA " + frame + " TYPE " + frameType + ".")
	const chunk = 24
	for start := 0; start < len(ctor.Params); start += chunk {
		name := fmt.Sprintf("args_decode_%d_%d", index, start/chunk)
		fmt.Fprintf(&defs, "CLASS-METHODS %s IMPORTING p1 TYPE %s CHANGING frame TYPE %s.\n", name, e.typ(hir.T(hir.Array, hir.T(hir.Dynamic))), frameType)
		part := &body{e: e, c: c, m: b.m, locals: locals}
		for i := start; i < len(ctor.Params) && i < start+chunk; i++ {
			p := ctor.Params[i]
			d := part.temp(hir.T(hir.Dynamic))
			part.line("IF p1 IS NOT BOUND.\nRAISE EXCEPTION TYPE " + e.classRefusal() + ".\nENDIF.")
			raw := part.rawTemp("REF TO object")
			part.line(fmt.Sprintf("READ TABLE p1->items INDEX %d INTO %s.", i+1, raw))
			part.line(d + " ?= " + raw + ".")
			part.constructorArgument(locals[p.Name], d, p.Type)
		}
		fmt.Fprintf(&impl, "METHOD %s.\n%sENDMETHOD.\n", name, part.constantDeclarations()+foldTempInits(part.code.String()))
		b.line(name + "( EXPORTING p1 = p1 CHANGING frame = " + frame + " ).")
	}
	b.initialize(c.Name)
	b.line("CREATE OBJECT result TYPE " + e.name(c.Name) + ".")
	instance := fmt.Sprintf("instance%d", index)
	b.line("DATA " + instance + " TYPE REF TO " + e.name(c.Name) + ".")
	b.line(instance + " ?= result.")
	for start := 0; start < len(ctor.Body.List); start += chunk {
		name := fmt.Sprintf("args_write_%d_%d", index, start/chunk)
		fmt.Fprintf(&defs, "CLASS-METHODS %s IMPORTING instance TYPE REF TO %s frame TYPE %s.\n", name, e.name(c.Name), frameType)
		part := &body{e: e, c: c, m: b.m, locals: locals}
		for i := start; i < len(ctor.Body.List) && i < start+chunk; i++ {
			s := ctor.Body.List[i]
			if s.Y.Kind == hir.Narrow && s.Y.X.Type.Kind == hir.Optional && !s.X.Type.IsRef() {
				part.line("IF " + locals[s.Y.X.Name] + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + e.classRefusal() + ".\nENDIF.")
			}
			value := part.value(s.Y, s.X.Type)
			part.line("instance->" + e.member(s.X.Name) + " = " + value + ".")
		}
		fmt.Fprintf(&impl, "METHOD %s.\n%sENDMETHOD.\n", name, part.constantDeclarations()+foldTempInits(part.code.String()))
		b.line(name + "( instance = " + instance + " frame = " + frame + " ).")
	}
	if c.Scoped {
		b.line("IF p0->lexical = abap_true.\n" + instance + "->" + e.name("class.lexicalDescriptor") + " = p0.\nENDIF.")
	}
	return b.constantDeclarations() + foldTempInits(b.code.String()), defs.String(), impl.String()
}
