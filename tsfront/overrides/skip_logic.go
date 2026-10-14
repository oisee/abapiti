package overrides

import "github.com/oisee/abapiti/hir"

// skipLogicGeneratedGroup: isGeneratedFunctionGroup is declared boolean but
// returns the record value, i.e. undefined for an area that has no TOBJ. Its
// only caller tests it in an && condition, where undefined and false agree;
// presence of the key (values are only ever set to true) is that truth value.
func skipLogicGeneratedGroup() []Entry {
	str := hir.T(hir.String)
	records := hir.T(hir.OrderedMap, str, hir.T(hir.Bool))
	return []Entry{
		{ID: "abaplint-skip-logic-generated-group-presence", Key: Key{"src/skip_logic.ts", "SkipLogic.isGeneratedFunctionGroup", "KindMethodDeclaration"}, SHA256: "6cdc80df73777f6dc6b0f87e5c02951800a10903986bc1391849e955ef7b9578",
			Rationale: "the boolean-declared record read yields undefined for a missing area; the only caller (&&) treats it as false, and every stored value is true, so key presence is the result",
			Patterns: &Patterns{Expressions: map[string]func() *hir.Expr{
				"this.tobj[group.getName().toUpperCase()]": func() *hir.Expr {
					this := &hir.Expr{Kind: hir.This, Type: hir.Ref("src/skip_logic.ts.SkipLogic")}
					tobj := &hir.Expr{Kind: hir.FieldGet, Name: "tobj", Type: hir.T(hir.Optional, records), X: this}
					present := &hir.Expr{Kind: hir.Narrow, Type: records, X: tobj}
					name := &hir.Expr{Kind: hir.VirtualCall, Name: "getName", Type: str, X: hir.V("group", hir.Ref("src/objects/function_group.ts.FunctionGroup"))}
					key := &hir.Expr{Kind: hir.RuntimeOp, Op: "string.toUpperCase", Type: str, X: name}
					return &hir.Expr{Kind: hir.RuntimeOp, Op: "map.has", Type: hir.T(hir.Bool), X: present, Args: []*hir.Expr{key}}
				},
			}}},
	}
}
