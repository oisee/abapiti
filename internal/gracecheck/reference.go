// Package gracecheck is independent, deliberately slow test support for Grace.
// It uses repeated whole-rule scans, never engine joins or deltas. Reference
// uses Cartesian joins; ReferenceFull uses independently built hash projections
// for large closures. Production code must not import it.
package gracecheck

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir/rewrite"
)

type node struct {
	text     string
	quoted   bool
	children []node
}
type atom struct {
	pred     string
	terms    []node
	negative bool
}
type clause struct {
	head  atom
	body  []atom
	bound int
}
type fact struct {
	pred  string
	args  []string
	depth int
}

func parse(src string) ([]clause, error) {
	pos := 0
	skip := func() {
		for pos < len(src) {
			if strings.ContainsRune(" \t\r\n", rune(src[pos])) {
				pos++
				continue
			}
			if src[pos] == ';' {
				for pos < len(src) && src[pos] != '\n' {
					pos++
				}
				continue
			}
			break
		}
	}
	var read func() (node, error)
	read = func() (node, error) {
		skip()
		if pos == len(src) {
			return node{}, fmt.Errorf("unexpected EOF")
		}
		if src[pos] == '(' {
			pos++
			n := node{}
			for {
				skip()
				if pos == len(src) {
					return n, fmt.Errorf("unclosed list")
				}
				if src[pos] == ')' {
					pos++
					return n, nil
				}
				child, e := read()
				if e != nil {
					return n, e
				}
				n.children = append(n.children, child)
			}
		}
		start := pos
		if src[pos] == '"' {
			pos++
			for pos < len(src) {
				if src[pos] == '\\' {
					pos += 2
					continue
				}
				if src[pos] == '"' {
					pos++
					v, e := strconv.Unquote(src[start:pos])
					return node{text: v, quoted: true}, e
				}
				pos++
			}
			return node{}, fmt.Errorf("unclosed string")
		}
		for pos < len(src) && !strings.ContainsRune("() \t\r\n;", rune(src[pos])) {
			pos++
		}
		if start == pos {
			return node{}, fmt.Errorf("unexpected delimiter")
		}
		return node{text: src[start:pos]}, nil
	}
	asAtom := func(n node) atom {
		neg := false
		if n.children[0].text == "not" {
			neg = true
			n = n.children[1]
		}
		return atom{n.children[0].text, n.children[1:], neg}
	}
	var out []clause
	for {
		skip()
		if pos == len(src) {
			break
		}
		n, e := read()
		if e != nil {
			return nil, e
		}
		if n.children[0].text != "rule" {
			continue
		}
		c := clause{bound: -1}
		for _, sec := range n.children[3:] {
			switch sec.children[0].text {
			case "head":
				c.head = asAtom(sec.children[1])
			case "bound":
				c.bound, _ = strconv.Atoi(sec.children[2].text)
			}
		}
		for _, sec := range n.children[3:] {
			if sec.children[0].text == "base" || sec.children[0].text == "tail" {
				cc := c
				for _, a := range sec.children[1:] {
					cc.body = append(cc.body, asAtom(a))
				}
				out = append(out, cc)
			}
		}
	}
	return out, nil
}
func variable(n node) bool     { return !n.quoted && strings.HasPrefix(n.text, "?") }
func wildcard(n node) bool     { return !n.quoted && n.text == "_" }
func comparison(p string) bool { return p == "le" || p == "neq" || p == "contains" }
func match(a atom, f fact, env map[string]string) (map[string]string, bool) {
	if a.pred != f.pred || len(a.terms) != len(f.args) {
		return nil, false
	}
	for i, n := range a.terms {
		if wildcard(n) {
			continue
		}
		if variable(n) {
			if v, ok := env[n.text]; ok && v != f.args[i] {
				return nil, false
			}
		} else if n.text != f.args[i] {
			return nil, false
		}
	}
	e := map[string]string{}
	for k, v := range env {
		e[k] = v
	}
	for i, n := range a.terms {
		if wildcard(n) {
			continue
		}
		if variable(n) {
			if v, ok := e[n.text]; ok && v != f.args[i] {
				return nil, false
			}
			e[n.text] = f.args[i]
		} else if n.text != f.args[i] {
			return nil, false
		}
	}
	return e, true
}

// Reference returns the full fixed point and counts premise/guard examinations.
// cap bounds actual work, including joins, so a regression cannot spin forever.
func Reference(base *rewrite.DB, source string, cap int) (*rewrite.DB, int, error) {
	return reference(base, source, cap, false)
}

// ReferenceFull keeps the same whole-rule fixed-point algorithm and parser as
// Reference, but rebuilds hash projections from each round's complete snapshot.
// It shares no parser, join, indexes, stratification or deltas with the engine.
func ReferenceFull(base *rewrite.DB, source string, cap int) (*rewrite.DB, int, error) {
	return methodFlowReference(base, source, cap)
}

func reference(base *rewrite.DB, source string, cap int, hashed bool) (*rewrite.DB, int, error) {
	cs, err := parse(source)
	if err != nil {
		return nil, 0, err
	}
	levels := map[string]int{}
	for _, c := range cs {
		levels[c.head.pred] = 0
		for _, a := range c.body {
			levels[a.pred] = 0
		}
	}
	for i := 0; i <= len(levels); i++ {
		changed := false
		for _, c := range cs {
			for _, a := range c.body {
				n := levels[a.pred]
				if a.negative && !comparison(a.pred) {
					n++
				}
				if levels[c.head.pred] < n {
					levels[c.head.pred] = n
					changed = true
				}
			}
		}
		if !changed {
			break
		}
		if i == len(levels) {
			return nil, 0, fmt.Errorf("negative cycle")
		}
	}
	var facts []fact
	for _, p := range base.Predicates() {
		for _, a := range base.Facts(p) {
			facts = append(facts, fact{p, a, 0})
		}
	}
	identity := func(f fact) string { b, _ := json.Marshal(append([]string{f.pred}, f.args...)); return string(b) }
	members := map[string]int{}
	for i, f := range facts {
		members[identity(f)] = i
	}
	steps := 0
	tick := func() bool { steps++; return steps <= cap }
	max := 0
	for _, l := range levels {
		if l > max {
			max = l
		}
	}
	for level := 0; level <= max; level++ {
		for {
			relations := map[string][]fact{}
			for _, f := range facts {
				relations[f.pred] = append(relations[f.pred], f)
			}
			// A projection uses all currently bound columns, rather than the engine's
			// per-column candidate selection. Build it lazily once per snapshot.
			projections := map[string]map[string][]fact{}
			candidates := func(a atom, env map[string]string) []fact {
				if !hashed {
					return relations[a.pred]
				}
				var columns []int
				var values []string
				for i, n := range a.terms {
					if wildcard(n) {
						continue
					}
					v := n.text
					if variable(n) {
						var ok bool
						v, ok = env[n.text]
						if !ok {
							continue
						}
					}
					columns = append(columns, i)
					values = append(values, v)
				}
				if len(columns) == 0 {
					return relations[a.pred]
				}
				mask, _ := json.Marshal(columns)
				id := a.pred + string(mask)
				projection, ok := projections[id]
				if !ok {
					projection = map[string][]fact{}
					for _, f := range relations[a.pred] {
						if !tick() {
							return nil
						}
						var keyValues []string
						for _, col := range columns {
							keyValues = append(keyValues, f.args[col])
						}
						key, _ := json.Marshal(keyValues)
						projection[string(key)] = append(projection[string(key)], f)
					}
					projections[id] = projection
				}
				key, _ := json.Marshal(values)
				return projection[string(key)]
			}

			var additions []fact
			for _, c := range cs {
				if levels[c.head.pred] != level {
					continue
				}
				// Every clause is evaluated anew every round. Cartesian mode uses
				// static order; full mode chooses the smallest bound projection.
				c.body = append([]atom(nil), c.body...)
				if !hashed {
					sort.SliceStable(c.body, func(i, j int) bool { return len(relations[c.body[i].pred]) < len(relations[c.body[j].pred]) })
				}
				var walk func(int, map[string]string, int)
				walk = func(pos int, env map[string]string, depth int) {
					if steps > cap {
						return
					}
					if pos < len(c.body) {
						if hashed {
							// Choose the smallest complete-snapshot projection under the
							// current bindings, avoiding unrelated Cartesian products.
							best, size := pos, int(^uint(0)>>1)
							for i := pos; i < len(c.body); i++ {
								a := c.body[i]
								if a.negative || comparison(a.pred) {
									continue
								}
								n := len(candidates(a, env))
								if n < size {
									best, size = i, n
								}
							}
							c.body[pos], c.body[best] = c.body[best], c.body[pos]
							defer func() { c.body[pos], c.body[best] = c.body[best], c.body[pos] }()
						}

						a := c.body[pos]
						if a.negative || comparison(a.pred) {
							walk(pos+1, env, depth)
							return
						}
						for _, f := range candidates(a, env) {
							if !tick() {
								return
							}
							if e, ok := match(a, f, env); ok {
								d := depth
								if f.depth > d {
									d = f.depth
								}
								walk(pos+1, e, d)
							}
						}
						return
					}
					for _, a := range c.body {
						if comparison(a.pred) {
							v := []string{}
							for _, n := range a.terms {
								x := n.text
								if variable(n) {
									x = env[x]
								}
								v = append(v, x)
							}
							ok := false
							switch a.pred {
							case "le":
								x, e := strconv.Atoi(v[0])
								y, f := strconv.Atoi(v[1])
								if e != nil || f != nil {
									return
								}
								ok = x <= y
							case "neq":
								ok = v[0] != v[1]
							case "contains":
								ok = strings.Contains(v[0], v[1])
							}
							if a.negative {
								ok = !ok
							}
							if !ok {
								return
							}
						} else if a.negative {
							for _, f := range candidates(a, env) {
								if !tick() {
									return
								}
								if _, ok := match(a, f, env); ok {
									return
								}
							}
						}
					}
					depth++
					if c.bound >= 0 && depth > c.bound {
						return
					}
					f := fact{pred: c.head.pred, depth: depth}
					for _, n := range c.head.terms {
						v := n.text
						if variable(n) {
							v = env[v]
						}
						f.args = append(f.args, v)
					}
					additions = append(additions, f)
				}
				walk(0, map[string]string{}, 0)
			}
			if steps > cap {
				return nil, steps, fmt.Errorf("reference step cap %d exceeded", cap)
			}
			changed := false
			for _, a := range additions {
				k := identity(a)
				if i, ok := members[k]; ok {
					if a.depth < facts[i].depth {
						facts[i].depth = a.depth
						changed = true
					}
				} else {
					members[k] = len(facts)
					facts = append(facts, a)
					changed = true
				}
			}

			if !changed {
				break
			}
		}
	}
	out := rewrite.NewDB()
	for _, f := range facts {
		if err := out.Add(f.pred, f.args...); err != nil {
			return nil, steps, err
		}
	}
	return out, steps, nil
}

// PositiveSource keeps the positive alternatives of the same parsed Grace
// rules. Negated comparisons are value filters and remain monotone. Removing
// relational negation makes the subset invariant under arbitrary fact growth.
func PositiveSource(source string) (string, error) {
	cs, e := parse(source)
	if e != nil {
		return "", e
	}
	var b strings.Builder
	writeAtom := func(a atom) string {
		var s strings.Builder
		if a.negative {
			s.WriteString("(not ")
		}
		s.WriteString("(" + a.pred)
		for _, n := range a.terms {
			s.WriteByte(' ')
			if n.quoted {
				s.WriteString(strconv.Quote(n.text))
			} else {
				s.WriteString(n.text)
			}
		}
		s.WriteByte(')')
		if a.negative {
			s.WriteByte(')')
		}
		return s.String()
	}
	for i, c := range cs {
		ok := true
		for _, a := range c.body {
			if a.negative && !comparison(a.pred) {
				ok = false
			}
		}
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "(rule positive%d 0 (head %s) (base", i, writeAtom(c.head))
		for _, a := range c.body {
			b.WriteByte(' ')
			b.WriteString(writeAtom(a))
		}
		b.WriteByte(')')
		if c.bound >= 0 {
			fmt.Fprintf(&b, " (bound depth %d)", c.bound)
		}
		b.WriteString(")\n")
	}
	return b.String(), nil
}
