// Package overrides holds source-pinned package adaptations. Generic lowering
// knows only this registry; package-specific mappings belong in this directory.
package overrides

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/oisee/abapiti/hir"
)

type Key struct{ File, Symbol, Kind string }
type Entry struct {
	ID        string
	Key       Key
	SHA256    string
	Rationale string
	// Exactly one builder is provided. Builders must return fresh nodes.
	Result      func() hir.Type
	Types       map[string]hir.Type
	Interface   func() *hir.Interface
	Statements  map[string]func() *hir.Stmt
	Expression  func() *hir.Expr
	Body        func() *hir.Stmt
	Method      func() *hir.Method
	Expressions map[string]func() *hir.Expr
}
type Registry struct{ entries map[Key]Entry }

func Fingerprint(span string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(span))) }
func New(entries ...Entry) (*Registry, error) {
	r := &Registry{entries: map[Key]Entry{}}
	ids := map[string]bool{}
	for _, e := range entries {
		if e.ID == "" || e.Key.File == "" || e.Key.Symbol == "" || e.Key.Kind == "" || e.Rationale == "" || len(e.SHA256) != 64 || builderCount(e) != 1 {
			return nil, fmt.Errorf("invalid override %q", e.ID)
		}
		if _, exists := r.entries[e.Key]; exists || ids[e.ID] {
			return nil, fmt.Errorf("duplicate override %s", e.ID)
		}
		ids[e.ID] = true
		r.entries[e.Key] = e
	}
	return r, nil
}
func (r *Registry) Lookup(key Key, span, location string) (Entry, bool, error) {
	if r == nil {
		return Entry{}, false, nil
	}
	e, ok := r.entries[key]
	if !ok {
		return Entry{}, false, nil
	}
	if got := Fingerprint(span); got != e.SHA256 {
		return Entry{}, true, fmt.Errorf("override %s is stale: source changed at %s (got %s, want %s)", e.ID, location, got, e.SHA256)
	}
	return e, true, nil
}
func (r *Registry) Inventory() []Entry {
	var out []Entry
	if r != nil {
		for _, e := range r.entries {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func builderCount(e Entry) int {
	n := 0
	if e.Result != nil {
		n++
	}
	if len(e.Types) > 0 {
		n++
	}
	if e.Interface != nil {
		n++
	}
	if len(e.Statements) > 0 {
		n++
	}
	if e.Expression != nil {
		n++
	}
	if e.Body != nil {
		n++
	}
	if e.Method != nil {
		n++
	}
	if len(e.Expressions) > 0 {
		n++
	}
	return n
}
