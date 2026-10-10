// Package rewrite implements Grace fact analysis and bounded rewrites over HIR.
package rewrite

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Tuple []string
type row struct {
	args  Tuple
	depth int
}
type table struct {
	rows  []*row
	keys  map[string]*row
	index []map[string][]*row
}

// DB is a set of memoised relations. Public reads are sorted copies.
type DB struct{ tables map[string]*table }

func NewDB() *DB { return &DB{tables: map[string]*table{}} }
func key(args Tuple) string {
	if args == nil {
		args = Tuple{}
	}
	b, _ := json.Marshal(args)
	return string(b)
}
func (d *DB) put(pred string, args Tuple, depth int) (bool, error) {
	t := d.tables[pred]
	if t == nil {
		t = &table{keys: map[string]*row{}, index: make([]map[string][]*row, len(args))}
		for i := range t.index {
			t.index[i] = map[string][]*row{}
		}
		d.tables[pred] = t
	}
	if len(t.index) != len(args) {
		return false, fmt.Errorf("%s: inconsistent arity", pred)
	}
	k := key(args)
	if old := t.keys[k]; old != nil {
		if depth < old.depth {
			old.depth = depth
			return true, nil
		}
		return false, nil
	}
	r := &row{append(Tuple{}, args...), depth}
	t.keys[k] = r
	t.rows = append(t.rows, r)
	for i, a := range args {
		t.index[i][a] = append(t.index[i][a], r)
	}
	return true, nil
}
func (d *DB) Add(pred string, args ...string) error { _, err := d.put(pred, args, 0); return err }
func (d *DB) Has(pred string, args ...string) bool {
	t := d.tables[pred]
	return t != nil && t.keys[key(args)] != nil
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
