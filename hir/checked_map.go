package hir

// CheckedMapLayouts conservatively follows storage-equivalent Map views. Since
// a view can cross calls and aliases, every access to an affected storage layout
// must carry the checked adapter's effects, even through a different local.
func CheckedMapLayouts(p *Program) map[string]bool {
	out := map[string]bool{}
	var edges [][2]string
	if p == nil {
		return out
	}
	visitMapExprs(p, func(x *Expr) {
		if x.Kind != RuntimeOp {
			return
		}
		if x.Op == "dynamic.mapView" {
			out[x.Type.String()] = true
		}
		if x.Op == "map.view" && x.X != nil {
			edges = append(edges, [2]string{x.Type.String(), x.X.Type.String()})
		}
	})
	for changed := true; changed; {
		changed = false
		for _, edge := range edges {
			if out[edge[0]] != out[edge[1]] {
				out[edge[0]] = true
				out[edge[1]] = true
				changed = true
			}
		}
	}
	return out
}
func visitMapExprs(p *Program, f func(*Expr)) {
	for _, c := range p.Classes {
		if c == nil {
			continue
		}
		if c.Ctor != nil {
			walk(c.Ctor.Body, func(*Stmt) {}, f)
		}
		for _, m := range c.Methods {
			if m != nil {
				walk(m.Body, func(*Stmt) {}, f)
			}
		}
	}
}

// SealCheckedMapAccess runs after lowering, before verification and fact export.
// Ordinary Map effects stay frozen; erased live values have explicit operations.
func SealCheckedMapAccess(p *Program) {
	layouts := CheckedMapLayouts(p)
	if len(layouts) == 0 {
		return
	}
	visitMapExprs(p, func(x *Expr) {
		if x.Kind == RuntimeOp && x.X != nil && layouts[x.X.Type.String()] {
			switch x.Op {
			case "map.get":
				x.Op = "map.checkedGet"
			case "map.set":
				x.Op = "map.checkedSet"
			case "map.values":
				x.Op = "map.checkedValues"
			}
		}
	})
}
func CheckedMapBaseOp(op string) string {
	switch op {
	case "map.checkedGet":
		return "map.get"
	case "map.checkedSet":
		return "map.set"
	case "map.checkedValues":
		return "map.values"
	}
	return op
}
