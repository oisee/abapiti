package rewrite

import "github.com/oisee/abapiti/hir"

type inlineAction struct{}

func newInlineAction(*runner) *inlineAction { return &inlineAction{} }
func (*runner) addInlineFacts() error       { return nil }
func (*inlineAction) expand(*hir.Expr, *hir.Stmt, *hir.Method, int, int) (*hir.Expr, *hir.Stmt) {
	return nil, nil
}
