// Package rewrite implements Grace fact analysis and bounded rewrites over HIR.
package rewrite

import (
	"encoding/binary"
	"fmt"
	"sort"
)

type Tuple []string
type row struct {
	args  Tuple
	depth int
	ids   []uint32
}
type table struct {
	rows  []*row
	keys  map[packedTuple]*row
	joins map[uint64]map[packedTuple][]*row
	arity int
}

// DB is a set of memoised relations. Public reads are sorted copies.
type DB struct {
	tables   map[string]*table
	demanded map[string]bool
	symbols  *symbols
}

// Symbols and tuples are IR-independent. Zero is reserved for unbound variables.
type symbols struct {
	ids    map[string]uint32
	values []string
}

func (s *symbols) intern(v string) uint32 {
	if id := s.ids[v]; id != 0 {
		return id
	}
	id := uint32(len(s.values))
	s.ids[v] = id
	s.values = append(s.values, v)
	return id
}

type packedTuple struct {
	ids   [8]uint32
	extra string
}

func pack(ids []uint32, mask uint64) packedTuple {
	var k packedTuple
	for i, id := range ids {
		if i < 8 && mask&(1<<i) != 0 {
			k.ids[i] = id
		}
	}
	if len(ids) > 8 && mask == ^uint64(0) {
		var b []byte
		for _, id := range ids[8:] {
			b = binary.LittleEndian.AppendUint32(b, id)
		}
		k.extra = string(b)
	}
	return k
}
func NewDB() *DB {
	return &DB{tables: map[string]*table{}, symbols: &symbols{ids: map[string]uint32{}, values: []string{""}}}
}
func (d *DB) empty() *DB { return &DB{tables: map[string]*table{}, symbols: d.symbols} }
func (d *DB) put(pred string, args Tuple, depth int) (bool, error) {
	if d.demanded != nil && !d.demanded[pred] {
		return false, nil
	}
	t := d.tables[pred]
	if t == nil {
		t = &table{keys: map[packedTuple]*row{}, joins: map[uint64]map[packedTuple][]*row{}, arity: len(args)}
		d.tables[pred] = t
	}
	if t.arity != len(args) {
		return false, fmt.Errorf("%s: inconsistent arity", pred)
	}
	var small [8]uint32
	ids := small[:]
	if len(args) <= len(small) {
		ids = ids[:len(args)]
	} else {
		ids = make([]uint32, len(args))
	}
	for i, a := range args {
		ids[i] = d.symbols.intern(a)
	}
	k := pack(ids, ^uint64(0))
	if old := t.keys[k]; old != nil {
		if depth < old.depth {
			old.depth = depth
			return true, nil
		}
		return false, nil
	}
	canonical := make(Tuple, len(args))
	for i, id := range ids {
		canonical[i] = d.symbols.values[id]
	}
	r := &row{args: canonical, depth: depth, ids: append([]uint32(nil), ids...)}
	t.keys[k] = r
	t.rows = append(t.rows, r)
	for mask, index := range t.joins {
		k := pack(ids, mask)
		index[k] = append(index[k], r)
	}
	return true, nil
}
func (d *DB) Add(pred string, args ...string) error { _, err := d.put(pred, args, 0); return err }
func (d *DB) Has(pred string, args ...string) bool {
	t := d.tables[pred]
	if t == nil || len(args) != t.arity {
		return false
	}
	var small [8]uint32
	ids := small[:]
	if len(args) <= len(small) {
		ids = ids[:len(args)]
	} else {
		ids = make([]uint32, len(args))
	}
	for i, a := range args {
		ids[i] = d.symbols.ids[a]
		if ids[i] == 0 {
			return false
		}
	}
	return t.keys[pack(ids, ^uint64(0))] != nil
}
func (d *DB) Facts(pred string) []Tuple {
	var out []Tuple
	if t := d.tables[pred]; t != nil {
		for _, r := range t.rows {
			out = append(out, append(Tuple{}, r.args...))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		for k := range out[i] {
			if out[i][k] != out[j][k] {
				return out[i][k] < out[j][k]
			}
		}
		return false
	})
	return out
}
func (d *DB) Predicates() []string {
	var out []string
	for p := range d.tables {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
func (d *DB) Count(pred string) int {
	if t := d.tables[pred]; t != nil {
		return len(t.rows)
	}
	return 0
}
