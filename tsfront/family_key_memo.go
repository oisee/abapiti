package tsfront

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Keys are compositional Merkle ids. Context-free results retain the original
// memo rule: cache only computations that encountered no back-reference.
// Cyclic results additionally record the open back-reference symbols and the
// type checks that observed the caller's stack. Reuse requires those checks to
// agree, including negative checks: a newly open descendant can change an id.
// Checker pointers are private validation data, never part of published ids.
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

type familyKeyContext struct {
	key    string
	checks map[*checker.Type]bool
}

type familyKeyMemo struct {
	contexts  map[familyKeyID]map[string][]familyKeyContext
	frames    []map[*checker.Type]bool
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
		l.familyKeys = &familyKeyMemo{keys: map[familyKeyID]string{}, contexts: map[familyKeyID]map[string][]familyKeyContext{}, refused: map[familyKeyID]familyKeyRefusal{}, diagnosed: map[familyKeyID]bool{}, sentinels: map[familyKeyID]string{}}
	}
	return l.familyKeys
}

func (l *lowerer) familyKeyCached(t *checker.Type, named bool, visiting map[*checker.Type]bool) (key string) {
	m := l.familyKeyState()
	if t == nil || m.off {
		return l.familyTypeKeyRaw(t, named, visiting)
	}
	id := familyKeyID{t, named}
	if m.depth == 0 {
		if r, ok := m.refused[id]; ok {
			return l.familyKeyRefuse(id, r)
		}
		defer func() {
			if p := recover(); p != nil {
				m.depth = 0
				m.frames = nil
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
	if visiting[t] {
		m.observe(t, true)
		m.backs++ // Either a back-reference or a named early return; never cached.
		return l.familyTypeKeyRaw(t, named, visiting)
	}
	if key, ok := m.keys[id]; ok {
		m.observe(t, false)
		return key
	}
	for _, entries := range m.contexts[id] {
		for _, c := range entries {
			matches := true
			for typ, open := range c.checks {
				if visiting[typ] != open {
					matches = false
					break
				}
			}
			if matches {
				m.merge(c.checks)
				m.observe(t, false)
				m.backs++ // A contextual hit must never become context-free upstream.
				return c.key
			}
		}
	}
	if r, ok := m.refused[id]; ok && m.depth > 0 {
		panic(r)
	}

	if m.depth >= familyKeyMaxDepth {
		panic(familyKeyRefusal{id, fmt.Sprintf("is nested deeper than %d levels", familyKeyMaxDepth)})
	}
	m.depth++
	checks := map[*checker.Type]bool{}
	m.frames = append(m.frames, checks)
	before := m.backs
	familyKeyRawEvaluation.Add(1)
	key = l.familyTypeKeyRaw(t, named, visiting)
	m.depth--
	m.frames = m.frames[:len(m.frames)-1]
	delete(checks, t) // References to this node were closed within this computation.
	m.merge(checks)
	m.observe(t, false)
	if len(key) > familyKeyMaxLen {
		panic(familyKeyRefusal{id, fmt.Sprintf("exceeds %d bytes", familyKeyMaxLen)})
	}
	if m.backs == before {
		m.keys[id] = key
	} else {
		symbols := map[string]bool{}
		for typ, open := range checks {
			if open {
				symbols[l.familySymbolKey(typ.Symbol())] = true
			}
		}
		parts := make([]string, 0, len(symbols))
		for symbol := range symbols {
			parts = append(parts, symbol)
		}
		sort.Strings(parts)
		if m.contexts[id] == nil {
			m.contexts[id] = map[string][]familyKeyContext{}
		}
		open := strings.Join(parts, "\x00")
		m.contexts[id][open] = append(m.contexts[id][open], familyKeyContext{key: key, checks: checks})
	}
	return key
}

func (m *familyKeyMemo) observe(t *checker.Type, open bool) {
	if len(m.frames) > 0 {
		m.frames[len(m.frames)-1][t] = open
	}
}

func (m *familyKeyMemo) merge(checks map[*checker.Type]bool) {
	if len(m.frames) > 0 {
		for t, open := range checks {
			m.observe(t, open)
		}
	}
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
