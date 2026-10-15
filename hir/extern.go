package hir

import "strings"

const ExternCall ExprKind = "extern.call"

type ExternSignature struct {
	Params []Type
	Rest   bool
	Spread bool
}

func ExternType(t Type, result bool) bool {
	switch t.Kind {
	case String, Bool, Number:
		return true
	case Void:
		return result
	case Optional:
		return result && len(t.Args) == 1 && t.Args[0].Kind == String
	case Array:
		return len(t.Args) == 1 && (t.Args[0].Kind == String || t.Args[0].Kind == ClassRef && strings.HasPrefix(t.Args[0].Name, "tuple."))
	}
	return false
}

func (v *verifier) externCall(e *Expr, check func(*Expr) Type) {
	s := e.Extern
	if s == nil || e.Owner == "" || e.Name == "" || !v.externType(e.Type, true) {
		v.fail(e.Node, "invalid extern call signature")
		return
	}
	if s.Spread && (!s.Rest || len(e.Args) == 0) {
		v.fail(e.Node, "invalid extern spread")
		return
	}
	min := len(s.Params)
	if s.Rest {
		min--
	}
	if min < 0 || len(e.Args) < min || s.Spread && len(e.Args) <= min || !s.Rest && len(e.Args) != min {
		v.fail(e.Node, "extern call arity mismatch")
		return
	}
	for i, p := range s.Params {
		if !v.externType(p, false) || s.Rest && i == len(s.Params)-1 && (p.Kind != Array || len(p.Args) != 1 || p.Args[0].Kind != String) {
			v.fail(e.Node, "invalid extern parameter")
			return
		}
	}
	for i, a := range e.Args {
		var p Type
		if s.Rest && i >= min {
			p = s.Params[min].Args[0]
			if s.Spread && i == len(e.Args)-1 {
				p = s.Params[min]
			}
		} else {
			p = s.Params[i]
		}
		if !check(a).Equal(p) {
			v.fail(e.Node, "extern argument type mismatch")
		}
	}
}

func (v *verifier) externType(t Type, result bool) bool {
	if !ExternType(t, result) {
		return false
	}
	if t.Kind == Array && t.Args[0].Kind == ClassRef {
		c := v.classes[t.Args[0].Name]
		return c != nil && len(c.Fields) == 2 && c.Fields[0].Name == "f0" && c.Fields[0].Type.Kind == String && c.Fields[1].Name == "f1" && c.Fields[1].Type.Kind == Bool
	}
	return true
}
