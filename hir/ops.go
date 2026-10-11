package hir

import "sort"

// UsedRuntimeOps includes constructors, methods, interfaces and Seq statements.
// The first occurrence of each operation supplies actionable source metadata.
func UsedRuntimeOps(p *Program) map[string]Node {
	used := map[string]Node{}
	if p == nil {
		return used
	}
	visit := func(x *Expr) {
		if x.Kind == RuntimeOp {
			if _, ok := used[x.Op]; !ok {
				used[x.Op] = x.Node
			}
		}
	}
	method := func(m *Method) {
		if m != nil {
			walk(m.Body, func(*Stmt) {}, visit)
		}
	}
	for _, c := range p.Classes {
		method(c.Ctor)
		for _, m := range c.Methods {
			method(m)
		}
	}
	for _, i := range p.Interfaces {
		for _, m := range i.Methods {
			method(m)
		}
	}
	return used
}

// CheckRuntimeOps rejects absent emitters before a target writes partial output.
func CheckRuntimeOps(p *Program, target string, supports func(string) bool) []error {
	used := UsedRuntimeOps(p)
	keys := []string{}
	for op := range used {
		keys = append(keys, op)
	}
	sort.Strings(keys)
	var errors []error
	for _, op := range keys {
		if !supports(op) {
			errors = append(errors, Error{Node: used[op], Message: "runtime operation " + op + " has no emitter for selected target " + target})
		}
	}
	return errors
}
