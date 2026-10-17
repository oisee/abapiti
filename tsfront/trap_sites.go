package tsfront

import "github.com/oisee/abapiti/hir"

// TrapSite locates one emitted trap statement. Kind is "body" when the trap
// is the whole method body (refused or unexecuted function), "guard" when it
// is the direct branch of a runtime check, and "stmt" otherwise.
type TrapSite struct {
	Class, Method, MethodSource, Source, Kind, Message string
}

// TrapSites lists every trap statement in program order, so step-4 tooling
// can intersect them with workload coverage without building the program.
func TrapSites(p *hir.Program) []TrapSite {
	var sites []TrapSite
	for _, c := range p.Classes {
		for _, m := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
			if m == nil || m.Body == nil {
				continue
			}
			whole := m.Body.Kind == hir.Block && len(m.Body.List) == 1 && m.Body.List[0].Kind == hir.Trap
			var stmt func(s *hir.Stmt, guard bool)
			var expr func(e *hir.Expr)
			stmt = func(s *hir.Stmt, guard bool) {
				if s == nil {
					return
				}
				if s.Kind == hir.Trap {
					kind := "stmt"
					if whole {
						kind = "body"
					} else if guard {
						kind = "guard"
					}
					source := s.Source
					if source == "" {
						source = m.Source
					}
					sites = append(sites, TrapSite{c.Name, m.Name, m.Source, source, kind, s.Name})
				}
				expr(s.X)
				expr(s.Y)
				branch := s.Kind == hir.If
				if branch && s.Body != nil && s.Body.Kind == hir.Block {
					for _, x := range s.Body.List {
						stmt(x, x.Kind == hir.Trap)
					}
				} else {
					stmt(s.Body, branch)
				}
				stmt(s.Else, false)
				for _, x := range s.List {
					stmt(x, false)
				}
			}
			expr = func(e *hir.Expr) {
				if e == nil {
					return
				}
				expr(e.X)
				expr(e.Y)
				expr(e.Z)
				for _, a := range e.Args {
					expr(a)
				}
				stmt(e.Stmt, false)
			}
			stmt(m.Body, false)
		}
	}
	return sites
}
