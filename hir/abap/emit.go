// Package abap lowers verified object HIR to ABAP 7.02 (plus int8).
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
	p     *hir.Program
	names *hir.Names
	files map[string]string
	types map[string]bool
	err   error
}

// Emit returns one source per global declaration, including all runtime dependencies.
func Emit(p *hir.Program) (map[string]string, error) {
	if errors := hir.Verify(p); len(errors) > 0 {
		return nil, errors[0]
	}
	e := &emitter{p: p, names: hir.NewNames(), files: map[string]string{}, types: map[string]bool{}}
	for _, i := range p.Interfaces {
		var b strings.Builder
		fmt.Fprintf(&b, "INTERFACE %s PUBLIC.\n", e.name(i.Name))
		for _, m := range i.Methods {
			b.WriteString(e.signature(m, false))
		}
		b.WriteString("ENDINTERFACE.\n")
		e.files[e.name(i.Name)+".intf.abap"] = b.String()
	}
	for _, c := range p.Classes {
		e.class(c)
	}
	for file, src := range e.files {
		out, err := wrap(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		e.files[file] = out
	}
	if e.err != nil {
		return nil, e.err
	}
	return e.files, nil
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
func (e *emitter) typ(t hir.Type) string {
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
	case hir.ClassRef, hir.InterfaceRef:
		return "REF TO " + e.name(t.Name)
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
		}
	}
	if m.Result.Kind != hir.Void {
		s += " RETURNING VALUE(result) TYPE " + e.typ(m.Result)
	}
	return s + ".\n"
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
	if c.Ctor != nil {
		b.WriteString(e.signature(c.Ctor, true))
	}
	for _, m := range c.Methods {
		base, _ := e.method(e.classBy(c.Super), m.Name)
		if base != nil {
			fmt.Fprintf(&b, "METHODS %s REDEFINITION.\n", e.member(m.Name))
		} else {
			b.WriteString(e.signature(m, false))
		}
	}
	b.WriteString("ENDCLASS.\nCLASS " + e.name(c.Name) + " IMPLEMENTATION.\n")
	if c.Ctor != nil {
		b.WriteString(e.body(c, c.Ctor, "constructor"))
	}
	for _, m := range c.Methods {
		if !m.Abstract {
			b.WriteString(e.body(c, m, e.member(m.Name)))
		}
	}
	for _, n := range c.Implements {
		for _, i := range e.p.Interfaces {
			if i.Name != n {
				continue
			}
			for _, m := range i.Methods {
				fmt.Fprintf(&b, "METHOD %s~%s.\n", e.name(n), e.member(m.Name))
				s := "CALL METHOD me->" + e.member(m.Name)
				if len(m.Params) > 0 {
					s += " EXPORTING"
					for _, p := range m.Params {
						s += " " + e.param(p.Name) + " = " + e.param(p.Name)
					}
				}
				if m.Result.Kind != hir.Void {
					s += " RECEIVING result = result"
				}
				b.WriteString(s + ".\nENDMETHOD.\n")
			}
		}
	}
	b.WriteString("ENDCLASS.\n")
	e.files[e.name(c.Name)+".clas.abap"] = b.String()
}

type body struct {
	e          *emitter
	c          *hir.Class
	m          *hir.Method
	decl, code strings.Builder
	locals     map[string]string
	serial     int
}

func (e *emitter) body(c *hir.Class, m *hir.Method, name string) string {
	b := &body{e: e, c: c, m: m, locals: map[string]string{}}
	for _, p := range m.Params {
		n := b.temp(p.Type)
		b.locals[p.Name] = n
		b.line(n + " = " + e.param(p.Name) + ".")
	}
	b.stmt(m.Body)
	return "METHOD " + name + ".\n" + b.decl.String() + b.code.String() + "ENDMETHOD.\n"
}
func (b *body) line(s string) { b.code.WriteString(s + "\n") }
func (b *body) temp(t hir.Type) string {
	b.serial++
	n := fmt.Sprintf("t%d", b.serial)
	fmt.Fprintf(&b.decl, "DATA %s TYPE %s.\n", n, b.e.typ(t))
	return n
}
func (b *body) rawTemp(typ string) string {
	b.serial++
	n := fmt.Sprintf("t%d", b.serial)
	fmt.Fprintf(&b.decl, "DATA %s TYPE %s.\n", n, typ)
	return n
}
func (b *body) convert(value string, src, dst hir.Type) string {
	if dst.Kind == hir.Optional && src.Kind != hir.Optional && !dst.Args[0].IsRef() {
		n := b.temp(dst)
		b.line("CREATE OBJECT " + n + ".")
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
		b.call(x, "")
		return ""
	}
	n := b.temp(t)
	b.line("CLEAR " + n + ".")
	switch x.Kind {
	case hir.Lit:
		if x.Value == nil {
			b.line("CLEAR " + n + ".")
			break
		}
		lt := t
		target := n
		if t.Kind == hir.Optional {
			lt = t.Args[0]
			b.line("CREATE OBJECT " + n + ".")
			b.line(n + "->has = abap_true.")
			target = n + "->value"
		}
		switch lt.Kind {
		case hir.String:
			b.stringLit(target, x.Value.(string))
		case hir.Bool:
			s := "abap_false"
			if x.Value.(bool) {
				s = "abap_true"
			}
			b.line(target + " = " + s + ".")
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
		b.line(n + " = " + e.name(x.Owner) + "=>" + e.member(x.Name) + ".")
	case hir.IndexGet:
		a, i := b.expr(x.X), b.expr(x.Y)
		b.line(i + " = " + i + " + 1.")
		b.line("READ TABLE " + a + "->items INDEX " + i + " INTO " + n + ".")
	case hir.New:
		s := "CREATE OBJECT " + n
		args := []string{}
		ctor := e.p.Constructor(t.Name)
		for i, a := range x.Args {
			args = append(args, e.param(ctor.Params[i].Name)+" = "+b.value(a, ctor.Params[i].Type))
		}
		if len(args) > 0 {
			s += " EXPORTING " + strings.Join(args, " ")
		}
		b.line(s + ".")
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
		op := map[string]string{"==": "=", "!=": "<>", "%": "MOD"}[x.Op]
		if op == "" {
			op = x.Op
		}
		if t.Kind == hir.Bool {
			b.line("IF " + a + " " + op + " " + z + ".")
			b.line(n + " = abap_true.")
			b.line("ENDIF.")
		} else {
			b.line(n + " = " + a + " " + op + " " + z + ".")
		}
	case hir.Unary:
		a := b.expr(x.X)
		if x.Op == "!" {
			b.line("IF " + a + " = abap_false.")
			b.line(n + " = abap_true.")
			b.line("ENDIF.")
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
		// A per-class helper uses a checked cast only to test ancestry. It never
		// exposes the narrowed reference and works on 7.02 without IS INSTANCE OF.
		helper := e.instanceHelper(x.Owner)
		b.line(n + " = " + helper + "=>test( " + b.expr(x.X) + " ).")
	case hir.IsUndefined:
		a := b.expr(x.X)
		b.line("IF " + a + " IS INITIAL.")
		b.line(n + " = abap_true.")
		if !x.X.Type.Args[0].IsRef() {
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
func (b *body) truth(n, a string, t hir.Type) {
	test := a + " IS NOT INITIAL"
	if t.Kind == hir.String {
		test = "strlen( " + a + " ) > 0"
	}
	if t.IsRef() || t.Kind == hir.Optional {
		test = a + " IS BOUND"
	}
	b.line("IF " + test + ".")
	b.line(n + " = abap_true.")
	b.line("ENDIF.")
}
func (b *body) stringLit(n, s string) {
	// Backtick literals preserve trailing blanks. Chunk before quoting so source
	// tokens always fit the 255-byte envelope (also bounds UTF-16 source length).
	b.line("CLEAR " + n + ".")
	for s != "" {
		r := []rune(s)
		if r[0] < 32 || r[0] == 127 {
			char := b.rawTemp("c LENGTH 1")
			b.line(fmt.Sprintf("%s = cl_abap_conv_in_ce=>uccpi( %d ).", char, r[0]))
			b.line("CONCATENATE " + n + " " + char + " INTO " + n + " RESPECTING BLANKS.")
			s = string(r[1:])
			continue
		}
		k := 0
		size := 0
		for k < len(r) && r[k] >= 32 && r[k] != 127 && size+len(string(r[k]))*2 <= 120 {
			size += len(string(r[k])) * 2
			k++
		}
		chunk := string(r[:k])
		s = string(r[k:])
		b.line("CONCATENATE " + n + " `" + strings.ReplaceAll(chunk, "`", "``") + "` INTO " + n + " RESPECTING BLANKS.")
	}
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
	m, _ = e.method(e.classBy(owner), x.Name)
	member := e.member(x.Name)
	if x.Kind == hir.SuperCall && x.Name == "constructor" {
		m = e.p.Constructor(owner)
		member = "constructor"
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
	s := "CALL METHOD " + recv + member
	args := []string{}
	for i, a := range x.Args {
		args = append(args, e.param(m.Params[i].Name)+" = "+b.value(a, m.Params[i].Type))
	}
	if x.Kind == hir.SuperCall && n != "" {
		b.line(n + " = " + recv + member + "( " + strings.Join(args, " ") + " ).")
		return
	}
	if len(args) > 0 {
		s += " EXPORTING " + strings.Join(args, " ")
	}
	if n != "" {
		s += " RECEIVING result = " + n
	}
	b.line(s + ".")
}
func (b *body) runtimeOp(x *hir.Expr, n string) {
	a := b.expr(x.X)
	args := []string{}
	ps, _, _ := hir.RuntimeSignature(x.Op, x.X.Type)
	for i, v := range x.Args {
		args = append(args, b.value(v, ps[i]))
	}
	if x.X.Type.Kind == hir.String {
		switch x.Op {
		case "string.length":
			conv := b.rawTemp("REF TO cl_abap_conv_out_ce")
			bytes := b.rawTemp("xstring")
			b.line(conv + " = cl_abap_conv_out_ce=>create( encoding = '4103' ).")
			b.line("CALL METHOD " + conv + "->convert EXPORTING data = " + a + " IMPORTING buffer = " + bytes + ".")
			b.line(n + " = xstrlen( " + bytes + " ) / 2.")
		case "string.concat":
			b.line("CONCATENATE " + a + " " + args[0] + " INTO " + n + " RESPECTING BLANKS.")
		case "string.substring":
			if x.X.Kind != hir.Lit || len(utf16.Encode([]rune(x.X.Value.(string)))) != len([]rune(x.X.Value.(string))) {
				b.e.err = fmt.Errorf("node %d (%s): substring requires a proven BMP literal receiver until portable surrogate slicing is available", x.ID, x.Source)
				return
			}
			lo, hi := args[0], args[1]
			length := b.temp(hir.T(hir.I32))
			b.line(length + " = strlen( " + a + " ).")
			for _, v := range []string{lo, hi} {
				b.line("IF " + v + " < 0.")
				b.line(v + " = 0.")
				b.line("ELSEIF " + v + " > " + length + ".")
				b.line(v + " = " + length + ".")
				b.line("ENDIF.")
			}
			b.line("IF " + lo + " > " + hi + ".")
			b.line(length + " = " + lo + ".")
			b.line(lo + " = " + hi + ".")
			b.line(hi + " = " + length + ".")
			b.line("ENDIF.")
			b.line(length + " = " + hi + " - " + lo + ".")
			b.line(n + " = " + a + "+" + lo + "(" + length + ").")
		case "string.charCodeAt":
			if x.X.Kind == hir.Lit && x.Args[0].Kind == hir.Lit {
				units := utf16.Encode([]rune(x.X.Value.(string)))
				index, err := strconv.Atoi(fmt.Sprint(x.Args[0].Value))
				if err == nil && index >= 0 && index < len(units) {
					b.line(fmt.Sprintf("%s = %d.", n, units[index]))
					return
				}
			}
			b.e.err = fmt.Errorf("node %d (%s): string.charCodeAt requires a proven in-range constant index and string (out-of-range is NaN)", x.ID, x.Source)
		}
		return
	}
	op := strings.Split(x.Op, ".")[1]
	s := "CALL METHOD " + a + "->" + op
	if len(args) > 0 {
		s += " EXPORTING"
		for i, arg := range args {
			s += fmt.Sprintf(" p%d = %s", i, arg)
		}
	}
	if n != "" {
		s += " RECEIVING result = " + n
	}
	b.line(s + ".")
}
func (b *body) stmt(s *hir.Stmt) {
	if s == nil {
		return
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
		b.line("CLEAR " + n + ".")
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
			target = e.name(s.X.Owner) + "=>" + e.member(s.X.Name)
		case hir.IndexGet:
			a, i := b.expr(s.X.X), b.expr(s.X.Y)
			v := b.value(s.Y, s.X.Type)
			b.line(i + " = " + i + " + 1.")
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
		b.line("LOOP AT " + a + "->items INTO " + n + ".")
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
		if s.X != nil && b.m.Result.Kind == hir.Void {
			b.expr(s.X)
		} else if s.X != nil {
			b.line("result = " + b.value(s.X, b.m.Result) + ".")
		}
		b.line("RETURN.")
	case hir.Throw:
		name := e.exception(s.X.Type)
		n := b.rawTemp("REF TO " + name)
		v := b.expr(s.X)
		b.line("CREATE OBJECT " + n + ".")
		b.line(n + "->payload = " + v + ".")
		b.line("RAISE EXCEPTION " + n + ".")
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
					if c == quote {
						if i+1 < len(line) && line[i+1] == quote {
							i++
						} else {
							quote = 0
						}
					}
				} else if c == '`' || c == '\'' {
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
