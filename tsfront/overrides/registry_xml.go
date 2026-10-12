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
	return []Entry{{FullSource: true, ID: "abaplint-registry-xml-subset", Key: Key{"src/objects/_abstract_object.ts", "AbstractObject.parseRaw2", "KindMethodDeclaration"}, SHA256: "09a8dbb883510647db87c9419ddadf5f04861e3e387e928c7ae5e9b5a9c4c9d2", Rationale: "fast-xml-parser 5.10.1 abapGit subset with parseTagValue:false, ignoreAttributes:true, trimValues:false; outside domain raises; missing XML remains undefined", Method: func() *hir.Method { return XMLRawMethod("src/objects/_abstract_object.ts.AbstractObject", "parseRaw2") }}}
}

// XMLValidationResult is the external validator ABI: true or a tagged error
// record. Parsing success is not a substitute for validation.
func XMLValidationResult(input *hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.RuntimeOp, Op: "xml.validate", Type: hir.T(hir.Dynamic), X: input}
}
func XMLValidationMessage(result *hir.Expr) *hir.Expr {
	get := func(value *hir.Expr, key string) *hir.Expr {
		return &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.get", Type: hir.T(hir.Dynamic), X: value, Args: []*hir.Expr{hir.L(hir.T(hir.String), key)}}
	}
	return &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.asString", Type: hir.T(hir.String), X: get(get(result, "err"), "msg")}
}
func XMLValidationFailed(result *hir.Expr) *hir.Expr {
	truth := &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.of", Type: hir.T(hir.Dynamic), X: hir.L(hir.T(hir.Bool), true)}
	equal := &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.strictEquals", Type: hir.T(hir.Bool), X: result, Args: []*hir.Expr{truth}}
	return &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: equal}
}
func registryXMLValidator() []Entry {
	return []Entry{{ID: "abaplint-rules-xml-validator", Key: Key{"src/rules/xml_consistency.ts", "XMLConsistency.run", "KindMethodDeclaration"}, SHA256: "e836d8338f35fa11b0524f912f36e72fd84aeecc82e45b91097274d3706c669a", Rationale: "fast-xml-parser 5.10.1 default validator, preserving true | structured error ABI and Node messages; parser and other rule expressions remain live", Patterns: &Patterns{Expressions: map[string]func() *hir.Expr{
		"XMLValidator.validate(xml)": func() *hir.Expr {
			return XMLValidationResult(&hir.Expr{Kind: hir.Narrow, Type: hir.T(hir.String), X: hir.V("xml", hir.T(hir.Optional, hir.T(hir.String)))})
		},
		"res !== true": func() *hir.Expr { return XMLValidationFailed(hir.V("res", hir.T(hir.Dynamic))) },
		"res.err.msg":  func() *hir.Expr { return XMLValidationMessage(hir.V("res", hir.T(hir.Dynamic))) },
	}}}}
}
