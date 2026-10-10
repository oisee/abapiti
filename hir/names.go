package hir

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// Names uses qualified identities, independent of discovery order. A collision
// is rejected rather than resolved according to traversal order.
type Names struct {
	byID, byName map[string]string
	// fixed holds precomputed readable names (NewReadableNames); fallback is
	// the prefix of hashed names ("z_" by default).
	fixed    map[string]string
	fallback string
}

func NewNames() *Names {
	return &Names{byID: map[string]string{}, byName: map[string]string{}, fallback: "z_"}
}
func (n *Names) Get(id string) string {
	if s, ok := n.byID[id]; ok {
		return s
	}
	if s, ok := n.fixed[id]; ok {
		if old, ok := n.byName[s]; ok && old != id {
			panic("HIR readable name collision: " + id + " and " + old)
		}
		n.byID[id] = s
		n.byName[s] = id
		return s
	}
	var b strings.Builder
	for _, r := range strings.ToLower(id) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > 12 {
		s = s[:12]
	}
	h := sha256.Sum256([]byte(id))
	if n.fallback == "z_" {
		s = fmt.Sprintf("z_%s_%x", s, h[:7])
	} else {
		// keep 30 characters with the longer project prefix
		s = fmt.Sprintf("%s%s_%x", n.fallback, s, h[:(abapNameMax-len(n.fallback)-len(s)-1)/2])
	}
	if old, ok := n.byName[s]; ok && old != id {
		panic("HIR name hash collision: " + id)
	}
	n.byID[id] = s
	n.byName[s] = id
	return s
}

// Pairs lists every assigned identity as (emitted name, qualified id).
func (n *Names) Pairs() map[string]string {
	out := make(map[string]string, len(n.byName))
	for name, id := range n.byName {
		out[name] = id
	}
	return out
}
