package tsfront

import (
	"context"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// HIR method interfaces are flat. Preserve the public inherited slots before
// class bridges and union views are completed, regardless of source-file order.
func (l *lowerer) completeInterfaceHeritage() {
	l.ifaceClassBases = map[string]string{}
	bases := map[string][]hir.Type{}
	for name, node := range l.ifaceNodes {
		heritage := node.AsInterfaceDeclaration().HeritageClauses
		if heritage == nil {
			continue
		}
		savedFile, savedChecker := l.file, l.ck
		f := ast.GetSourceFileOfNode(node)
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		for _, clause := range heritage.Nodes {
			for _, base := range clause.AsHeritageClause().Types.Nodes {
				sym := l.resolve(base.Expression())
				if c := l.classOf(sym); c != nil {
					bases[name] = append(bases[name], hir.Ref(c.Name))
				} else if i := l.ifaceOf(sym); i != nil {
					bases[name] = append(bases[name], hir.Type{Kind: hir.InterfaceRef, Name: i.Name})
				} else {
					l.diagf(base, "unsupported-interface-heritage", "interface base is not a lowered class or method interface")
				}
			}
		}
		done()
		l.file, l.ck = savedFile, savedChecker
	}
	interfaces := map[string]*hir.Interface{}
	names := []string{}
	for _, i := range l.out.Interfaces {
		interfaces[i.Name] = i
		names = append(names, i.Name)
	}
	sortStrings(names)
	completed, visiting := map[string]bool{}, map[string]bool{}
	var complete func(string)
	complete = func(name string) {
		if completed[name] || visiting[name] {
			return
		}
		visiting[name] = true
		iface := interfaces[name]
		if iface == nil {
			return
		}
		own := map[string]bool{}
		for _, m := range iface.Methods {
			own[m.Name] = true
		}
		for _, base := range bases[name] {
			if base.Kind == hir.InterfaceRef {
				complete(base.Name)
			}
			if base.Kind == hir.ClassRef && l.classHasNominalBrand(base.Name) {
				l.ifaceClassBases[name] = base.Name
			} else if parent := l.ifaceClassBases[base.Name]; parent != "" {
				l.ifaceClassBases[name] = parent
			}
			methods := l.unionMethods(base)
			inherited := []string{}
			for method := range methods {
				inherited = append(inherited, method)
			}
			sortStrings(inherited)
			for _, method := range inherited {
				if own[method] {
					continue
				}
				copy := *methods[method]
				copy.Abstract, copy.Body = false, nil
				copy.Params = append([]hir.Param(nil), copy.Params...)
				iface.Methods = append(iface.Methods, &copy)
				own[method] = true
			}
		}
		visiting[name], completed[name] = false, true
	}
	for _, name := range names {
		complete(name)
	}
	// Nominal ABAP casts to an inherited interface need concrete declarations
	// of that interface, although TypeScript inherits them structurally.
	for _, class := range l.out.Classes {
		seen := map[string]bool{}
		var inherit func(string)
		inherit = func(name string) {
			if seen[name] {
				return
			}
			seen[name] = true
			for _, base := range bases[name] {
				if base.Kind != hir.InterfaceRef {
					continue
				}
				l.recordImplements(hir.Ref(class.Name), base)
				inherit(base.Name)
			}
		}
		for _, name := range class.Implements {
			inherit(name)
		}
	}
	l.covariantImplements()
}

// A public-only class is structural in TypeScript: implementing its interface
// need not instantiate that class. Only a private/protected instance brand
// proves that an interface-to-base-class ABAP cast preserves the JS reference.
func (l *lowerer) classHasNominalBrand(name string) bool {
	for class := l.classByName(name); class != nil; class = l.classByName(class.Super) {
		for symbol, declared := range l.classes {
			if declared.Name != class.Name {
				continue
			}
			for _, node := range symbol.Declarations {
				for _, member := range node.Members() {
					if member.Kind == ast.KindConstructor {
						for _, parameter := range member.Parameters() {
							if parameter.ModifierFlags()&(ast.ModifierFlagsPrivate|ast.ModifierFlagsProtected) != 0 {
								return true
							}
						}
						continue
					}
					if member.ModifierFlags()&ast.ModifierFlagsStatic == 0 && (member.ModifierFlags()&(ast.ModifierFlagsPrivate|ast.ModifierFlagsProtected) != 0 || member.Name() != nil && member.Name().Kind == ast.KindPrivateIdentifier) {
						return true
					}
				}
			}
		}
	}
	return false
}
