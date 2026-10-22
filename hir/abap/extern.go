package abap

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/oisee/abapiti/hir"
)

// Extern host ABI on ABAP: a static method of the module's host class per
// exported function, named in snake case, with the TS parameter names. string,
// boolean and number map to string, abap_bool and f; string[] (also the rest
// array) to string_table; [string, boolean][] to the host's TYPES ty_pairs
// (STANDARD TABLE OF a structure with f0 TYPE string and f1 TYPE abap_bool);
// a string | undefined result to REF TO string (unbound is undefined). A host
// exception that is not CX_NO_CHECK becomes a catchable builtin Error with
// its original CX object and diagnostics retained on the payload.
var externABAPName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,29}$`)

func externABAPIdentifier(name string) string {
	var s strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				s.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		s.WriteRune(r)
	}
	return s.String()
}

func externABAPType(class string, t hir.Type) string {
	switch t.Kind {
	case hir.String:
		return "string"
	case hir.Bool:
		return "abap_bool"
	case hir.Number:
		return "f"
	case hir.Array:
		if t.Args[0].Kind == hir.String {
			return "string_table"
		}
		return class + "=>ty_pairs"
	case hir.Optional:
		return "REF TO string"
	}
	return ""
}

// externTemp declares a host-typed temporary, cleared on every execution.
// OSGo refuses VALUE with the DDIC table type string_table and CAST string
// for a data reference. Keep these declarations out of assignment folding: a
// REF TO string host result must remain a data reference until externResult
// reads it through a field symbol.
func (b *body) externTemp(typ string) string {
	if typ != "REF TO string" && !strings.Contains(typ, "_table") && !strings.Contains(typ, "=>") {
		return b.rawTemp(typ)
	}
	b.serial++
	n := fmt.Sprintf("t%d", b.serial)
	b.line("DATA " + n + " TYPE " + typ + ".")
	b.line("CLEAR " + n + ".")
	return n
}

func (b *body) externCall(x *hir.Expr) string {
	e, s := b.e, x.Extern
	if s.ABAPClass == "" {
		e.refuse(hir.Error{Node: x.Node, Message: "extern module call " + x.Name + " is Go-only"})
		return ""
	}
	method := externABAPIdentifier(x.Name)
	names := make([]string, len(s.ParamNames))
	for i, p := range s.ParamNames {
		names[i] = externABAPIdentifier(p)
	}
	for _, n := range append([]string{method}, names...) {
		if !externABAPName.MatchString(n) || n == "result" {
			e.refuse(hir.Error{Node: x.Node, Message: fmt.Sprintf("extern module call %s: %q is not an ABAP host identifier", x.Name, n)})
			return ""
		}
	}
	fixed := len(s.Params)
	if s.Rest {
		fixed--
	}
	// Arguments are evaluated before the host call, outside its TRY: only
	// host exceptions become builtin Error.
	actual := []string{}
	for i := 0; i < fixed; i++ {
		actual = append(actual, names[i]+" = "+b.externArgument(s.ABAPClass, x.Args[i], s.Params[i]))
	}
	if s.Rest {
		rest := b.externTemp("string_table")
		for i := fixed; i < len(x.Args); i++ {
			a := b.expr(x.Args[i])
			if s.Spread && i == len(x.Args)-1 {
				b.line(a + "->dense( ).")
				b.line("APPEND LINES OF " + a + "->items TO " + rest + ".")
			} else {
				b.line("APPEND " + a + " TO " + rest + ".")
			}
		}
		actual = append(actual, names[fixed]+" = "+rest)
	}
	call := s.ABAPClass + "=>" + method + "( " + strings.Join(actual, " ") + " )"
	if len(actual) == 0 {
		call = s.ABAPClass + "=>" + method + "( )"
	}
	result := ""
	if x.Type.Kind != hir.Void {
		result = b.externTemp(externABAPType(s.ABAPClass, x.Type))
		call = result + " = " + call
	}
	failure := b.rawTemp("REF TO cx_root")
	b.line("TRY.")
	b.line(call + ".")
	b.line("CATCH cx_static_check cx_dynamic_check INTO " + failure + ".")
	e.abapErrors = true
	errorType := hir.Ref("builtin.Error")
	payload := b.expr(&hir.Expr{Kind: hir.New, Node: x.Node, Type: errorType})
	b.line(payload + "->capture_abap( " + failure + " ).")
	local := fmt.Sprintf("extern.error.%d", b.serial)
	b.locals[local] = payload
	b.stmt(&hir.Stmt{Kind: hir.Throw, Node: x.Node, X: hir.V(local, errorType)})
	delete(b.locals, local)
	b.line("ENDTRY.")
	if result == "" {
		return ""
	}
	return b.externResult(x, result)
}

func (b *body) externArgument(class string, x *hir.Expr, t hir.Type) string {
	a := b.expr(x)
	switch t.Kind {
	case hir.Array:
		v := b.externTemp(externABAPType(class, t))
		b.line(a + "->dense( ).")
		if t.Args[0].Kind == hir.String {
			b.line(v + " = " + a + "->items.")
			return v
		}
		el := t.Args[0]
		item, tuple := b.rawTemp("REF TO object"), b.rawTemp("REF TO "+b.e.name(el.Name))
		local := fmt.Sprintf("extern.tuple.%d", b.serial)
		b.locals[local] = tuple
		b.line("LOOP AT " + a + "->items INTO " + item + ".")
		b.line(tuple + " ?= " + item + ".")
		f0 := b.expr(&hir.Expr{Kind: hir.FieldGet, Node: x.Node, Name: "f0", Type: hir.T(hir.String), X: hir.V(local, el)})
		f1 := b.expr(&hir.Expr{Kind: hir.FieldGet, Node: x.Node, Name: "f1", Type: hir.T(hir.Bool), X: hir.V(local, el)})
		b.line("APPEND VALUE #( f0 = " + f0 + " f1 = " + f1 + " ) TO " + v + ".")
		b.line("ENDLOOP.")
		delete(b.locals, local)
		return v
	}
	v := b.rawTemp(externABAPType(class, t))
	b.line(v + " = " + a + ".")
	return v
}

func (b *body) externResult(x *hir.Expr, result string) string {
	t := x.Type
	switch t.Kind {
	case hir.String, hir.Bool, hir.Number:
		return result
	case hir.Optional:
		// OSGo reads a data reference through a field symbol only.
		n := b.temp(t)
		b.serial++
		value := fmt.Sprintf("<t%d>", b.serial)
		b.line("FIELD-SYMBOLS " + value + " TYPE string.")
		b.line("IF " + result + " IS BOUND.")
		b.line("ASSIGN " + result + "->* TO " + value + ".")
		b.line(n + " = NEW #( ).")
		b.line(n + "->has = abap_true.")
		b.line(n + "->value = " + value + ".")
		b.line("ENDIF.")
		return n
	}
	n := b.temp(t)
	b.line("CREATE OBJECT " + n + ".")
	if t.Args[0].Kind == hir.String {
		b.line(n + "->items = " + result + ".")
		return n
	}
	el := t.Args[0]
	row := b.externTemp(x.Extern.ABAPClass + "=>ty_pair")
	f0, f1 := b.rawTemp("string"), b.rawTemp("abap_bool")
	l0, l1 := fmt.Sprintf("extern.f0.%d", b.serial), fmt.Sprintf("extern.f1.%d", b.serial)
	b.locals[l0], b.locals[l1] = f0, f1
	b.line("LOOP AT " + result + " INTO " + row + ".")
	b.line(f0 + " = " + row + "-f0.")
	b.line(f1 + " = " + row + "-f1.")
	tuple := b.expr(&hir.Expr{Kind: hir.New, Node: x.Node, Type: el, Args: []*hir.Expr{hir.V(l0, hir.T(hir.String)), hir.V(l1, hir.T(hir.Bool))}})
	b.line("APPEND " + tuple + " TO " + n + "->items.")
	b.line("ENDLOOP.")
	delete(b.locals, l0)
	delete(b.locals, l1)
	return n
}
