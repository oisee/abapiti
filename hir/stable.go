package hir

// AssignedLocals returns the names of the locals that evaluating the
// expressions may assign. Expressions write locals only through the
// statements of a Seq. A declaration (VarDecl, ForEach, Try) binds a new
// variable in the Seq's own scope and writes no outer one. Any other local keeps
// its value during the evaluation, so a backend may read the local's own
// storage at the point of use instead of copying it first.
func AssignedLocals(xs ...*Expr) map[string]bool {
	names := map[string]bool{}
	for _, x := range xs {
		assignedInExpr(x, names)
	}
	return names
}

func assignedInExpr(x *Expr, names map[string]bool) {
	if x == nil {
		return
	}
	assignedInStmt(x.Stmt, names)
	assignedInExpr(x.X, names)
	assignedInExpr(x.Y, names)
	assignedInExpr(x.Z, names)
	for _, a := range x.Args {
		assignedInExpr(a, names)
	}
}

func assignedInStmt(s *Stmt, names map[string]bool) {
	if s == nil {
		return
	}
	if s.Kind == Assign && s.X != nil && s.X.Kind == Local {
		names[s.X.Name] = true
	}
	assignedInExpr(s.X, names)
	assignedInExpr(s.Y, names)
	assignedInStmt(s.Body, names)
	assignedInStmt(s.Else, names)
	for _, c := range s.List {
		assignedInStmt(c, names)
	}
}
