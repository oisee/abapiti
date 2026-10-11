package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Body lowering can discover new union views. Their optional tails widen
// existing slots, which in turn widen other views sharing an implementation.
// Complete that finite closure before binding calls to the final physical ABI.
func (l *lowerer) stabilizeUnionABI() {
	for {
		before := l.unionABIStamp()
		l.completeUnionInterfaces()
		l.covariantImplements()
		l.eraseGenericOverrides()
		l.completeUnionInterfaces()
		if before == l.unionABIStamp() {
			return
		}
	}
}

func (l *lowerer) unionABIStamp() string {
	var out strings.Builder
	for _, c := range l.out.Classes {
		out.WriteString(c.Name)
		for _, m := range c.Methods {
			out.WriteString(m.Name)
			out.WriteString(m.Result.String())
			for _, p := range m.Params {
				out.WriteString(p.Name)
				out.WriteString(p.Type.String())
				out.WriteByte(';')
			}
			out.WriteByte(';')
		}
	}
	for _, i := range l.out.Interfaces {
		out.WriteString(i.Name)
		out.WriteByte('{')
		for _, m := range i.Methods {
			out.WriteString(m.Name)
			out.WriteByte('(')
			for _, p := range m.Params {
				out.WriteString(p.Name)
				out.WriteByte(':')
				out.WriteString(p.Type.String())
				if p.Variadic {
					out.WriteByte('*')
				}
				out.WriteByte(';')
			}
			out.WriteByte(')')
			out.WriteString(m.Result.String())
			out.WriteByte(';')
		}
		out.WriteByte('}')
	}
	return out.String()
}

// Calls were lowered against the slot available at their source position.
// Late ABI widening introduces only optional tails in compatible union views;
// those omitted JavaScript arguments must be explicitly absent in physical HIR.
// Leave required/malformed calls for the ordinary verifier to reject.
func (l *lowerer) completeCallArguments() {
	methods := map[string]map[string]*hir.Method{}
	supers := map[string]string{}
	for _, c := range l.out.Classes {
		methods[c.Name] = map[string]*hir.Method{}
		supers[c.Name] = c.Super
		for _, m := range c.Methods {
			methods[c.Name][m.Name] = m
		}
	}
	for _, i := range l.out.Interfaces {
		methods[i.Name] = map[string]*hir.Method{}
		for _, m := range i.Methods {
			methods[i.Name][m.Name] = m
		}
	}
	for _, c := range l.out.Classes {
		for _, m := range numberMethods(c) {
			walkNumberStmt(m.Body, func(*hir.Stmt) {}, func(e *hir.Expr) {
				owner := e.Owner
				switch e.Kind {
				case hir.VirtualCall:
					if e.X == nil {
						return
					}
					owner = e.X.Type.Name
				case hir.SuperCall:
					owner = c.Super
				case hir.DirectCall:
					if e.X != nil {
						owner = e.X.Type.Name
					}
				default:
					return
				}
				var target *hir.Method
				for owner != "" {
					target = methods[owner][e.Name]
					if target != nil {
						break
					}
					owner = supers[owner]
				}
				if target == nil || len(e.Args) >= len(target.Params) {
					return
				}
				for _, p := range target.Params[len(e.Args):] {
					if p.Type.Kind != hir.Optional {
						return
					}
				}
				for _, p := range target.Params[len(e.Args):] {
					e.Args = append(e.Args, &hir.Expr{Kind: hir.Lit, Node: e.Node, Type: p.Type})
				}
			})
		}
	}
}
