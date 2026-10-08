package overrides

import "github.com/oisee/abapiti/hir"

// XMLRawMethod routes the package's external parser call to the reviewed
// subset adapter. Missing XML stays undefined; malformed/unsupported XML
// raises RegistryXMLSubsetError as required by Addendum 3.
func XMLRawMethod(owner, name string) *hir.Method {
	optString := hir.T(hir.Optional, hir.T(hir.String))
	result := hir.T(hir.Optional, hir.T(hir.Dynamic))
	xml := hir.V("xml", optString)
	return &hir.Method{Name: name, Virtual: true, Result: result, Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "xml", Type: optString, X: &hir.Expr{Kind: hir.VirtualCall, Name: "getXML", Type: optString, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(owner)}}},
		&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: xml}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Lit, Type: result}})},
		&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "xml.parseSubset", Type: hir.T(hir.Dynamic), X: &hir.Expr{Kind: hir.Narrow, Type: hir.T(hir.String), X: xml}}},
	)}
}
func registryXML() []Entry {
	return []Entry{{ID: "abaplint-registry-xml-subset", Key: Key{"src/objects/_abstract_object.ts", "AbstractObject.parseRaw2", "KindMethodDeclaration"}, SHA256: "09a8dbb883510647db87c9419ddadf5f04861e3e387e928c7ae5e9b5a9c4c9d2", Rationale: "fast-xml-parser 5.10.1 abapGit subset with parseTagValue:false, ignoreAttributes:true, trimValues:false; outside domain raises; missing XML remains undefined", Method: func() *hir.Method { return XMLRawMethod("src/objects/_abstract_object.ts.AbstractObject", "parseRaw2") }}}
}
