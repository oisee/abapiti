package abap

import (
	"fmt"
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

func (e *emitter) argumentFactoryABAP() string {
	var dispatch strings.Builder
	trap := e.classRefusal()
	dispatch.WriteString("IF p0 IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.\nDATA class_name TYPE string.\nclass_name = p0->emitted.\nTRANSLATE class_name TO UPPER CASE.\n")
	if e.arrayViews {
		dispatch.WriteString("IF p1 IS BOUND.\np1->load( ).\nENDIF.\n")
	}
	dispatch.WriteString("CASE class_name.\n")
	serial := 0
	for _, c := range e.p.Classes {
		fmt.Fprintf(&dispatch, "WHEN `%s`.\n", strings.ToUpper(e.name(c.Name)))
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
			dispatch.WriteString("RAISE EXCEPTION TYPE " + trap + ".\n")
			continue
		}
		// Independent body scopes need unique temporary names within the
		// single ABAP dispatch method (CASE does not introduce local scope).
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
		dispatch.WriteString(b.constantDeclarations() + foldTempInits(b.code.String()))
		serial = b.serial
	}
	dispatch.WriteString("WHEN OTHERS.\nRAISE EXCEPTION TYPE " + trap + ".\nENDCASE.\n")
	return dispatch.String()
}
