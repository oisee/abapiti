package rewrite

import "sort"

// recursiveCalls records only cycle membership. All-pairs call reachability is
// unnecessary for MayDiverge and makes the whole-rule reference prohibitively
// expensive. Receiver selection mirrors the extracted closed-world hierarchy;
// test support checks these seeds independently against Grace's resolved calls.
func (x *extractor) recursiveCalls() {
	if !x.db.Demands("recursive_call") {
		return
	}
	graph := map[string][]string{}
	for _, r := range x.db.Facts("calls") {
		graph[r[0]] = append(graph[r[0]], r[1])
	}
	interfaces := map[string][]string{}
	for _, r := range x.db.Facts("interface_subtype") {
		interfaces[r[0]] = append(interfaces[r[0]], r[1])
	}
	types := map[string]string{}
	for _, r := range x.db.Facts("site_type") {
		types[r[0]] = r[1]
	}
	narrowed := map[string]bool{}
	for _, r := range x.db.Facts("narrowed") {
		narrowed[r[0]] = true
	}
	exact := map[string][]string{}
	for _, r := range x.db.Facts("exact_receiver") {
		exact[r[0]] = append(exact[r[0]], r[1])
	}
	classes := []string{}
	for _, r := range x.db.Facts("concrete") {
		classes = append(classes, r[0])
	}
	accepts := func(class, typ string) bool {
		pending := []string{class}
		seen := map[string]bool{}
		for len(pending) > 0 {
			c := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if seen[c] {
				continue
			}
			seen[c] = true
			if c == typ {
				return true
			}
			if cl := x.classes[c]; cl != nil {
				if cl.Super != "" {
					pending = append(pending, cl.Super)
				}
				pending = append(pending, cl.Implements...)
			}
			pending = append(pending, interfaces[c]...)
		}
		return false
	}
	for _, r := range x.db.Facts("virtual_call") {
		candidates := exact[r[3]]
		if !narrowed[r[3]] {
			candidates = nil
			for _, c := range classes {
				if accepts(c, types[r[3]]) {
					candidates = append(candidates, c)
				}
			}
		}
		for _, c := range candidates {
			if n := x.resolve(c, r[2]); n != "" {
				graph[r[0]] = append(graph[r[0]], n)
			}
		}
	}
	indices := map[string]int{}
	low := map[string]int{}
	active := map[string]bool{}
	stack := []string{}
	serial := 0
	var visit func(string)
	visit = func(v string) {
		serial++
		indices[v] = serial
		low[v] = serial
		stack = append(stack, v)
		active[v] = true
		self := false
		for _, w := range graph[v] {
			if w == v {
				self = true
			}
			if indices[w] == 0 {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if active[w] && indices[w] < low[v] {
				low[v] = indices[w]
			}
		}
		if low[v] != indices[v] {
			return
		}
		component := []string{}
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			active[w] = false
			component = append(component, w)
			if w == v {
				break
			}
		}
		if len(component) > 1 || self {
			for _, m := range component {
				x.add("recursive_call", m)
			}
		}
	}
	nodes := []string{}
	for m := range graph {
		nodes = append(nodes, m)
	}
	sort.Strings(nodes)
	for _, m := range nodes {
		if indices[m] == 0 {
			visit(m)
		}
	}
}
