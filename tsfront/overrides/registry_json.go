package overrides

import "github.com/oisee/abapiti/hir"

// StrictJSONProjection is the package adapter boundary, not a native reference
// cast. The runtime retains the entire graph as backing metadata on data shapes.
func StrictJSONProjection(input *hir.Expr, result hir.Type) *hir.Expr {
	graph := &hir.Expr{Kind: hir.RuntimeOp, Op: "json.parseSubset", Type: hir.T(hir.Dynamic), X: input}
	return &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.materialize", Type: result, X: graph}
}
func registryJSON() []Entry {
	return []Entry{{ID: "abaplint-registry-strict-json-constructor", Key: Key{"src/config.ts", "Config", "KindConstructor"}, SHA256: "1df51a230b639a8b5a03152b5d6138c6280a6d47d19614b4c6696a442dc9b4c7", Rationale: "strict JSON package adapter: retain complete tagged graph, project typed config fields, leave TS defaults/version checks live; generalise later", Patterns: &Patterns{
		Statements: map[string]func() *hir.Stmt{"if (JSON5.parse === undefined) {\n      // @ts-ignore\n      JSON5.parse = JSON5.default.parse;\n    }": func() *hir.Stmt { return hir.B() }},
		Expressions: map[string]func() *hir.Expr{"JSON5.parse(json)": func() *hir.Expr {
			return StrictJSONProjection(hir.V("json", hir.T(hir.String)), hir.Ref("src/_config.ts.IConfig"))
		}},
	}}}
}
