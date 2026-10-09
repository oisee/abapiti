package tsfront

import (
	"context"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Interface heritage: a TypeScript interface that extends other interfaces or
// a class carries their members. The lowered HIR interface must carry them
// too, otherwise a union `IClassDefinition | IInterfaceDefinition` has no
// common base and a call like `def.getName()` resolves to no interface slot.
//
// Interface bases are flattened during the signature pass (the base is
// lowered first when it lives in a later file). Class bases are completed
// after class signatures exist, then propagated through interface edges.

type ifaceClassBase struct {
	iface *hir.Interface
	class *hir.Class
}

// interfaceHeritage appends the members of each `extends` base to i.
func (l *lowerer) interfaceHeritage(node *ast.Node, i *hir.Interface) {
	heritage := node.AsInterfaceDeclaration().HeritageClauses
	if heritage == nil {
		return
	}
	for _, clause := range heritage.Nodes {
		if clause.AsHeritageClause().Types == nil {
			continue
		}
		for _, baseNode := range clause.AsHeritageClause().Types.Nodes {
			expr := baseNode.Expression()
			if expr == nil {
				continue
			}
			sym := l.resolve(expr)
			if sym == nil {
				l.diagf(baseNode, "note-interface-heritage", "interface %s: unresolved base is not flattened", i.Name)
				continue
			}
			if base := l.ensureInterfaceSignatures(sym); base != nil {
				l.appendInterfaceMethods(i, base.Methods)
				if l.ifaceBases == nil {
					l.ifaceBases = map[*hir.Interface][]*hir.Interface{}
				}
				l.ifaceBases[i] = append(l.ifaceBases[i], base)
				continue
			}
			if c := l.classOf(sym); c != nil {
				l.ifaceClassBases = append(l.ifaceClassBases, ifaceClassBase{iface: i, class: c})
				continue
			}
			l.diagf(baseNode, "note-interface-heritage", "interface %s: base %s is not lowered; its members are not flattened", i.Name, expr.Text())
		}
	}
}

// ensureInterfaceSignatures lowers the signatures of a non-data interface
// declared in any file before its members are copied.
func (l *lowerer) ensureInterfaceSignatures(sym *ast.Symbol) *hir.Interface {
	i := l.ifaceOf(sym)
	if i == nil {
		return nil
	}
	if l.ifaceDone[i] {
		return i
	}
	var decl *ast.Node
	for _, d := range sym.Declarations {
		if d.Kind == ast.KindInterfaceDeclaration {
			decl = d
			break
		}
	}
	if decl == nil || l.isDataInterface(decl) {
		return i
	}
	f := ast.GetSourceFileOfNode(decl)
	if f == nil {
		return i
	}
	savedFile, savedCk := l.file, l.ck
	ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
	l.file, l.ck = f, ck
	l.interfaceSignatures(decl, i)
	done()
	l.file, l.ck = savedFile, savedCk
	return i
}

// appendInterfaceMethods adds the methods i does not declare itself.
func (l *lowerer) appendInterfaceMethods(i *hir.Interface, methods []*hir.Method) {
	for _, m := range methods {
		if strings.Contains(m.Name, "_instantiated_") {
			continue
		}
		dup := false
		for _, own := range i.Methods {
			if own.Name == m.Name {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		cp := *m
		cp.Virtual = true
		cp.Static = false
		cp.Abstract = false
		cp.Body = nil
		cp.Params = append([]hir.Param(nil), m.Params...)
		i.Methods = append(i.Methods, &cp)
	}
}

// completeIfaceClassHeritage runs once class signatures exist: class bases
// contribute their public instance methods, then every interface edge is
// propagated to a fixed point.
func (l *lowerer) completeIfaceClassHeritage() {
	if l.ifaceClassBaseNames == nil {
		l.ifaceClassBaseNames = map[string][]string{}
	}
	for _, b := range l.ifaceClassBases {
		l.ifaceClassBaseNames[b.iface.Name] = append(l.ifaceClassBaseNames[b.iface.Name], b.class.Name)
		methods := l.unionMethods(hir.Ref(b.class.Name))
		names := make([]string, 0, len(methods))
		for name := range methods {
			names = append(names, name)
		}
		sort.Strings(names)
		var list []*hir.Method
		for _, name := range names {
			list = append(list, methods[name])
		}
		l.appendInterfaceMethods(b.iface, list)
	}
	for changed := true; changed; {
		changed = false
		for iface, bases := range l.ifaceBases {
			for _, base := range bases {
				before := len(iface.Methods)
				l.appendInterfaceMethods(iface, base.Methods)
				if len(iface.Methods) != before {
					changed = true
				}
				for _, class := range l.ifaceClassBaseNames[base.Name] {
					if !containsString(l.ifaceClassBaseNames[iface.Name], class) {
						l.ifaceClassBaseNames[iface.Name] = append(l.ifaceClassBaseNames[iface.Name], class)
						changed = true
					}
				}
			}
		}
	}
	// A class implementing a derived interface implements its bases too.
	for _, c := range l.out.Classes {
		for i := 0; i < len(c.Implements); i++ {
			for _, base := range l.interfaceBasesByName(c.Implements[i]) {
				if !containsString(c.Implements, base) {
					c.Implements = append(c.Implements, base)
				}
			}
		}
	}
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// interfaceBasesByName lists the direct interface bases of a lowered interface.
func (l *lowerer) interfaceBasesByName(name string) []string {
	var out []string
	for iface, bases := range l.ifaceBases {
		if iface.Name != name {
			continue
		}
		for _, b := range bases {
			out = append(out, b.Name)
		}
	}
	return out
}

// unionPartsAccepted reports whether every constituent of a union view is
// accepted by the class (the view is then a proven subtype at run time).
func (l *lowerer) unionPartsAccepted(view string, class hir.Type) bool {
	for _, u := range l.unions {
		if u.iface.Name != view {
			continue
		}
		for _, p := range u.parts {
			if !l.acceptsType(class, p) {
				return false
			}
		}
		return len(u.parts) > 0
	}
	return false
}

// interfaceHasClassBase reports whether every implementer of the interface
// is, by declaration, an instance of the class (the interface extends it).
func (l *lowerer) interfaceHasClassBase(iface string, class hir.Type) bool {
	for _, base := range l.ifaceClassBaseNames[iface] {
		if l.acceptsType(class, hir.Ref(base)) {
			return true
		}
	}
	return false
}
