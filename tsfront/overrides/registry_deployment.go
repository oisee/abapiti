package overrides

import "github.com/oisee/abapiti/hir"

// RegistryDeployment replaces the reflective object inventory only for the
// explicitly authorized five-type deployment. Both original spans are pinned.
func RegistryDeployment() []Entry {
	refs := []Key{
		{"src/objects/class.ts", "Class", "KindClassDeclaration"},
		{"src/objects/interface.ts", "Interface", "KindClassDeclaration"},
		{"src/objects/program.ts", "Program", "KindClassDeclaration"},
		{"src/objects/type_pool.ts", "TypePool", "KindClassDeclaration"},
		{"src/objects/transformation.ts", "Transformation", "KindClassDeclaration"},
		{"src/objects/_iobject.ts", "IObject", "KindInterfaceDeclaration"},
	}
	entries := []Entry{
		{ID: "abaplint-deployment-factory", Key: Key{"src/artifacts_objects.ts", "ArtifactsObjects.newObject", "KindMethodDeclaration"}, SHA256: "35aeaedb7b7aa7d9db79b05357ff0494df605b9000820b974c615fc571703bb5", Rationale: "authorized CLAS/INTF/PROG/TYPE/XSLT factory; other object types raise the located coverage trap", References: refs, Method: func() *hir.Method {
			result := hir.Type{Kind: hir.InterfaceRef, Name: "src/objects/_iobject.ts.IObject"}
			m := &hir.Method{Name: "newObject", Static: true, Result: result, Params: []hir.Param{{Name: "name", Type: hir.T(hir.String)}, {Name: "type", Type: hir.T(hir.String)}}}
			var body []*hir.Stmt
			for i, typ := range []string{"CLAS", "INTF", "PROG", "TYPE", "XSLT"} {
				ref := refs[i]
				constructed := &hir.Expr{Kind: hir.New, Type: hir.Ref(ref.File + "." + ref.Symbol), Args: []*hir.Expr{hir.V("name", hir.T(hir.String))}}
				body = append(body, &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Binary, Op: "===", Type: hir.T(hir.Bool), X: hir.V("type", hir.T(hir.String)), Y: hir.L(hir.T(hir.String), typ)}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Cast, Type: result, X: constructed}})})
			}
			body = append(body, &hir.Stmt{Kind: hir.Trap, Name: "src/artifacts_objects.ts:8"})
			m.Body = hir.B(body...)
			return m
		}},
		{ID: "abaplint-deployment-reflection-trap", Key: Key{"src/artifacts_objects.ts", "ArtifactsObjects.buildObjectMap", "KindMethodDeclaration"}, SHA256: "60600506cb27de399926e1e11c7f3f02603e9965b8bc753c0c44e23261ecdc6d", Rationale: "reflective inventory replaced by the explicit five-type factory; direct invocation traps", Method: func() *hir.Method {
			return &hir.Method{Name: "buildObjectMap", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Trap, Name: "src/artifacts_objects.ts:20"})}
		}},
	}
	entries = append(entries, registryClocks()...)
	entries = append(entries, registryXML()...)
	entries = append(entries, registryIterators()...)
	entries = append(entries, registryJSON()...)
	return append(entries, registryOpaqueSignatures()...)
}
