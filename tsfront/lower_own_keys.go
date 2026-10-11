package tsfront

import "github.com/oisee/abapiti/internal/tsgo/ast"

// Keep presence separate from values: an explicit undefined property is still
// enumerable. Unknown spread order is refused by the backend, never guessed
// from the destination layout.
func (l *lowerer) literalOwnKeys(n *ast.Node) *[]string {
	keys := []string{}
	seen := map[string]bool{}
	for _, p := range n.AsObjectLiteralExpression().Properties.Nodes {
		if p.Kind == ast.KindSpreadAssignment {
			return nil
		}
		name := propertyNameNode(p)
		if name == nil {
			return nil
		}
		var key string
		if name.Kind == ast.KindComputedPropertyName {
			var ok bool
			key, ok = l.constantKeyName(name)
			if !ok {
				return nil
			}
		} else {
			key = name.Text()
		}
		// An ordinary __proto__ initializer is the JS prototype setter,
		// unlike a computed key or shorthand property. This layout has no ABI
		// for that behavior, so enumeration must retain an explicit refusal.
		if key == "__proto__" && p.Kind == ast.KindPropertyAssignment && name.Kind != ast.KindComputedPropertyName {
			return nil
		}
		if seen[key] {
			return nil
		} // Earlier duplicate initializers are not retained by the shape ABI.
		if !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	return &keys
}
