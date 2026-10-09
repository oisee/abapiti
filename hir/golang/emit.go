// Package golang emits a deliberately small, standalone Go HIR prototype.
package golang

import (
	"fmt"
	"go/format"
	"go/token"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oisee/abapiti/hir"
)

// Emit emits one package named main. Add a main function to execute an entry point.
func Emit(p *hir.Program) (map[string]string, error) { return EmitPackage(p, "main") }

// EmitPackage selects the package name. Output contains only standard-library dependencies.
func EmitPackage(p *hir.Program, pkg string) (map[string]string, error) {
	if !token.IsIdentifier(pkg) || token.Lookup(pkg).IsKeyword() || pkg == "_" {
		return nil, fmt.Errorf("invalid Go package name %q", pkg)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		return nil, errs[0]
	}
	e := &emitter{p: p, names: hir.NewNames()}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			e.checkOps(m.Body)
		}
		if c.Ctor != nil {
			e.checkOps(c.Ctor.Body)
		}
	}
	if e.err != nil {
		return nil, e.err
	}
	e.line("package %s", pkg)
	interfaces := append([]*hir.Interface(nil), p.Interfaces...)
	sort.Slice(interfaces, func(i, j int) bool { return interfaces[i].Name < interfaces[j].Name })
	for _, in := range interfaces {
		e.line("type %s interface {", e.ref(in.Name))
		for _, m := range in.Methods {
			e.line("%s(%s)%s", e.member(m.Name), e.params(m, false), e.result(m.Result))
		}
		e.line("}")
	}
	classes := append([]*hir.Class(nil), p.Classes...)
	sort.Slice(classes, func(i, j int) bool { return classes[i].Name < classes[j].Name })
	for _, c := range classes {
		e.class(c)
	}
	if e.err != nil {
		return nil, e.err
	}
	files := map[string]string{"hir.go": e.code.String() + e.extra.String(), "runtime.go": strings.Replace(runtimeSource+unicodeUpperSource+jsonRuntimeSource+xmlRuntimeSource, "package main", "package "+pkg, 1)}
	for n, s := range files {
		b, err := format.Source([]byte(s))
		if err != nil {
			return nil, fmt.Errorf("format %s: %w", n, err)
		}
		files[n] = string(b)
	}
	return files, nil
}

type emitter struct {
	p              *hir.Program
	names          *hir.Names
	code           strings.Builder
	err            error
	extra          strings.Builder
	materializers  map[string]bool
	stringLiterals map[string]string
}

func (e *emitter) line(f string, a ...any) { fmt.Fprintf(&e.code, f+"\n", a...) }
func (e *emitter) unsupported(n hir.Node, s string) {
	if e.err == nil {
		e.err = hir.Error{Node: n, Message: "not supported in the Go prototype: " + s}
	}
}
func (e *emitter) name(s string) string    { return e.names.Get(s) }
func (e *emitter) ref(s string) string     { return e.name("ref." + s) }
func (e *emitter) obj(s string) string     { return e.name("struct." + s) }
func (e *emitter) member(s string) string  { return e.name("member." + s) }
func (e *emitter) getter(s string) string  { return e.name("base." + s) }
func (e *emitter) body(c, m string) string { return e.name("body." + c + "." + m) }
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
		if n == "constructor" && c.Ctor != nil {
			return c.Ctor, c
		}
	}
	return nil, nil
}
func (e *emitter) fieldOwner(c *hir.Class, n string) *hir.Class {
	for ; c != nil; c = e.classBy(c.Super) {
		for _, f := range c.Fields {
			if f.Name == n {
				return c
			}
		}
	}
	return nil
}
func arrayStorage(t hir.Type) hir.Type {
	if t.Kind == hir.Array && t.Args[0].IsRef() {
		return hir.T(hir.Array, hir.Ref(hir.RootObject))
	}
	return t
}
func (e *emitter) typ(t hir.Type) string {
	t = arrayStorage(t)
	switch t.Kind {
	case hir.Void:
		return ""
	case hir.Bool:
		return "bool"
	case hir.I32:
		return "int32"
	case hir.I64:
		return "int64"
	case hir.Number:
		return "float64"
	case hir.String:
		return "jsString"
	case hir.ClassValue:
		return "*classDescriptor"
	case hir.RegExp:
		return "*jsRegExp"
	case hir.Dynamic:
		return "*dynamic"
	case hir.ClassRef:
		if t.Name == hir.RootObject {
			return "any"
		}
		return e.ref(t.Name)
	case hir.InterfaceRef:
		return e.ref(t.Name)
	case hir.Optional:
		if t.Args[0].IsRef() {
			return e.typ(t.Args[0])
		}
		return "optional[" + e.typ(t.Args[0]) + "]"
	case hir.Array:
		return "*array[" + e.typ(t.Args[0]) + "]"
	case hir.OrderedMap:
		return "*orderedMap[" + e.typ(t.Args[0]) + "," + e.typ(t.Args[1]) + "]"
	case hir.OrderedSet:
		return "*orderedSet[" + e.typ(t.Args[0]) + "]"
	default:
		e.unsupported(hir.Node{}, "type "+t.String())
		return "any"
	}
}
func (e *emitter) result(t hir.Type) string {
	if t.Kind == hir.Void {
		return ""
	}
	return " " + e.typ(t)
}
func (e *emitter) params(m *hir.Method, named bool) string {
	a := []string{}
	for i, p := range m.Params {
		if p.Variadic {
			e.unsupported(m.Node, "variadic parameters")
		}
		s := e.typ(p.Type)
		if named {
			s = fmt.Sprintf("p%d %s", i, s)
		}
		a = append(a, s)
	}
	return strings.Join(a, ",")
}
func (e *emitter) effective(c *hir.Class) []string {
	seen := map[string]bool{}
	for ; c != nil; c = e.classBy(c.Super) {
		for _, m := range c.Methods {
			if !m.Static {
				seen[m.Name] = true
			}
		}
	}
	a := []string{}
	for n := range seen {
		a = append(a, n)
	}
	sort.Strings(a)
	return a
}
func (e *emitter) class(c *hir.Class) {
	e.descriptor(c)
	e.line("type %s struct {", e.obj(c.Name))
	e.line("source *dynamic")
	if c.Super != "" {
		e.line("%s", e.obj(c.Super))
	} else {
		e.line("identity byte")
	}
	for _, f := range c.Fields {
		if !f.Static {
			e.line("%s %s", e.member(f.Name), e.typ(f.Type))
		}
	}
	e.line("}")
	e.line("func (self *%s) nilReference() bool {return self==nil}", e.obj(c.Name))
	e.line("func (self *%s) dynamicSource() *dynamic {return self.source}", e.obj(c.Name))
	e.line("func (self *%s) %s() *%s {return self}", e.obj(c.Name), e.getter(c.Name), e.obj(c.Name))
	if e.concreteClass(c.Name) {
		// A class with no subclasses has one possible concrete representation.
		e.line("type %s = *%s", e.ref(c.Name), e.obj(c.Name))
	} else {
		e.line("type %s interface {", e.ref(c.Name))
		e.line("nilReference() bool")
		if c.Super != "" {
			e.line("%s", e.ref(c.Super))
		}
		e.line("%s() *%s", e.getter(c.Name), e.obj(c.Name))
		for _, n := range e.effective(c) {
			m, _ := e.method(c, n)
			e.line("%s(%s)%s", e.member(n), e.params(m, false), e.result(m.Result))
		}
		e.line("}")
	}
	for _, f := range c.Fields {
		if f.Static {
			e.line("var %s %s", e.name("static."+c.Name+"."+f.Name), e.typ(f.Type))
		}
	}
	// Reentrant first-use initialization matches ABAP's guarded class initialization.
	e.line("var %s bool", e.name("initialized."+c.Name))
	e.line("func %s() {if %s {return}; %s=true", e.name("init."+c.Name), e.name("initialized."+c.Name), e.name("initialized."+c.Name))
	for _, m := range c.Methods {
		if m.Name == "class_constructor" {
			e.line("%s()", e.body(c.Name, m.Name))
		}
	}
	e.line("}")
	ctor, owner := e.method(c, "constructor")
	params := ""
	args := []string{}
	if ctor != nil {
		params = e.params(ctor, true)
		for i := range ctor.Params {
			args = append(args, fmt.Sprintf("p%d", i))
		}
	}
	e.line("func %s(%s) %s {", e.name("new."+c.Name), params, e.ref(c.Name))
	e.line("%s()", e.name("init."+c.Name))
	e.line("self:=&%s{}", e.obj(c.Name))
	if ctor != nil {
		e.line("%s(self%s)", e.body(owner.Name, "constructor"), comma(args))
	}
	e.line("return self }")
	for _, n := range e.effective(c) {
		m, owner := e.method(c, n)
		a := []string{}
		for i := range m.Params {
			a = append(a, fmt.Sprintf("p%d", i))
		}
		e.line("func (self *%s) %s(%s)%s {", e.obj(c.Name), e.member(n), e.params(m, true), e.result(m.Result))
		if m.Abstract {
			e.line("panic(trap{Source:%q})", "abstract "+c.Name+"."+n)
		} else {
			prefix := ""
			if m.Result.Kind != hir.Void {
				prefix = "return "
			}
			e.line("%s%s(self%s)", prefix, e.body(owner.Name, n), comma(a))
		}
		e.line("}")
	}
	for _, m := range c.Methods {
		if !m.Abstract {
			e.emitBody(c, m)
		}
	}
	if c.Ctor != nil {
		e.emitBody(c, c.Ctor)
	}
}
func comma(a []string) string {
	if len(a) == 0 {
		return ""
	}
	return "," + strings.Join(a, ",")
}
func (e *emitter) emitBody(c *hir.Class, m *hir.Method) {
	params := e.params(m, true)
	if !m.Static {
		params = "self " + e.ref(c.Name) + func() string {
			if params != "" {
				return "," + params
			}
			return ""
		}()
	}
	result := ""
	if m.Result.Kind != hir.Void {
		result = "(result " + e.typ(m.Result) + ")"
	}
	e.line("func %s(%s) %s {", e.body(c.Name, m.Name), params, result)
	if hasReturnBoundary(m.Body) {
		if m.Result.Kind == hir.Void {
			e.line("defer func(){if x:=recover();x!=nil {if _,ok:=x.(returnedVoid);!ok {panic(x)}}}()")
		} else {
			e.line("defer func(){if x:=recover();x!=nil {if r,ok:=x.(returned[%s]);ok {result=r.Value}else{panic(x)}}}()", e.typ(m.Result))
		}
	}
	if !m.Static {
		if e.concreteClass(c.Name) {
			e.line("if self==nil {panic(rangeFault{})}")
		} else {
			e.line("if self==nil || self.nilReference() {panic(rangeFault{})}")
		}
	}
	if m.Name != "class_constructor" {
		e.line("%s()", e.name("init."+c.Name))
	}
	b := &body{e: e, c: c, m: m, locals: map[string]string{}}
	for i, p := range m.Params {
		b.locals[p.Name] = fmt.Sprintf("p%d", i)
		e.line("_ = p%d", i)
	}
	b.stmt(m.Body)
	e.line("return")
	e.line("}")
}

type body struct {
	e        *emitter
	c        *hir.Class
	m        *hir.Method
	locals   map[string]string
	next     int
	tryDepth int
	loops    []int
}

func (b *body) line(f string, a ...any) { b.e.line(f, a...) }
func (b *body) fresh() string           { b.next++; return fmt.Sprintf("v%d", b.next) }
func (b *body) temp(t hir.Type, code string) string {
	if t.Kind == hir.Void {
		b.line("%s", code)
		return ""
	}
	n := b.fresh()
	b.line("var %s %s = %s; _ = %s", n, b.e.typ(t), code, n)
	return n
}
func (b *body) value(x *hir.Expr, t hir.Type) string {
	v := b.expr(x)
	if t.Kind == hir.Optional && x.Type.Kind != hir.Optional && !t.Args[0].IsRef() {
		return "present(" + v + ")"
	}
	if t.Kind == hir.Array && !arrayStorage(t).Equal(arrayStorage(x.Type)) {
		b.e.unsupported(x.Node, "covariant array view")
	}
	return v
}
func (b *body) literal(t hir.Type, v any, node hir.Node) string {
	if t.Kind == hir.Optional {
		if v == nil {
			if t.Args[0].IsRef() {
				return "nil"
			}
			return b.e.typ(t) + "{}"
		}
		s := b.literal(t.Args[0], v, node)
		if t.Args[0].IsRef() {
			return s
		}
		return "present(" + s + ")"
	}
	switch t.Kind {
	case hir.String:
		return b.e.stringLiteral(v.(string))
	case hir.Bool:
		return fmt.Sprint(v)
	case hir.Number:
		f := numberLiteral(v)
		if math.IsInf(f, 0) || math.IsNaN(f) {
			b.e.unsupported(node, "non-finite Number literal")
			return "0"
		}
		if f == 0 && math.Signbit(f) {
			return "negativeZero()"
		}
		return "float64(" + strconv.FormatFloat(f, 'g', -1, 64) + ")"
	default:
		return b.e.typ(t) + "(" + fmt.Sprint(v) + ")"
	}
}
func (b *body) truth(v string, t hir.Type) string {
	switch t.Kind {
	case hir.Optional:
		if t.Args[0].IsRef() {
			return "!nilRef(" + v + ")"
		}
		return v + ".Has && (" + b.truth(v+".Value", t.Args[0]) + ")"
	case hir.Dynamic:
		return "dynTruth(" + v + ")"
	case hir.Bool:
		return v
	case hir.String:
		return v + ".length()!=0"
	case hir.Number:
		return v + "!=0"
	case hir.I32, hir.I64:
		return v + "!=0"
	default:
		return "!nilRef(" + v + ")"
	}
}
func (b *body) lvalue(x *hir.Expr) string {
	e := b.e
	switch x.Kind {
	case hir.Local:
		return b.locals[x.Name]
	case hir.FieldGet:
		v := b.expr(x.X)
		c := e.fieldOwner(e.classBy(x.X.Type.Name), x.Name)
		return v + "." + e.getter(c.Name) + "()." + e.member(x.Name)
	case hir.StaticGet:
		c := e.fieldOwner(e.classBy(x.Owner), x.Name)
		b.line("%s()", e.name("init."+c.Name))
		return e.name("static." + c.Name + "." + x.Name)
	}
	e.unsupported(x.Node, "assignment "+string(x.Kind))
	return ""
}
func (b *body) expr(x *hir.Expr) string {
	e := b.e
	t := x.Type
	code := ""
	switch x.Kind {
	case hir.Lit:
		if v, ok := x.Value.(string); ok && !utf8.ValidString(v) {
			e.unsupported(x.Node, "invalid UTF-8 or lone surrogate literal")
		}
		code = b.literal(t, x.Value, x.Node)
	case hir.Local:
		code = b.locals[x.Name]
	case hir.This:
		code = "self"
	case hir.FieldGet, hir.StaticGet:
		code = b.lvalue(x)
	case hir.IndexGet:
		a, i := b.expr(x.X), b.expr(x.Y)
		code = a + ".get(" + i + ").Value"
		if x.X.Type.Args[0].IsRef() {
			code = "castRef[" + e.typ(t) + "](" + code + ")"
		}
	case hir.New:
		a := []string{}
		if t.Kind == hir.ClassRef {
			m, _ := e.method(e.classBy(t.Name), "constructor")
			for i, v := range x.Args {
				a = append(a, b.value(v, m.Params[i].Type))
			}
			code = e.name("new."+t.Name) + "(" + strings.Join(a, ",") + ")"
		} else {
			for _, v := range x.Args {
				a = append(a, b.expr(v))
			}
			code = "&" + strings.TrimPrefix(e.typ(t), "*") + "{}"
			if t.Kind == hir.RegExp {
				flags := `str("")`
				if len(a) > 1 {
					flags = a[1]
				}
				code = "newRegExp(" + a[0] + "," + flags + ")"
			}
			if t.Kind == hir.Array && len(a) > 0 {
				code = "&" + strings.TrimPrefix(e.typ(t), "*") + "{Items:make([]" + e.typ(arrayStorage(t).Args[0]) + "," + a[0] + ")}"
			}
		}
	case hir.DirectCall, hir.VirtualCall, hir.SuperCall:
		owner := x.Owner
		recv := ""
		if x.Kind == hir.SuperCall {
			owner = b.c.Super
			if x.Name != b.m.Name && !(x.Name == "constructor" && b.m == b.c.Ctor) {
				e.unsupported(x.Node, "cross-method super call")
			}
			recv = "self"
		} else if x.X != nil {
			recv = b.expr(x.X)
			owner = x.X.Type.Name
		}
		m, c := e.method(e.classBy(owner), x.Name)
		if m == nil {
			for _, in := range e.p.Interfaces {
				if in.Name == owner {
					for _, f := range in.Methods {
						if f.Name == x.Name {
							m = f
						}
					}
				}
			}
		}
		a := []string{}
		for i, v := range x.Args {
			a = append(a, b.value(v, m.Params[i].Type))
		}
		if x.Kind == hir.VirtualCall {
			code = recv + "." + e.member(x.Name) + "(" + strings.Join(a, ",") + ")"
		} else {
			if recv != "" {
				a = append([]string{recv}, a...)
			}
			code = e.body(c.Name, m.Name) + "(" + strings.Join(a, ",") + ")"
		}
	case hir.Binary:
		a := b.expr(x.X)
		if x.Op == "&&" || x.Op == "||" {
			n := b.temp(t, a)
			condition := n
			if x.Op == "||" {
				condition = "!" + n
			}
			b.line("if %s {", condition)
			v := b.expr(x.Y)
			b.line("%s=%s }", n, v)
			return n
		}
		v := b.expr(x.Y)
		code = "(" + a + " " + x.Op + " " + v + ")"
		if x.Op == "==" || x.Op == "!=" {
			if x.X.Type.IsRef() || x.X.Type.Kind == hir.Optional && x.X.Type.Args[0].IsRef() {
				code = "equal(" + a + "," + v + ")"
				if x.Op == "!=" {
					code = "!" + code
				}
			}
		}
		if t.Kind == hir.Number && (x.Op == "+" || x.Op == "-" || x.Op == "*" || x.Op == "/") {
			if x.Op == "/" && (x.Y.Kind != hir.Lit || numberLiteral(x.Y.Value) == 0) {
				e.unsupported(x.Node, "Number division requires a non-zero literal divisor")
			}
			code = "finite(" + code + ")"
		}
		if t.Kind == hir.Number && x.Op == "%" {
			e.unsupported(x.Node, "Number remainder")
		}
		if t.Kind == hir.I32 || t.Kind == hir.I64 {
			if t.Kind == hir.I32 && (x.Op == "+" || x.Op == "-" || x.Op == "*") {
				// int64 holds every exact result of these operations on int32 inputs.
				code = fmt.Sprintf("checkedI32(int64(%s) %s int64(%s))", a, x.Op, v)
			} else {
				code = fmt.Sprintf("%s(integerArithmetic(int64(%s),int64(%s),%q,%t,%d))", e.typ(t), a, v, x.Op, x.CheckIntegerOverflow, integerBits(t))
			}
		}
	case hir.Unary:
		a := b.expr(x.X)
		code = x.Op + a
		if t.Kind == hir.I32 || t.Kind == hir.I64 {
			if t.Kind == hir.I32 {
				code = "checkedI32(-int64(" + a + "))"
			} else {
				code = fmt.Sprintf("%s(integerArithmetic(0,int64(%s),%q,%t,%d))", e.typ(t), a, "-", x.CheckIntegerOverflow, integerBits(t))
			}
		} else if t.Kind == hir.Number && x.Op == "-" {
			code = "finite(" + code + ")"
		}
	case hir.Conditional:
		a := b.expr(x.X)
		n := b.fresh()
		b.line("var %s %s; _ = %s; if %s {", n, e.typ(t), n, a)
		v := b.value(x.Y, t)
		b.line("%s=%s }else{", n, v)
		v = b.value(x.Z, t)
		b.line("%s=%s }", n, v)
		return n
	case hir.ClassOf:
		code = "&" + e.name("descriptor."+x.Owner)
	case hir.InstanceOf:
		a := b.expr(x.X)
		if x.Y != nil {
			cv := b.expr(x.Y)
			return b.temp(t, "descriptorInstance("+a+","+cv+")")
		}
		if x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			a = "unwrap(" + a + ")"
		}
		code = "func() bool {_,ok:=any(" + a + ").(" + e.ref(x.Owner) + ");return !nilRef(" + a + ") && ok}()"
	case hir.IsUndefined:
		a := b.expr(x.X)
		code = "nilRef(" + a + ")"
		if x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			code = "!" + a + ".Has"
		}
	case hir.ToBoolean:
		a := b.expr(x.X)
		code = b.truth(a, x.X.Type)
	case hir.Narrow, hir.Cast:
		a := b.expr(x.X)
		src := x.X.Type
		if src.Kind == hir.Optional {
			if !src.Args[0].IsRef() {
				a = "unwrap(" + a + ")"
			}
			src = src.Args[0]
		}
		if src.Equal(t) || src.Kind == hir.Array && t.Kind == hir.Array && arrayStorage(src).Equal(arrayStorage(t)) {
			code = a
		} else if t.Kind == hir.ClassRef || t.Kind == hir.InterfaceRef {
			code = "castRef[" + e.typ(t) + "](" + a + ")"
		} else {
			e.unsupported(x.Node, "narrow "+t.String())
			code = a
		}
	case hir.NumericConvert, hir.CheckedNumericConvert:
		a := b.expr(x.X)
		if x.Kind == hir.CheckedNumericConvert {
			if t.Kind == hir.I32 {
				// An exact round trip proves integrality, finiteness and int32 range.
				narrowed := b.temp(t, e.typ(t)+"("+a+")")
				checks := []string{a + " != " + e.typ(x.X.Type) + "(" + narrowed + ")"}
				if x.Range.Min > math.MinInt32 {
					checks = append(checks, fmt.Sprintf("int64(%s) < %d", narrowed, x.Range.Min))
				}
				if x.Range.Max < math.MaxInt32 {
					checks = append(checks, fmt.Sprintf("int64(%s) > %d", narrowed, x.Range.Max))
				}
				b.line("if %s {panic(rangeFault{})}", strings.Join(checks, " || "))
				return narrowed
			} else {
				b.line("if float64(%s) < %d || float64(%s) > %d || float64(%s)!=float64(%s) || float64(%s)!=float64(%s) {panic(rangeFault{})}", a, x.Range.Min, a, x.Range.Max, a, a, a, e.typ(t)+"("+a+")")
			}
		}
		code = e.typ(t) + "(" + a + ")"
	case hir.NumericMinMax:
		a, v := b.expr(x.X), b.expr(x.Y)
		code = x.Op + "(" + a + "," + v + ")"
	case hir.Seq:
		old := b.locals
		b.locals = clone(old)
		n := b.fresh()
		b.line("var %s %s; _=%s; {", n, e.typ(t), n)
		for _, s := range x.Stmt.List {
			b.stmt(s)
		}
		v := b.expr(x.Y)
		b.line("%s=%s }", n, v)
		b.locals = old
		return n
	case hir.RuntimeOp:
		return b.runtime(x)
	default:
		e.unsupported(x.Node, string(x.Kind))
		code = "*new(" + e.typ(t) + ")"
	}
	return b.temp(t, code)
}
func clone(m map[string]string) map[string]string {
	n := map[string]string{}
	for k, v := range m {
		n[k] = v
	}
	return n
}
func (b *body) stmt(s *hir.Stmt) {
	if s == nil {
		return
	}
	e := b.e
	switch s.Kind {
	case hir.Block:
		old := b.locals
		b.locals = clone(old)
		b.line("{")
		for _, s := range s.List {
			b.stmt(s)
		}
		b.line("}")
		b.locals = old
	case hir.VarDecl:
		n := b.fresh()
		v := ""
		if s.X != nil {
			v = " = " + b.value(s.X, s.Type)
		}
		b.line("var %s %s%s; _=%s", n, e.typ(s.Type), v, n)
		b.locals[s.Name] = n
	case hir.Assign:
		if s.X.Kind == hir.IndexGet {
			a, i := b.expr(s.X.X), b.expr(s.X.Y)
			v := b.value(s.Y, s.X.Type)
			b.line("%s.put(%s,%s)", a, i, v)
		} else {
			a := b.lvalue(s.X)
			v := b.value(s.Y, s.X.Type)
			b.line("%s=%s", a, v)
		}
	case hir.ExprStmt:
		b.expr(s.X)
	case hir.If:
		a := b.expr(s.X)
		b.line("if %s {", a)
		old := b.locals
		b.locals = clone(old)
		b.stmt(s.Body)
		b.locals = old
		if s.Else != nil {
			b.line("}else{")
			b.locals = clone(old)
			b.stmt(s.Else)
			b.locals = old
		}
		b.line("}")
	case hir.While:
		b.line("for {")
		a := b.expr(s.X)
		b.line("if !%s {break}", a)
		b.loops = append(b.loops, b.tryDepth)
		old := b.locals
		b.locals = clone(old)
		b.stmt(s.Body)
		b.locals = old
		b.loops = b.loops[:len(b.loops)-1]
		b.line("}")
	case hir.ForEach:
		a := b.expr(s.X)
		index := b.fresh()
		b.line("for %s:=0; %s<len(%s.Items); %s++ {", index, index, a, index)
		code := a + ".Items[" + index + "]"
		if s.Type.IsRef() {
			code = "castRef[" + e.typ(s.Type) + "](" + code + ")"
		}
		n := b.temp(s.Type, code)
		old := b.locals
		b.locals = clone(old)
		b.locals[s.Name] = n
		b.loops = append(b.loops, b.tryDepth)
		b.stmt(s.Body)
		b.loops = b.loops[:len(b.loops)-1]
		b.locals = old
		b.line("}")
	case hir.Break, hir.Continue:
		if len(b.loops) > 0 && b.loops[len(b.loops)-1] != b.tryDepth {
			e.unsupported(s.Node, "loop control across try")
		}
		b.line(string(s.Kind))
	case hir.Return:
		if b.m.Result.Kind == hir.Void {
			if s.X != nil {
				b.expr(s.X)
			}
			if b.tryDepth == 0 {
				b.line("return")
			} else {
				b.line("panic(returnedVoid{})")
			}
		} else {
			v := b.value(s.X, b.m.Result)
			if b.tryDepth == 0 {
				b.line("return %s", v)
			} else {
				b.line("panic(returned[%s]{%s})", e.typ(b.m.Result), v)
			}
		}
	case hir.Throw:
		v := b.expr(s.X)
		b.line("panic(payload[%s]{Type:%q,Value:%s})", e.typ(s.X.Type), s.X.Type.String(), v)
	case hir.Trap:
		b.line("panic(trap{Source:%q})", s.Name)
	case hir.Finally:
		b.line("func(){defer func(){")
		old := b.locals
		b.locals = clone(old)
		b.tryDepth++
		b.stmt(s.Else)
		b.tryDepth--
		b.locals = old
		b.line("}()")
		b.locals = clone(old)
		b.tryDepth++
		b.stmt(s.Body)
		b.tryDepth--
		b.locals = old
		b.line("}()")
	case hir.Try:
		b.line("func(){defer func(){if x:=recover();x!=nil {if caught,ok:=x.(payload[%s]);ok && caught.Type==%q {", e.typ(s.Type), s.Type.String())
		old := b.locals
		b.locals = clone(old)
		n := b.temp(s.Type, "caught.Value")
		b.locals[s.Name] = n
		b.tryDepth++
		b.stmt(s.Else)
		b.tryDepth--
		b.locals = old
		b.line("}else{panic(x)}}}()")
		b.locals = clone(old)
		b.tryDepth++
		b.stmt(s.Body)
		b.tryDepth--
		b.locals = old
		b.line("}()")
	default:
		e.unsupported(s.Node, string(s.Kind))
	}
}

func integerBits(t hir.Type) int {
	if t.Kind == hir.I32 {
		return 32
	}
	return 64
}

// Check the entire body before emission so an unsupported operation is always
// diagnosed by its catalogue name, even if its operand type is also unsupported.
func (e *emitter) checkOps(s *hir.Stmt) {
	if s == nil {
		return
	}
	e.checkExprOps(s.X)
	e.checkExprOps(s.Y)
	e.checkOps(s.Body)
	e.checkOps(s.Else)
	for _, v := range s.List {
		e.checkOps(v)
	}
}
func (e *emitter) checkExprOps(x *hir.Expr) {
	if x == nil {
		return
	}
	if x.Kind == hir.New && x.Type.Kind == hir.RegExp || x.Kind == hir.RuntimeOp && x.Op == "regexp.new" {
		e.checkRegexp(x)
	}
	if x.Kind == hir.RuntimeOp && !supportedOps[x.Op] {
		e.unsupported(x.Node, x.Op)
	}
	e.checkExprOps(x.X)
	e.checkExprOps(x.Y)
	e.checkExprOps(x.Z)
	for _, a := range x.Args {
		e.checkExprOps(a)
	}
	e.checkOps(x.Stmt)
}

func numberLiteral(v any) float64 {
	r := reflect.ValueOf(v)
	switch {
	case r.Kind() <= reflect.Int64:
		return float64(r.Int())
	case r.Kind() <= reflect.Uintptr:
		return float64(r.Uint())
	default:
		return r.Float()
	}
}

// Returns crossing generated try/finally closures must unwind to the method.
// Ordinary returns use Go control flow and need no recovery wrapper.
func hasReturnBoundary(s *hir.Stmt) bool {
	if s == nil {
		return false
	}
	if s.Kind == hir.Try || s.Kind == hir.Finally {
		return true
	}
	if hasExprReturnBoundary(s.X) || hasExprReturnBoundary(s.Y) || hasReturnBoundary(s.Body) || hasReturnBoundary(s.Else) {
		return true
	}
	for _, child := range s.List {
		if hasReturnBoundary(child) {
			return true
		}
	}
	return false
}
func hasExprReturnBoundary(x *hir.Expr) bool {
	if x == nil {
		return false
	}
	if hasReturnBoundary(x.Stmt) || hasExprReturnBoundary(x.X) || hasExprReturnBoundary(x.Y) || hasExprReturnBoundary(x.Z) {
		return true
	}
	for _, arg := range x.Args {
		if hasExprReturnBoundary(arg) {
			return true
		}
	}
	return false
}

func (e *emitter) concreteClass(name string) bool {
	if name == hir.RootObject {
		return false
	}
	for _, c := range e.p.Classes {
		if c.Super == name {
			return false
		}
	}
	return true
}

// Convert immutable source literals once, rather than in each execution.
func (e *emitter) stringLiteral(value string) string {
	if e.stringLiterals == nil {
		e.stringLiterals = map[string]string{}
	}
	if name, ok := e.stringLiterals[value]; ok {
		return name
	}
	name := e.name("literal." + value)
	e.stringLiterals[value] = name
	fmt.Fprintf(&e.extra, "var %s = str(%q)\n", name, value)
	return name
}
