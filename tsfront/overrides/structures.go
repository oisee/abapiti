package overrides

import "github.com/oisee/abapiti/hir"

// These adaptations preserve the structure parser's execution paths. The
// result annotation is structurally identical to IStructureResult; selecting
// its named shape avoids introducing an unrelated nominal ABAP class. The
// screen regex is used only for truthiness, so regexp.test has the same result
// as string.match here (no global flag or captures).
func structuresOverrides() []Entry {
	return []Entry{
		{ID: "abaplint-structure-result", Key: Key{"src/abap/3_structures/structure_parser.ts", "StructureParser.runFile", "KindMethodDeclaration"}, SHA256: "01880132c915dd980afca090db0d8a2776722cc86a03116bde366d3c89592505", Rationale: "anonymous result and IStructureResult have identical fields; retain the named result shape without copying or changing identity", Result: func() hir.Type { return hir.Ref("src/abap/3_structures/structure_result.ts.IStructureResult") }},
		{ID: "abaplint-structure-screen-regex", Key: Key{"src/abap/3_structures/structure_parser.ts", "StructureParser.findStructureForFile", "KindMethodDeclaration"}, SHA256: "a66bfe7e235b9b1dcba6fffe2a1833174550b7441ed0c888028541e69e6f2ec9", Rationale: "match result is only tested for truthiness; non-global regex.test preserves the filename routing", Expressions: map[string]func() *hir.Expr{
			`filename.match(/\.screen\_\d+\.abap$/i)`: func() *hir.Expr {
				return &hir.Expr{Kind: hir.RuntimeOp, Op: "regexp.test", Type: hir.T(hir.Bool), X: &hir.Expr{Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(hir.T(hir.String), `\.screen\_\d+\.abap$`), hir.L(hir.T(hir.String), "i")}}, Args: []*hir.Expr{hir.V("filename", hir.T(hir.String))}}
			},
		}},
	}
}
