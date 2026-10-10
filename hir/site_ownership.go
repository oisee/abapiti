package hir

// ownSiteNodes detaches trees shared by distinct methods before stamping.
// Value bridges intentionally share expressions with their void counterparts;
// their generated operations are identical, but their owning site IDs differ.
// Sharing within a method is preserved. Legacy IDs, sources and values stay
// unchanged; inline origins survive detachment.
func ownSiteNodes(p *Program) {
	exprOwners := map[*Expr]string{}
	stmtOwners := map[*Stmt]string{}
	methodOwners := map[*Method]string{}
	method := func(owner string, m *Method) *Method {
		if m == nil {
			return nil
		}
		owner += "." + m.Name
		if previous, ok := methodOwners[m]; ok && previous != owner {
			clone := *m
			clone.Node = unownedSiteNode(m.Node)
			m = &clone
		}
		methodOwners[m] = owner
		expressions := map[*Expr]*Expr{}
		statements := map[*Stmt]*Stmt{}
		var expr func(*Expr) *Expr
		var stmt func(*Stmt) *Stmt
		expr = func(x *Expr) *Expr {
			if x == nil {
				return nil
			}
			if copy, ok := expressions[x]; ok {
				return copy
			}
			original := x
			if previous, ok := exprOwners[x]; ok && previous != owner {
				copy := *x
				copy.Node = unownedSiteNode(x.Node)
				x = &copy
			}
			expressions[original] = x
			exprOwners[x] = owner
			x.X = expr(x.X)
			x.Y = expr(x.Y)
			x.Z = expr(x.Z)
			x.Stmt = stmt(x.Stmt)
			copied := false
			for i, a := range x.Args {
				child := expr(a)
				if child != a {
					if !copied {
						x.Args = append([]*Expr{}, x.Args...)
						copied = true
					}
					x.Args[i] = child
				}
			}
			return x
		}
		stmt = func(s *Stmt) *Stmt {
			if s == nil {
				return nil
			}
			if copy, ok := statements[s]; ok {
				return copy
			}
			original := s
			if previous, ok := stmtOwners[s]; ok && previous != owner {
				copy := *s
				copy.Node = unownedSiteNode(s.Node)
				s = &copy
			}
			statements[original] = s
			stmtOwners[s] = owner
			s.X = expr(s.X)
			s.Y = expr(s.Y)
			s.Body = stmt(s.Body)
			s.Else = stmt(s.Else)
			copied := false
			for i, c := range s.List {
				child := stmt(c)
				if child != c {
					if !copied {
						s.List = append([]*Stmt{}, s.List...)
						copied = true
					}
					s.List[i] = child
				}
			}
			return s
		}
		m.Body = stmt(m.Body)
		return m
	}
	for _, c := range p.Classes {
		owner := c.SiteOwner
		if owner == "" {
			owner = c.Name
		}
		c.Ctor = method(owner, c.Ctor)
		for i, m := range c.Methods {
			c.Methods[i] = method(owner, m)
		}
	}
	for _, c := range p.Interfaces {
		for i, m := range c.Methods {
			c.Methods[i] = method(c.Name, m)
		}
	}
}

func unownedSiteNode(n Node) Node {
	if len(n.InlinePath) == 0 {
		n.SiteID = ""
		n.SiteSource = ""
		n.SiteOwner = ""
	}
	return n
}
