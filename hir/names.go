package hir

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// Names uses qualified identities, independent of discovery order. A collision
// is rejected rather than resolved according to traversal order.
type Names struct{ byID, byName map[string]string }

func NewNames() *Names { return &Names{map[string]string{}, map[string]string{}} }
func (n *Names) Get(id string) string {
	if s, ok := n.byID[id]; ok {
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
	s = fmt.Sprintf("z_%s_%x", s, h[:7])
	if old, ok := n.byName[s]; ok && old != id {
		panic("HIR name hash collision: " + id)
	}
	n.byID[id] = s
	n.byName[s] = id
	return s
}
