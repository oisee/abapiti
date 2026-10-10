package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/internal/hirclone"
)

// arrayGraph is a context-insensitive inclusion analysis. Fields are merged
// by declaring owner; array rows are merged by allocation site. Unknown native
// producers carry explicit unknown rows, never an exact-type proof.
type arrayGraph struct {
	edges  map[string]map[string]bool
	values map[string]map[string]bool
	events map[string][]func(string)
	queue  [][2]string
	types  map[string]hir.Type
	views  map[string]map[string]bool
}

func newArrayGraph() *arrayGraph {
	return &arrayGraph{edges: map[string]map[string]bool{}, values: map[string]map[string]bool{}, events: map[string][]func(string){}, types: map[string]hir.Type{}, views: map[string]map[string]bool{}}
}
func (g *arrayGraph) seed(n, v string) {
	if g.values[n] == nil {
		g.values[n] = map[string]bool{}
	}
	if g.values[n][v] {
		return
	}
	g.values[n][v] = true
	g.queue = append(g.queue, [2]string{n, v})
}
func (g *arrayGraph) edge(a, b string) { // a flows into b
	if a == "" || b == "" {
		return
	}
	if g.edges[a] == nil {
		g.edges[a] = map[string]bool{}
	}
	if g.edges[a][b] {
		return
	}
	g.edges[a][b] = true
	for v := range g.values[a] {
		g.seed(b, v)
	}
}
func (g *arrayGraph) on(n string, f func(string)) {
	g.events[n] = append(g.events[n], f)
	for v := range g.values[n] {
		f(v)
	}
}
func (g *arrayGraph) solve() {
	for i := 0; i < len(g.queue); i++ {
		q := g.queue[i]
		for n := range g.edges[q[0]] {
			g.seed(n, q[1])
		}
		for _, f := range g.events[q[0]] {
			f(q[1])
		}
	}
	g.queue = nil
}
func arrayElem(t hir.Type) (hir.Type, bool) {
	if t.Kind == hir.Optional && len(t.Args) == 1 {
		t = t.Args[0]
	}
	if t.Kind == hir.Array && len(t.Args) == 1 {
		e := t.Args[0]
		if e.Kind == hir.Optional && len(e.Args) == 1 {
			e = e.Args[0]
		}
		return e, e.Kind == hir.ClassRef || e.Kind == hir.InterfaceRef
	}
	return hir.Type{}, false
}
func refBase(t hir.Type) hir.Type {
	if t.Kind == hir.Optional && len(t.Args) == 1 {
		return t.Args[0]
	}
	return t
}

type arraySite struct {
	Caller, Path, Source, Stage, Kind, Element, Row, Receiver, Proof, Classes, Views, Storage string
	Order, Loop, Casts, Removed, Compatible                                                   int
	Hot                                                                                       bool
}
type arrayAnalysis struct {
	g            *arrayGraph
	sites        []*arraySite
	allLoops     []*arraySite
	classes      map[string]*hir.Class
	interfaces   map[string]*hir.Interface
	methods      map[string]*hir.Method
	unknownCalls int
	elementTypes map[string]bool
}

func (a *arrayAnalysis) owner(c, n string) string {
	for a.classes[c] != nil {
		x := a.classes[c]
		for _, f := range x.Fields {
			if f.Name == n {
				return c
			}
		}
		c = x.Super
	}
	return c
}
func (a *arrayAnalysis) targets(e *hir.Expr, current string) []string {
	owner := e.Owner
	if e.X != nil {
		owner = refBase(e.X.Type).Name
	}
	if e.Kind == hir.New {
		owner = e.Type.Name
	}
	if e.Kind == hir.SuperCall {
		owner = a.classes[current].Super
	}
	name := e.Name
	if e.Kind == hir.New {
		name = "constructor"
	}
	resolve := func(c string) string {
		for c != "" {
			id := c + "::" + name
			if a.methods[id] != nil {
				return id
			}
			if a.classes[c] == nil {
				break
			}
			c = a.classes[c].Super
		}
		return ""
	}
	if e.Kind != hir.VirtualCall {
		if t := resolve(owner); t != "" {
			return []string{t}
		}
		return nil
	}
	// Class receivers use their nominal hierarchy. Interface receivers use all
	// declared implementors, including structurally compatible interface views.
	set := map[string]bool{}
	for c := range a.classes {
		if e.X != nil && refBase(e.X.Type).Kind == hir.InterfaceRef && a.interfaces[refBase(e.X.Type).Name] != nil {
			found := false
			for base := c; a.classes[base] != nil; base = a.classes[base].Super {
				for _, iface := range a.classes[base].Implements {
					if a.interfaceFits(iface, refBase(e.X.Type).Name) {
						found = true
					}
				}
			}
			if !found {
				continue
			}
		}
		if e.X != nil && refBase(e.X.Type).Kind == hir.ClassRef && refBase(e.X.Type).Name != hir.RootObject {
			found := false
			for base := c; a.classes[base] != nil; base = a.classes[base].Super {
				if base == refBase(e.X.Type).Name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if t := resolve(c); t != "" {
			set[t] = true
		}
	}
	return sortedKeys(set)
}

func (a *arrayAnalysis) interfaceFits(src, dst string) bool {
	if src == dst {
		return true
	}
	s, d := a.interfaces[src], a.interfaces[dst]
	if s == nil || d == nil {
		return false
	}
	for _, want := range d.Methods {
		found := false
		for _, got := range s.Methods {
			if got.Name != want.Name || !got.Result.Equal(want.Result) || len(got.Params) != len(want.Params) {
				continue
			}
			match := true
			for i, p := range want.Params {
				if p.Name != got.Params[i].Name || !p.Type.Equal(got.Params[i].Type) {
					match = false
				}
			}
			if match {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sortedKeys[V any](m map[string]V) []string {
	r := []string{}
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
func (a *arrayAnalysis) scan(p *hir.Program, dir string) {
	a.elementTypes = map[string]bool{}
	a.classes = map[string]*hir.Class{}
	a.interfaces = map[string]*hir.Interface{}
	for _, i := range p.Interfaces {
		a.interfaces[i.Name] = i
		for _, m := range i.Methods {
			if el, ok := arrayElem(m.Result); ok {
				a.elementTypes[el.String()] = true
			}
			for _, param := range m.Params {
				if el, ok := arrayElem(param.Type); ok {
					a.elementTypes[el.String()] = true
				}
			}
		}
	}
	a.methods = map[string]*hir.Method{}
	for _, c := range p.Classes {
		a.classes[c.Name] = c
		for _, m := range c.Methods {
			a.methods[c.Name+"::"+m.Name] = m
		}
		if c.Ctor != nil {
			a.methods[c.Name+"::constructor"] = c.Ctor
		}
	}
	for _, c := range p.Classes {
		ms := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			ms = append(ms, c.Ctor)
		}
		for _, m := range ms {
			name := m.Name
			if m == c.Ctor {
				name = "constructor"
			}
			caller := c.Name + "::" + name
			env := map[string]string{"this": caller + "/this"}
			a.g.seed(env["this"], "class:"+c.Name)
			for _, p := range m.Params {
				env[p.Name] = caller + "/param/" + p.Name
				if el, ok := arrayElem(p.Type); ok {
					a.elementTypes[el.String()] = true
					binding := env[p.Name]
					a.g.on(binding, func(v string) {
						if strings.HasPrefix(v, "array:") {
							if a.g.views[v] == nil {
								a.g.views[v] = map[string]bool{}
							}
							a.g.views[v][el.String()] = true
						}
					})
				}
			}
			clone := func(env map[string]string) map[string]string {
				x := map[string]string{}
				for k, v := range env {
					x[k] = v
				}
				return x
			}
			add := func(path, source, kind string, t hir.Type, loop int, row hir.Type, recv string) *arraySite {
				el, ok := arrayElem(t)
				if !ok {
					return nil
				}
				storage := "shared object table"
				raw := refBase(t)
				if raw.Kind == hir.Array && raw.Args[0].Kind == hir.Optional {
					storage = "typed optional-reference table"
				}
				s := &arraySite{Storage: storage, Order: len(a.sites), Caller: caller, Path: path, Source: strings.TrimPrefix(source, dir+"/"), Stage: stage(caller), Kind: kind, Element: el.String(), Row: row.String(), Loop: loop, Receiver: recv}
				a.sites = append(a.sites, s)
				return s
			}
			unknown := func(n string, t hir.Type) {
				if el, ok := arrayElem(t); ok {
					token := "array:" + n
					a.g.types[token] = t
					a.g.seed(n, token)
					a.g.seed(token+"/rows", "unknown:"+el.String())
				} else if refBase(t).IsRef() {
					a.g.seed(n, "unknown:"+t.String())
				}
			}
			var ex func(*hir.Expr, string, int, map[string]string) string
			var st func(*hir.Stmt, string, int, map[string]string)
			ex = func(e *hir.Expr, path string, loop int, env map[string]string) string {
				if e == nil {
					return ""
				}
				if el, ok := arrayElem(e.Type); ok {
					a.elementTypes[el.String()] = true
				}
				if e.Kind == hir.Local {
					n := env[e.Name]
					if el, ok := arrayElem(e.Type); ok {
						a.g.on(n, func(v string) {
							if strings.HasPrefix(v, "array:") {
								if a.g.views[v] == nil {
									a.g.views[v] = map[string]bool{}
								}
								a.g.views[v][el.String()] = true
							}
						})
					}
					return n
				}
				if e.Kind == hir.This {
					return env["this"]
				}
				if e.Kind == hir.Seq {
					env = clone(env)
					st(e.Stmt, path+"/seq", loop, env)
				}
				x := ex(e.X, path+"/0", loop, env)
				y := ex(e.Y, path+"/1", loop, env)
				z := ex(e.Z, path+"/2", loop, env)
				args := []string{}
				for i, v := range e.Args {
					args = append(args, ex(v, fmt.Sprintf("%s/arg%d", path, i), loop, env))
				}
				n := path
				switch e.Kind {
				case hir.FieldGet:
					owner := e.Owner
					if owner == "" && e.X != nil {
						owner = refBase(e.X.Type).Name
					}
					n = "field:" + a.owner(owner, e.Name) + "/" + e.Name
				case hir.StaticGet:
					n = "field:" + a.owner(e.Owner, e.Name) + "/" + e.Name
				case hir.Seq:
					a.g.edge(y, n)
				case hir.Cast, hir.Narrow:
					a.g.edge(x, n)
				case hir.Conditional:
					a.g.edge(y, n)
					a.g.edge(z, n)
				case hir.New:
					if _, ok := arrayElem(e.Type); ok {
						token := "array:" + n
						a.g.types[token] = e.Type
						a.g.seed(n, token)
						add(path, e.Source, "new", e.Type, loop, hir.Type{}, "")
					} else if e.Type.Kind == hir.ClassRef {
						a.g.seed(n, "class:"+e.Type.Name)
					}
					fallthrough
				case hir.DirectCall, hir.VirtualCall, hir.SuperCall:
					if e.Kind != hir.New || e.Type.Kind == hir.ClassRef {
						targets := a.targets(e, c.Name)
						if len(targets) == 0 && e.Kind != hir.New {
							unknown(n, e.Type)
							a.unknownCalls++
							for i, arg := range e.Args {
								if el, ok := arrayElem(arg.Type); ok {
									binding := args[i]
									a.g.on(binding, func(v string) {
										if strings.HasPrefix(v, "array:") {
											a.g.seed(v+"/rows", "unknown:"+el.String())
										}
									})
								}
							}
						}
						for _, t := range targets {
							mm := a.methods[t]
							a.g.edge(t+"/return", n)
							if e.Kind == hir.New {
								a.g.edge(n, t+"/this")
							} else {
								if e.Kind == hir.SuperCall {
									a.g.edge(env["this"], t+"/this")
								} else {
									a.g.edge(x, t+"/this")
								}
							}
							for i, v := range args {
								if i < len(mm.Params) {
									a.g.edge(v, t+"/param/"+mm.Params[i].Name)
								}
							}
						}
					}
				case hir.IndexGet:
					if s := add(path, e.Source, "index", e.X.Type, loop, e.Type, x); s != nil {
						if e.Type.IsRef() {
							s.Casts = 1
						}
						a.g.on(x, func(v string) {
							if strings.HasPrefix(v, "array:") {
								a.g.edge(v+"/rows", n)
							} else {
								unknown(n, e.Type)
							}
						})
					}
				case hir.RuntimeOp:
					op := e.Op
					if op == "set.fromArray" && len(e.Args) > 0 {
						if el, ok := arrayElem(e.Args[0].Type); ok {
							if ss := add(path, e.Source, "set.fromArray", e.Args[0].Type, loop+1, el, args[0]); ss != nil {
								ss.Casts = 1
							}
						}
					}
					if op == "array.get" || op == "array.pop" || op == "array.shift" {
						if s := add(path, e.Source, op, e.X.Type, loop, e.Type, x); s != nil {
							s.Casts = 1
						}
						a.g.on(x, func(v string) {
							if strings.HasPrefix(v, "array:") {
								a.g.edge(v+"/rows", n)
							} else {
								unknown(n, e.Type)
							}
						})
					} else if op == "array.push" || op == "array.unshift" || op == "array.splice3" {
						idx := 0
						if op == "array.splice3" {
							idx = 2
						}
						if idx < len(args) {
							a.g.on(x, func(v string) {
								if strings.HasPrefix(v, "array:") {
									a.g.edge(args[idx], v+"/rows")
								}
							})
						}
					}
					if _, ok := arrayElem(e.Type); ok {
						if op == "array.reverse" {
							a.g.edge(x, n)
						} else {
							token := "array:" + n
							a.g.types[token] = e.Type
							a.g.seed(n, token)
							add(path, e.Source, op, e.Type, loop, hir.Type{}, "")
							if op == "array.splice1_view" {
								a.g.edge(x, n)
							}
							if strings.HasPrefix(op, "array.slice") || strings.HasPrefix(op, "array.splice") || op == "array.concat" {
								a.g.on(x, func(v string) { a.g.edge(v+"/rows", token+"/rows") })
								if op == "array.concat" && len(args) > 0 {
									a.g.on(args[0], func(v string) { a.g.edge(v+"/rows", token+"/rows") })
								}
							} else {
								el, _ := arrayElem(e.Type)
								a.g.seed(token+"/rows", "unknown:"+el.String())
							}
						}
					}
				default:
					if e.Kind != hir.Lit {
						unknown(n, e.Type)
					}
				}
				if _, ok := arrayElem(e.Type); ok {
					a.g.on(n, func(v string) {
						if strings.HasPrefix(v, "array:") {
							if a.g.views[v] == nil {
								a.g.views[v] = map[string]bool{}
							}
							el, _ := arrayElem(e.Type)
							a.g.views[v][el.String()] = true
						}
					})
				}
				return n
			}
			st = func(s *hir.Stmt, path string, loop int, env map[string]string) {
				if s == nil {
					return
				}
				if s.Kind == hir.Block {
					for i, b := range s.List {
						blockEnv := env
						if b != nil && b.Kind == hir.Block {
							blockEnv = clone(env)
						}
						st(b, fmt.Sprintf("%s/s%d", path, i), loop, blockEnv)
					}
					return
				}
				x := ""
				indexWrite := s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.IndexGet
				if indexWrite {
					x = ex(s.X.X, path+"/x/0", loop, env)
					ex(s.X.Y, path+"/x/1", loop, env)
				} else {
					x = ex(s.X, path+"/x", loop, env)
				}
				y := ex(s.Y, path+"/y", loop, env)
				switch s.Kind {
				case hir.VarDecl:
					env[s.Name] = path + "/local/" + s.Name
					if el, ok := arrayElem(s.Type); ok {
						a.elementTypes[el.String()] = true
						binding := env[s.Name]
						a.g.on(binding, func(v string) {
							if strings.HasPrefix(v, "array:") {
								if a.g.views[v] == nil {
									a.g.views[v] = map[string]bool{}
								}
								a.g.views[v][el.String()] = true
							}
						})
					}
					a.g.edge(x, env[s.Name])
				case hir.Assign:
					if s.X != nil && s.X.Kind == hir.IndexGet {
						recv := x
						a.g.on(recv, func(v string) { a.g.edge(y, v+"/rows") })
					} else {
						a.g.edge(y, x)
					}
				case hir.Return:
					a.g.edge(x, caller+"/return")
				}
				body := clone(env)
				depth := loop
				if s.Kind == hir.ForEach {
					depth++
					body[s.Name] = path + "/local/" + s.Name
					row := body[s.Name]
					ss := add(path, s.Source, "foreach", s.X.Type, depth, s.Type, x)
					if ss == nil {
						ss = &arraySite{Caller: caller, Path: path, Source: strings.TrimPrefix(s.Source, dir+"/"), Kind: "foreach", Element: s.X.Type.Args[0].String(), Row: s.Type.String(), Loop: depth, Receiver: x, Proof: "value row"}
					}
					if s.Type.IsRef() {
						ss.Casts = 1
					}
					a.allLoops = append(a.allLoops, ss)
					a.g.on(x, func(v string) {
						if strings.HasPrefix(v, "array:") {
							a.g.edge(v+"/rows", row)
						} else {
							unknown(row, s.Type)
						}
					})
				}
				if s.Kind == hir.While {
					depth++
				}
				st(s.Body, path+"/body", depth, body)
				st(s.Else, path+"/else", loop, clone(env))
			}
			st(m.Body, caller+"/body", 0, env)
		}
	}
	a.g.solve()
	for _, s := range a.sites {
		if s.Receiver == "" {
			s.Receiver = "array:" + s.Path
		}
		tokens := a.g.values[s.Receiver]
		if strings.HasPrefix(s.Receiver, "array:") {
			tokens = map[string]bool{s.Receiver: true}
		}
		classes, views := map[string]bool{}, map[string]bool{}
		unknown := len(tokens) == 0
		exact := true
		for v := range tokens {
			if !strings.HasPrefix(v, "array:") {
				unknown = true
				continue
			}
			for view := range a.g.views[v] {
				views[view] = true
			}
			for cl := range a.g.values[v+"/rows"] {
				classes[cl] = true
				if strings.HasPrefix(cl, "unknown:") {
					unknown = true
				}
				if cl != "class:"+strings.TrimSuffix(strings.TrimPrefix(s.Element, "classref<"), ">") {
					exact = false
				}
			}
		}
		s.Classes = strings.Join(sortedKeys(classes), ";")
		s.Views = strings.Join(sortedKeys(views), ";")
		switch {
		case unknown:
			s.Proof = "unknown native/unresolved flow"
		case len(classes) == 0:
			s.Proof = "no non-null rows resolved (not an exact proof)"
		case exact:
			s.Proof = "exact"
		default:
			s.Proof = "hierarchy/mixed upper bound"
		}
		if s.Casts > 0 && s.Row == s.Element || s.Casts > 0 && s.Row == "optional<"+s.Element+">" {
			s.Removed = s.Casts
			if len(tokens) > 0 && len(views) == 1 && views[s.Element] {
				s.Compatible = s.Casts
			}
		}
	}
	sort.Slice(a.sites, func(i, j int) bool { return a.sites[i].Path < a.sites[j].Path })
}

func typedArrays(p *hir.Program, out, dir string) (string, error) {
	// Emit on a clone: the production emitter applies Singleton and Inline.
	// Count both the original lowering and precisely the post-pass HIR emitted today.
	q := hirclone.Clone(p)
	files, names, err := abap.EmitNamed(q)
	if err != nil {
		return "", err
	}
	data, err := abap.SourceMap(q, names)
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(out, "names.json"), data, 0644); err != nil {
		return "", err
	}
	mapping := map[string]abap.SourceEntry{}
	if err = json.Unmarshal(data, &mapping); err != nil {
		return "", err
	}
	hotIDs := map[string]string{}
	suffixes := map[string]string{"Sequence": "B56EAE38", "AlternativePriority": "97ADC9EF", "Expression": "4228F5CC", "Token": "53E10374", "Word": "63FEEC61", "Regex": "DA316F76"}
	var b strings.Builder
	fmt.Fprintln(&b, "Typed reference arrays — full pinned 1538-file closure + RegistryRun")
	fmt.Fprintf(&b, "Production flags: ABAPITI_INLINE=%q ABAPITI_SINGLETON=%q; assume-int=1.\n", os.Getenv("ABAPITI_INLINE"), os.Getenv("ABAPITI_SINGLETON"))
	fmt.Fprintln(&b, "Profile #6 names.json mapping (8-digit hash prefixes resolve to current 14-digit ABAP names):")
	for _, class := range sortedKeys(suffixes) {
		for name, e := range mapping {
			if e.Kind == "class" && strings.Contains(name, "_"+suffixes[class]) {
				if e.ID != "src/abap/2_statements/combi.ts."+class {
					return "", fmt.Errorf("profile hash mapped to unexpected class: %s", e.ID)
				}
				hotIDs[e.ID] = name
				fmt.Fprintf(&b, "%s %s -> %s -> %s\n", class, suffixes[class], name, e.ID)
			}
		}
	}
	if len(hotIDs) != 6 {
		return "", fmt.Errorf("resolved %d of 6 profile IDs", len(hotIDs))
	}
	for _, phase := range []struct {
		name string
		p    *hir.Program
	}{{"lowered", p}, {"emitted", q}} {
		a := &arrayAnalysis{g: newArrayGraph()}
		a.scan(phase.p, dir)
		for _, s := range a.sites {
			s.Hot = hotIDs[strings.Split(s.Caller, "::")[0]] != ""
		}
		if phase.name == "emitted" {
			if err := a.writeHotEvidence(out, hotIDs, files, names); err != nil {
				return "", err
			}
		}
		report, err := a.write(out, phase.name, hotIDs, files, names)
		if err != nil {
			return "", err
		}
		fmt.Fprint(&b, report)
	}
	if err = os.WriteFile(filepath.Join(out, "typed-array-tables.txt"), []byte(b.String()), 0644); err != nil {
		return "", err
	}
	return b.String(), nil
}
func (a *arrayAnalysis) write(out, phase string, hotIDs map[string]string, files map[string]string, names *hir.Names) (string, error) {
	rows := [][]string{{"caller", "site", "source", "stage", "operation", "element_type", "storage_family", "read_type", "loop_depth", "profile_6_class", "casts_today", "casts_removed_same_view", "casts_without_cross_view", "closed_world_proof", "row_class_upper_bound", "array_element_views"}}
	type tally struct{ alloc, reads, casts, removed, compatible, in, out, hot, hotStage, sharedAlloc, sharedRead int }
	types, stages := map[string]*tally{}, map[string]*tally{}
	for el := range a.elementTypes {
		types[el] = &tally{}
	}
	for _, c := range a.classes {
		for _, f := range c.Fields {
			if el, ok := arrayElem(f.Type); ok {
				types[el.String()] = &tally{}
			}
		}
		ms := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			ms = append(ms, c.Ctor)
		}
		for _, m := range ms {
			if el, ok := arrayElem(m.Result); ok {
				types[el.String()] = &tally{}
			}
			for _, p := range m.Params {
				if el, ok := arrayElem(p.Type); ok {
					types[el.String()] = &tally{}
				}
			}
		}
	}
	proofs := map[string]map[string]bool{}
	readSites := []*arraySite{}
	for _, s := range a.sites {
		rows = append(rows, []string{s.Caller, s.Path, s.Source, s.Stage, s.Kind, s.Element, s.Storage, s.Row, strconv.Itoa(s.Loop), strconv.FormatBool(s.Hot), strconv.Itoa(s.Casts), strconv.Itoa(s.Removed), strconv.Itoa(s.Compatible), s.Proof, s.Classes, s.Views})
		if types[s.Element] == nil {
			types[s.Element] = &tally{}
		}
		if stages[s.Stage] == nil {
			stages[s.Stage] = &tally{}
		}
		isRead := s.Kind == "foreach" || s.Kind == "index" || s.Kind == "array.get" || s.Kind == "array.pop" || s.Kind == "array.shift" || s.Kind == "set.fromArray"
		for _, t := range []*tally{types[s.Element], stages[s.Stage]} {
			if isRead {
				if s.Storage == "shared object table" {
					t.sharedRead++
				}
				t.reads++
				t.casts += s.Casts
				t.removed += s.Removed
				t.compatible += s.Compatible
				if s.Stage != "other" {
					t.hotStage++
				}
				if s.Loop > 0 {
					t.in++
				} else {
					t.out++
				}
				if s.Hot {
					t.hot++
				}
			} else {
				t.alloc++
				if s.Storage == "shared object table" {
					t.sharedAlloc++
				}
			}
		}
		if proofs[s.Element] == nil {
			proofs[s.Element] = map[string]bool{}
		}
		proofs[s.Element][s.Proof] = true
		if isRead {
			readSites = append(readSites, s)
		}
	}
	if err := writeCSV(filepath.Join(out, "typed-array-"+phase+"-sites.csv"), rows); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s HIR\n", phase)
	fmt.Fprintln(&b, "Element type | allocations | reads | casts | removable same view | no cross-view | hot-class reads | shared allocations | shared reads | proof categories")
	typeRows := [][]string{{"element_type", "allocations", "reads", "casts", "removable_same_view", "no_cross_view", "hot_class_reads", "shared_allocations", "shared_reads", "proof_categories"}}
	hotTypes, hotStageTypes, allReads, allCasts, allocs, compatible := 0, 0, 0, 0, 0, 0
	for _, k := range sortedKeys(types) {
		t := types[k]
		pr := strings.Join(sortedKeys(proofs[k]), ";")
		fmt.Fprintf(&b, "%s | %d | %d | %d | %d | %d | %d | %d | %d | %s\n", k, t.alloc, t.reads, t.casts, t.removed, t.compatible, t.hot, t.sharedAlloc, t.sharedRead, pr)
		typeRows = append(typeRows, []string{k, strconv.Itoa(t.alloc), strconv.Itoa(t.reads), strconv.Itoa(t.casts), strconv.Itoa(t.removed), strconv.Itoa(t.compatible), strconv.Itoa(t.hot), strconv.Itoa(t.sharedAlloc), strconv.Itoa(t.sharedRead), pr})
		if t.hot > 0 {
			hotTypes++
		}
		if t.hotStage > 0 {
			hotStageTypes++
		}
		compatible += t.compatible
		allReads += t.reads
		allCasts += t.casts
		allocs += t.alloc
	}
	if err := writeCSV(filepath.Join(out, "typed-array-"+phase+"-types.csv"), typeRows); err != nil {
		return "", err
	}
	fmt.Fprintln(&b, "Caller stage | allocations | in-loop reads | outside reads | casts | removable same view | no cross-view")
	for _, k := range sortedKeys(stages) {
		t := stages[k]
		fmt.Fprintf(&b, "%s | %d | %d | %d | %d | %d | %d\n", k, t.alloc, t.in, t.out, t.casts, t.removed, t.compatible)
	}
	runtimeID := "runtime." + hir.T(hir.Array, hir.Ref(hir.RootObject)).String()
	src := files[names.Get(runtimeID)+".clas.abap"]
	lines := strings.Count(src, "\n")
	sharedAlloc, sharedReads := 0, 0
	for _, t := range types {
		sharedAlloc += t.sharedAlloc
		sharedReads += t.sharedRead
	}
	fmt.Fprintf(&b, "Shared object-table subset: %d allocations, %d reads. Other %d allocations/%d reads have Optional reference rows (already typed storage).\n", sharedAlloc, sharedReads, allocs-sharedAlloc, allReads-sharedReads)
	extraTypes := len(types)
	if types[hir.Ref(hir.RootObject).String()] != nil {
		extraTypes--
	}
	fmt.Fprintf(&b, "TOTAL: %d element types, %d allocation sites, %d read sites, %d casts. Hot profile classes use %d element types. Shared object-array class: %d lines. All typed classes: +%d objects (~%d lines), retaining shared fallback; hot-only: +%d objects (~%d lines).\n", len(types), allocs, allReads, allCasts, hotTypes, lines, extraTypes, extraTypes*lines, hotTypes, hotTypes*lines)
	fmt.Fprintf(&b, "Hot pipeline stages (lexer/statements/structures/syntax/rules): %d types (~%d extra lines). %d casts have no observed cross-element view.\n", hotStageTypes, hotStageTypes*lines, compatible)
	fmt.Fprintln(&b, "Profile #6 loops (one ?= per row where casts=1; the narrow-view casts are retained):")
	for _, s := range readSites {
		if s.Hot && s.Kind == "foreach" {
			fmt.Fprintf(&b, "%s | %s | %s | %s | cast/row=%d | removed/row=%d | no-cross-view=%d | %s\n", s.Caller, s.Source, s.Receiver, s.Element, s.Casts, s.Removed, s.Compatible, s.Proof)
		}
	}
	sort.Slice(readSites, func(i, j int) bool {
		x, y := readSites[i], readSites[j]
		if x.Hot != y.Hot {
			return x.Hot
		}
		if x.Removed != y.Removed {
			return x.Removed > y.Removed
		}
		if x.Loop != y.Loop {
			return x.Loop > y.Loop
		}
		return x.Path < y.Path
	})
	fmt.Fprintln(&b, "Top 20 static read sites (profile class, removable casts, loop depth, stable path; no invented execution weights):")
	for i, s := range readSites {
		if i == 20 {
			break
		}
		fmt.Fprintf(&b, "%d | %s | %s | %s | depth=%d | casts=%d removed=%d | %s\n", i+1, s.Caller, s.Source, s.Element, s.Loop, s.Casts, s.Removed, s.Path)
	}
	return b.String(), nil
}

// Cross-check each profiled ForEach against the exact current ABAP LOOP AT
// line and its following cast. Keep these small class sources as review evidence.
func (a *arrayAnalysis) writeHotEvidence(out string, hotIDs map[string]string, files map[string]string, names *hir.Names) error {
	methodNames := map[string]string{"constructor": "constructor"}
	for _, m := range a.methods {
		methodNames[names.Get("member."+m.Name)] = m.Name
	}
	loops := map[string][]*arraySite{}
	ordered := a.allLoops
	for _, s := range ordered {
		if hotIDs[strings.Split(s.Caller, "::")[0]] != "" {
			loops[s.Caller] = append(loops[s.Caller], s)
		}
	}
	rows := [][]string{{"abap_class", "abap_method", "abap_loop_line", "loop_at", "cast_line", "caller", "hir_site", "ts_source", "array_binding", "element_type", "casts_per_row", "potential_removed_per_row", "no_cross_view", "proof"}}
	for _, id := range sortedKeys(hotIDs) {
		name := hotIDs[id]
		src := files[strings.ToLower(name)+".clas.abap"]
		if src == "" {
			return fmt.Errorf("no ABAP evidence for %s", name)
		}
		if err := os.WriteFile(filepath.Join(out, strings.ToLower(name)+".clas.abap"), []byte(src), 0644); err != nil {
			return err
		}
		lines := strings.Split(src, "\n")
		method := ""
		counts := map[string]int{}
		for i, line := range lines {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "METHOD ") {
				method = strings.TrimSuffix(strings.TrimPrefix(trim, "METHOD "), ".")
			}
			if !strings.HasPrefix(trim, "LOOP AT ") || !strings.Contains(trim, "->items") {
				continue
			}
			caller := id + "::" + methodNames[method]
			n := counts[caller]
			if n >= len(loops[caller]) {
				return fmt.Errorf("unmatched profiled ABAP loop %s:%d (%s)", name, i+1, caller)
			}
			s := loops[caller][n]
			counts[caller]++
			cast := ""
			if i+1 < len(lines) && strings.Contains(lines[i+1], " ?= ") {
				cast = strings.TrimSpace(lines[i+1])
			}
			if (cast != "") != (s.Casts > 0) {
				return fmt.Errorf("profiled cast mismatch %s:%d", name, i+1)
			}
			rows = append(rows, []string{name, method, strconv.Itoa(i + 1), trim, cast, caller, s.Path, s.Source, s.Receiver, s.Element, strconv.Itoa(s.Casts), strconv.Itoa(s.Removed), strconv.Itoa(s.Compatible), s.Proof})
		}
		for caller, ls := range loops {
			if strings.HasPrefix(caller, id+"::") && counts[caller] != len(ls) {
				return fmt.Errorf("missing ABAP loops for %s: %d/%d", caller, counts[caller], len(ls))
			}
		}
	}
	runtimeName := names.Get("runtime."+hir.T(hir.Array, hir.Ref(hir.RootObject)).String()) + ".clas.abap"
	if err := os.WriteFile(filepath.Join(out, runtimeName), []byte(files[runtimeName]), 0644); err != nil {
		return err
	}
	return writeCSV(filepath.Join(out, "profile-6-loops.csv"), rows)
}
