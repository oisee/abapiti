package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// slotInterface reports an interface of methods plus optional `unknown` data
// slots (OSG's ArtifactObject: getType(), getName(), ..., syntaxResult?,
// def?). Its values are ordinary class instances (the core's IObject
// objects), so it lowers to a method interface; a slot read is the object's
// own JS property, or undefined (slotRead).
func (l *lowerer) slotInterface(node *ast.Node) bool {
	if !l.unprunedFeatures || node == nil || node.Kind != ast.KindInterfaceDeclaration || node.AsInterfaceDeclaration().HeritageClauses != nil || node.AsInterfaceDeclaration().TypeParameters != nil {
		return false
	}
	methods, slots := 0, 0
	for _, m := range node.Members() {
		switch m.Kind {
		case ast.KindMethodSignature:
			methods++
		case ast.KindPropertySignature:
			if m.QuestionToken() == nil || m.Type() == nil || m.Type().Kind != ast.KindUnknownKeyword || m.Name() == nil || m.Name().Kind != ast.KindIdentifier {
				return false
			}
			slots++
		default:
			return false
		}
	}
	return methods > 0 && slots > 0
}

// slotInterfaceRef is the declaration of a lowered slot interface type.
func (l *lowerer) slotInterfaceRef(t hir.Type) bool {
	if t.Kind != hir.InterfaceRef || !l.unprunedFeatures {
		return false
	}
	if known, ok := l.slotIfaces[t.Name]; ok {
		return known
	}
	if l.slotIfaces == nil {
		l.slotIfaces = map[string]bool{}
	}
	found := false
	for sym, i := range l.ifaces {
		if i.Name == t.Name {
			for _, d := range sym.Declarations {
				found = found || l.slotInterface(d)
			}
		}
	}
	l.slotIfaces[t.Name] = found
	return found
}

// slotRead lowers a read of any property name (a declared slot, or a key
// computed at run time) on a slot interface value: the object's own field or
// prototype member, else undefined, exactly as JavaScript reads it.
func (l *lowerer) slotRead(n *ast.Node, recv *hir.Expr, key *hir.Expr) *hir.Expr {
	if recv.Type.Kind == hir.Optional {
		recv = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: recv.Type.Args[0], X: recv}
	}
	return l.rtOp("object.get", recv, hir.T(hir.Dynamic), key)
}

// slotInterfaceImplements records every lowered class whose methods match a
// slot interface's exactly as implementing it: the core's IObject classes
// reach ArtifactObject through IObject values and checked narrowings
// (`o instanceof Group`) without a class-to-interface coercion.
func (l *lowerer) slotInterfaceImplements() {
	if !l.unprunedFeatures {
		return
	}
	for _, i := range l.out.Interfaces {
		if len(i.Methods) == 0 || !l.slotInterfaceRef(hir.Type{Kind: hir.InterfaceRef, Name: i.Name}) {
			continue
		}
		for _, c := range l.out.Classes {
			if c.Structural || l.shapeLike[c.Name] {
				continue
			}
			complete := true
			for _, m := range i.Methods {
				var impl *hir.Method
				for owner := c; owner != nil && impl == nil; owner = l.classByName(owner.Super) {
					for _, cm := range owner.Methods {
						if cm.Name == m.Name && !cm.Static {
							impl = cm
							break
						}
					}
				}
				complete = complete && impl != nil && sameMethodSignature(m, impl)
			}
			if complete {
				l.recordImplements(hir.Ref(c.Name), hir.Type{Kind: hir.InterfaceRef, Name: i.Name})
			}
		}
	}
}

// sameMethodSignature is the verifier's interface implementation check.
func sameMethodSignature(a, b *hir.Method) bool {
	if !a.Result.Equal(b.Result) || len(a.Params) != len(b.Params) {
		return false
	}
	for i, p := range a.Params {
		if p.Name != b.Params[i].Name || !p.Type.Equal(b.Params[i].Type) || p.Variadic != b.Params[i].Variadic {
			return false
		}
	}
	return true
}

// slotIntersectionNarrowing: `o instanceof Class` on a slot interface value
// narrows to ArtifactObject & Class when Class is not assignable to it (a
// private field shares a slot's name), and a later union of such checks to
// Group | (ArtifactObject & Class). The value is an instance of the classes
// the checks proved: a checked cast to them, never a structural copy.
func (l *lowerer) slotIntersectionNarrowing(n *ast.Node, x *hir.Expr, base hir.Type, t *checker.Type) (*hir.Expr, bool) {
	if !l.unprunedFeatures || t.Flags()&(checker.TypeFlagsIntersection|checker.TypeFlagsUnion) == 0 || !l.slotInterfaceRef(base) {
		return nil, false
	}
	members := []*checker.Type{t}
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		members = t.AsUnionOrIntersectionType().Types()
	}
	var classes []*checker.Type
	intersected := false
	for _, m := range members {
		if m.Flags()&checker.TypeFlagsIntersection == 0 {
			if m.Symbol() == nil || m.Symbol().Flags&ast.SymbolFlagsClass == 0 || l.classOf(m.Symbol()) == nil {
				return nil, false
			}
			classes = append(classes, m)
			continue
		}
		intersected = true
		var class *checker.Type
		for _, part := range m.AsUnionOrIntersectionType().Types() {
			if part.Symbol() == nil || part.Symbol().Flags&ast.SymbolFlagsClass == 0 {
				continue
			}
			c := l.classOf(part.Symbol())
			if c == nil || c.Structural || l.shapeLike[c.Name] {
				return nil, false
			}
			if class != nil {
				// Group & Clas after `obj instanceof Clas` on a value proven
				// Group | Clas: the enclosing guard names the class.
				return l.neverGuardNarrowing(n, x, t)
			}
			class = part
		}
		if class == nil {
			return nil, false
		}
		classes = append(classes, class)
	}
	if !intersected {
		return nil, false
	}
	target := classes[0]
	if len(classes) > 1 {
		target = l.ck.GetUnionType(classes)
	}
	before := len(l.diags)
	typ := l.mapCheckerType(n, target)
	if hasBlocking(l.diags[before:]) || typ.Kind != hir.ClassRef && typ.Kind != hir.InterfaceRef || strings.HasPrefix(typ.Name, "shape.") || l.shapeLike[typ.Name] {
		l.diags = l.diags[:before]
		return nil, false
	}
	l.diags = l.diags[:before]
	return l.narrowed(n, x, typ), true
}

// slotInterfaceReceiver reports a receiver the checker types as a slot
// interface itself: its method signatures dispatch on the object, even when a
// structural copy of the interface (an intersection with it) registered them
// as function-valued fields.
func (l *lowerer) slotInterfaceReceiver(recv *ast.Node) bool {
	if !l.unprunedFeatures || recv == nil {
		return false
	}
	t := l.ck.GetTypeAtLocation(recv)
	if t != nil {
		t = l.ck.GetNonNullableType(t)
	}
	if _, ok := l.optionalSlotIntersection(t); ok {
		return true
	}
	if t == nil || t.Symbol() == nil || t.Flags()&checker.TypeFlagsObject == 0 {
		return false
	}
	if picked, ok := l.pickedMethodsSource(recv, t); ok && l.slotInterfaceRef(picked) {
		return true
	}
	for _, d := range t.Symbol().Declarations {
		if l.slotInterface(d) {
			return true
		}
	}
	return false
}

// A slot interface intersected with optional members still carries the same
// objects. Optional prototype methods are dispatched on their runtime class;
// they must not turn the existing method interface into copied function fields.
func (l *lowerer) optionalSlotIntersection(t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || t == nil || t.Flags()&checker.TypeFlagsIntersection == 0 {
		return hir.Type{}, false
	}
	for _, part := range t.AsUnionOrIntersectionType().Types() {
		if part.Symbol() == nil {
			continue
		}
		base := l.ifaceOf(part.Symbol())
		if base == nil {
			continue
		}
		typ := hir.Type{Kind: hir.InterfaceRef, Name: base.Name}
		if !l.slotInterfaceRef(typ) {
			continue
		}
		valid := true
		for _, prop := range l.ck.GetPropertiesOfType(t) {
			if l.ck.GetPropertyOfType(part, prop.Name) == nil && prop.Flags&ast.SymbolFlagsOptional == 0 {
				valid = false
				break
			}
		}
		if valid {
			return typ, true
		}
	}
	return hir.Type{}, false
}
