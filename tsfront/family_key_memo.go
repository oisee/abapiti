package tsfront

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// familyTypeKey expands shared subtypes of a type DAG on every path. The memo
// only caches: a key is stored when its computation met no "back:" reference
// at all, so it never depended on the visiting stack and is byte-identical in
// any context. A subtree that produced a back-reference, even one to itself,
// is recomputed: another member of its cycle may be on the stack later.
// Back-references carry only a symbol key, no depth or position.
const (
	familyKeyMaxLen   = 64 << 10
	familyKeyMaxDepth = 256
)

type familyKeyID struct {
	t     *checker.Type
	named bool
}

type familyKeyRefusal struct {
	id     familyKeyID
	reason string
}

type familyKeyMemo struct {
	keys      map[familyKeyID]string
	refused   map[familyKeyID]familyKeyRefusal
	diagnosed map[familyKeyID]bool
	sentinels map[familyKeyID]string
	depth     int
	backs     int
	refusals  int
	node      *ast.Node
	off       bool
}

// familyKeyCrossCheck makes every outermost key computation run again without
// the memo and record a mismatch (tests only).
var (
	familyKeyCrossCheck    atomic.Bool
	familyKeyMismatchMu    sync.Mutex
	familyKeyMismatches    []string
	familyKeyCrossChecked  atomic.Int64
	familyKeyRawEvaluation atomic.Int64
)

func (l *lowerer) familyKeyState() *familyKeyMemo {
	if l.familyKeys == nil {
		l.familyKeys = &familyKeyMemo{keys: map[familyKeyID]string{}, refused: map[familyKeyID]familyKeyRefusal{}, diagnosed: map[familyKeyID]bool{}, sentinels: map[familyKeyID]string{}}
	}
	return l.familyKeys
}

func (l *lowerer) familyKeyCached(t *checker.Type, named bool, visiting map[*checker.Type]bool) (key string) {
	m := l.familyKeyState()
	if t == nil || m.off {
		return l.familyTypeKeyRaw(t, named, visiting)
	}
	id := familyKeyID{t, named}
	if visiting[t] {
		m.backs++ // Either a back-reference or a named early return; never cached.
		return l.familyTypeKeyRaw(t, named, visiting)
	}
	if key, ok := m.keys[id]; ok {
		return key
	}
	if r, ok := m.refused[id]; ok && m.depth > 0 {
		panic(r)
	}
	if m.depth == 0 {
		if r, ok := m.refused[id]; ok {
			return l.familyKeyRefuse(id, r)
		}
		defer func() {
			if p := recover(); p != nil {
				m.depth = 0
				r, ok := p.(familyKeyRefusal)
				if !ok {
					panic(p)
				}
				m.refused[r.id] = r
				key = l.familyKeyRefuse(id, r)
				return
			}
			if familyKeyCrossCheck.Load() {
				l.familyKeyCompare(t, named, key)
			}
		}()
	}
	if m.depth >= familyKeyMaxDepth {
		panic(familyKeyRefusal{id, fmt.Sprintf("is nested deeper than %d levels", familyKeyMaxDepth)})
	}
	m.depth++
	before := m.backs
	familyKeyRawEvaluation.Add(1)
	key = l.familyTypeKeyRaw(t, named, visiting)
	m.depth--
	if len(key) > familyKeyMaxLen {
		panic(familyKeyRefusal{id, fmt.Sprintf("exceeds %d bytes", familyKeyMaxLen)})
	}
	if m.backs == before {
		m.keys[id] = key
	}
	return key
}

// familyKeyRefuse returns a key that is stable per refused type, so cyclic
// registration still terminates, and never equals a structural key.
func (l *lowerer) familyKeyRefuse(id familyKeyID, r familyKeyRefusal) string {
	m := l.familyKeys
	m.refusals++
	m.refused[id] = r
	key, ok := m.sentinels[id]
	if !ok {
		key = fmt.Sprintf("refused:%d", len(m.sentinels))
		m.sentinels[id] = key
	}
	if m.diagnosed[r.id] {
		return key
	}
	m.diagnosed[r.id] = true
	text := l.ck.TypeToString(r.id.t)
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	l.diagf(m.node, "unsupported-type", "recursive family structural key %s for type %s; family lowering refused", r.reason, text)
	return key
}

func (l *lowerer) familyKeyCompare(t *checker.Type, named bool, memo string) {
	m := l.familyKeys
	m.off = true
	plain := l.familyTypeKeyRaw(t, named, map[*checker.Type]bool{})
	m.off = false
	familyKeyCrossChecked.Add(1)
	if plain != memo {
		familyKeyMismatchMu.Lock()
		familyKeyMismatches = append(familyKeyMismatches, fmt.Sprintf("%s (named=%t): memo %q, plain %q", l.ck.TypeToString(t), named, memo, plain))
		familyKeyMismatchMu.Unlock()
	}
}

// familyKeyScope sets the location for key refusals and reports whether a
// key was refused while recursiveFamilyType ran.
type familyKeyScope struct {
	l      *lowerer
	saved  *ast.Node
	before int
}

func (l *lowerer) familyKeyEnter(n *ast.Node) familyKeyScope {
	m := l.familyKeyState()
	s := familyKeyScope{l: l, saved: m.node, before: m.refusals}
	m.node = n
	return s
}

func (s familyKeyScope) refused() bool { return s.l.familyKeys.refusals != s.before }

func (s familyKeyScope) exit(res *hir.Type, handled *bool) {
	s.l.familyKeys.node = s.saved
	if s.refused() {
		*res, *handled = hir.T(hir.Void), true
	}
}
