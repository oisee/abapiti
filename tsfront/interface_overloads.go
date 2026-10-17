package tsfront

import "github.com/oisee/abapiti/internal/tsgo/ast"

// A fixed-arity overload set can share a declared general signature when
// that signature accepts every overload's arguments and returned values.
// Keep the checker's literal-call result views; only the physical ABI is
// shared. Sets without such a signature retain the existing refusal.
func (l *lowerer) coveringInterfaceOverload(node, member *ast.Node) *ast.Node {
	if member.Symbol() == nil {
		return nil
	}
	var group []*ast.Node
	for _, m := range node.Members() {
		if (m.Kind == ast.KindMethodSignature || m.Kind == ast.KindMethodDeclaration) && m.Symbol() == member.Symbol() {
			group = append(group, m)
		}
	}
	if len(group) < 2 {
		return nil
	}
	for _, m := range group {
		for _, p := range m.Parameters() {
			if ast.IsThisParameter(p) || p.AsParameterDeclaration().DotDotDotToken != nil {
				return nil
			}
		}
	}
	for i := len(group) - 1; i >= 0; i-- {
		candidate := l.ck.GetSignatureFromDeclaration(group[i])
		if candidate == nil {
			continue
		}
		covers := true
		for _, other := range group {
			sig := l.ck.GetSignatureFromDeclaration(other)
			if sig == nil || len(sig.Parameters()) != len(candidate.Parameters()) ||
				!l.ck.IsTypeAssignableTo(l.ck.GetReturnTypeOfSignature(sig), l.ck.GetReturnTypeOfSignature(candidate)) {
				covers = false
				break
			}
			for j, p := range sig.Parameters() {
				if !l.ck.IsTypeAssignableTo(l.ck.GetTypeOfSymbol(p), l.ck.GetTypeOfSymbol(candidate.Parameters()[j])) {
					covers = false
					break
				}
			}
			if !covers {
				break
			}
		}
		if covers {
			return group[i]
		}
	}
	return nil
}
