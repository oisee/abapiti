package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Reuse each clause's original checker flow type. Selection is still performed
// once by switchStatement; falling through executes subsequent bodies without
// evaluating their labels again. Direct breaks end this concatenated body.
func (l *lowerer) familySwitchBody(start *ast.Node, clauses []*ast.Node) *hir.Stmt {
	var result []*hir.Stmt
	active := false
	for _, c := range clauses {
		active = active || c == start
		if !active {
			continue
		}
		statements := clauseStatements(c)
		result = append(result, l.stmtList(statements)...)
		for _, s := range c.AsCaseOrDefaultClause().Statements.Nodes {
			if switchClauseEnds(s) {
				return hir.B(result...)
			}
		}
	}
	return hir.B(result...)
}
