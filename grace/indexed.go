package grace

import (
	"math/bits"
	"strconv"
	"strings"
)

// joinPlan is an IR-independent binding program. Each invocation reuses its
// environment and undo stack; failed candidate tuples allocate nothing.
type joinTerm struct {
	slot     int
	constant uint32
}
type joinAtom struct {
	pred     string
	terms    []joinTerm
	mask     uint64
	negative bool
	pivot    bool
}
type joinPlan struct {
	pivot    int
	bound    int
	vars     map[string]int
	head     []joinTerm
	atoms    []joinAtom
	guards   []joinAtom
	env      []uint32
	undo     []int
	output   Tuple
	region   joinTerm
	selected func(string) bool
}

func planJoin(db *DB, c clause, pivot int, initial []term) *joinPlan {
	p := &joinPlan{pivot: pivot, bound: c.bound, vars: map[string]int{}}
	compile := func(a atom) joinAtom {
		out := joinAtom{pred: a.pred, negative: a.negative}
		for _, t := range a.args {
			v := joinTerm{slot: -1}
			if t.variable {
				slot, ok := p.vars[t.value]
				if !ok {
					slot = len(p.vars)
					p.vars[t.value] = slot
				}
				v.slot = slot
			} else if !t.wild {
				v.constant = db.symbols.intern(t.value)
			}
			out.terms = append(out.terms, v)
		}
		return out
	}
	bound := map[string]bool{}
	for _, t := range initial {
		if t.variable {
			bound[t.value] = true
		}
	}
	remaining := map[int]bool{}
	for i, a := range c.body {
		if a.negative || comparison(a.pred) {
			continue
		}
		remaining[i] = true
	}
	for len(remaining) > 0 {
		best, bestScore := -1, 1e300
		// Source order breaks ties deterministically. The delta pivot is consumed
		// first, so a missing delta never scans the stable relations.
		for i, a := range c.body {
			if !remaining[i] {
				continue
			}
			n := 1e6
			if t := db.tables[a.pred]; t != nil {
				n = float64(len(t.rows) + 1)
			}
			mask := uint64(0)
			for col, t := range a.args {
				if !t.wild && (!t.variable || bound[t.value]) && col < 8 {
					mask |= 1 << col
				}
			}
			score := n
			for j := 0; j < bits.OnesCount64(mask); j++ {
				score /= 1000
			}
			if len(p.atoms) == 0 && pivot >= 0 {
				if i == pivot {
					score = -1
				} else {
					score = 1e299
				}
			}
			if score < bestScore {
				best, bestScore = i, score
			}
		}
		a := c.body[best]
		out := compile(a)
		out.pivot = best == pivot
		for col, t := range a.args {
			if col < 8 && !t.wild && (!t.variable || bound[t.value]) {
				out.mask |= 1 << col
			}
		}
		for _, t := range a.args {
			if t.variable {
				bound[t.value] = true
			}
		}
		p.atoms = append(p.atoms, out)
		delete(remaining, best)
	}
	for _, a := range c.body {
		if a.negative || comparison(a.pred) {
			out := compile(a)
			for col, t := range a.args {
				if col < 8 && !t.wild {
					out.mask |= 1 << col
				}
			}
			p.guards = append(p.guards, out)
		}
	}
	p.head = compile(c.head).terms
	if selection, ok := db.selections[c.head.pred]; ok {
		p.region = p.head[selection.Column]
		p.selected = selection.Contains
	}
	p.env = make([]uint32, len(p.vars))
	p.undo = make([]int, 0, len(p.vars))
	p.output = make(Tuple, len(p.head))
	// Index layouts are chosen from the loaded rule bodies, before evaluation.
	for _, a := range append(append([]joinAtom{}, p.atoms...), p.guards...) {
		if !comparison(a.pred) {
			if t := db.tables[a.pred]; t != nil {
				t.ensureIndex(a.mask)
			}
		}
	}
	return p
}

func (t *table) ensureIndex(mask uint64) map[packedTuple][]*row {
	if mask == 0 {
		return nil
	}
	if index := t.joins[mask]; index != nil {
		return index
	}
	index := map[packedTuple][]*row{}
	for _, r := range t.rows {
		k := pack(r.ids, mask)
		index[k] = append(index[k], r)
	}
	t.joins[mask] = index
	return index
}
func (p *joinPlan) value(t joinTerm) uint32 {
	if t.slot >= 0 {
		return p.env[t.slot]
	}
	return t.constant
}
func (p *joinPlan) candidates(db *DB, a joinAtom) []*row {
	t := db.tables[a.pred]
	if t == nil {
		return nil
	}
	if a.mask == 0 {
		return t.rows
	}
	var k packedTuple
	for col, term := range a.terms {
		if col < 8 && a.mask&(1<<col) != 0 {
			k.ids[col] = p.value(term)
		}
	}
	return t.ensureIndex(a.mask)[k]
}
func (p *joinPlan) match(a joinAtom, r *row) bool {
	for col, term := range a.terms {
		if term.slot < 0 && term.constant == 0 {
			continue
		}
		v := p.value(term)
		if v != 0 {
			if v != r.ids[col] {
				return false
			}
		} else {
			p.env[term.slot] = r.ids[col]
			p.undo = append(p.undo, term.slot)
		}
	}
	return true
}
func (p *joinPlan) restore(mark int) {
	for _, slot := range p.undo[mark:] {
		p.env[slot] = 0
	}
	p.undo = p.undo[:mark]
}
func (p *joinPlan) compare(db *DB, a joinAtom) bool {
	if len(a.terms) != 2 {
		return false
	}
	x, y := p.value(a.terms[0]), p.value(a.terms[1])
	if x == 0 || y == 0 {
		return false
	}
	result := false
	switch a.pred {
	case "neq":
		result = x != y
	case "contains":
		result = strings.Contains(db.symbols.values[x], db.symbols.values[y])
	case "le":
		n, e := strconv.Atoi(db.symbols.values[x])
		m, f := strconv.Atoi(db.symbols.values[y])
		if e != nil || f != nil {
			return false
		}
		result = n <= m
	}
	if a.negative {
		result = !result
	}
	return result
}
func (p *joinPlan) run(db, delta *DB, initial map[string]string, emit func(Tuple, int)) {
	clear(p.env)
	p.undo = p.undo[:0]
	for name, value := range initial {
		if slot, ok := p.vars[name]; ok {
			p.env[slot] = db.symbols.intern(value)
		}
	}
	p.walk(db, delta, 0, 0, emit)
}
func (p *joinPlan) walk(db, delta *DB, pos, depth int, emit func(Tuple, int)) {
	if p.selected != nil {
		if id := p.value(p.region); id != 0 && !p.selected(db.symbols.values[id]) {
			return
		}
	}
	if pos == len(p.atoms) {
		for _, a := range p.guards {
			if comparison(a.pred) {
				if !p.compare(db, a) {
					return
				}
				continue
			}
			for _, r := range p.candidates(db, a) {
				mark := len(p.undo)
				found := p.match(a, r)
				p.restore(mark)
				if found {
					return
				}
			}
		}
		depth++
		if p.bound >= 0 && depth > p.bound {
			return
		}
		for i, t := range p.head {
			p.output[i] = db.symbols.values[p.value(t)]
		}
		emit(p.output, depth)
		return
	}
	a := p.atoms[pos]
	source := db
	if a.pivot {
		source = delta
	}
	for _, r := range p.candidates(source, a) {
		mark := len(p.undo)
		if p.match(a, r) {
			n := depth
			if r.depth > n {
				n = r.depth
			}
			p.walk(db, delta, pos+1, n, emit)
		}
		p.restore(mark)
	}
}
