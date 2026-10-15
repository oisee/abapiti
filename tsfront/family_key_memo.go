package tsfront

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Family keys are context-free 128-bit ids over the type graph, condensed into
// strongly connected components (SCCs).
//
// Nodes: a type reached with named=true resolves to an alias leaf (its
// arguments are independent key computations), a decl node (interface/class
// symbol plus its arguments) or the type's structural node. Structural nodes
// for named=false and for named=true without a short-circuit are the same
// node, exactly as the #186 traversal treated them. Edges are the child
// references familyDescribe lists (union/intersection members, reference
// arguments, property types).
//
// A node outside every cycle (trivial SCC) gets the #186 Merkle id: the hash of
// its canonical text over its children's ids. #186 spelled an in-cycle
// reference as "back:"+symbol, which made the id depend on the entry point.
// Here every member of a cyclic SCC is labelled with that same symbol key and
// refined to a fixed point (partition refinement over symbol + structure, in-SCC
// children by their current signature). The SCC id hashes the sorted member
// symbol keys and the sorted refined signatures; a member's id hashes the SCC
// id and its own signature. Ids therefore do not depend on the entry member or
// on iteration order, members with equal symbols and bisimilar structure get
// equal ids, and isomorphic cycles over different symbols stay different, as
// their "back:" spellings made them before.
//
// Every id is cached per node. Size and depth guards (#184) remain fail-loud:
// a refused computation publishes nothing.
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

type familyNodeKind uint8

const (
	familyNodeMissing familyNodeKind = iota
	familyNodeAlias
	familyNodeDecl
	familyNodeStruct
)

type familyNode struct {
	t    *checker.Type
	kind familyNodeKind
}

type familyKeyMemo struct {
	keys      map[familyNode]string
	aliases   map[familyNode]bool // alias leaves whose arguments are being keyed
	refused   map[familyKeyID]familyKeyRefusal
	diagnosed map[familyKeyID]bool
	sentinels map[familyKeyID]string
	depth     int
	refusals  int
	node      *ast.Node
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
		l.familyKeys = &familyKeyMemo{keys: map[familyNode]string{}, aliases: map[familyNode]bool{}, refused: map[familyKeyID]familyKeyRefusal{}, diagnosed: map[familyKeyID]bool{}, sentinels: map[familyKeyID]string{}}
	}
	return l.familyKeys
}

func familyHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum[:16])
}

func (l *lowerer) familyResolve(t *checker.Type, named bool) familyNode {
	switch {
	case t == nil:
		return familyNode{nil, familyNodeMissing}
	case named && t.Alias() != nil && l.familySymbolKey(t.Alias().Symbol()) != "":
		return familyNode{t, familyNodeAlias}
	case named && t.Symbol() != nil && t.Symbol().Flags&(ast.SymbolFlagsInterface|ast.SymbolFlagsClass) != 0:
		return familyNode{t, familyNodeDecl}
	}
	return familyNode{t, familyNodeStruct}
}

// familyKeyCached returns the context-free id of (t, named).
func (l *lowerer) familyKeyCached(t *checker.Type, named bool) (key string) {
	m := l.familyKeyState()
	id := familyKeyID{t, named}
	if m.depth == 0 {
		if r, ok := m.refused[id]; ok {
			return l.familyKeyRefuse(id, r)
		}
		defer func() {
			if p := recover(); p != nil {
				m.depth = 0
				clear(m.aliases)
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
			if familyKeyOracle != nil {
				l.familyKeyOracleRecord(t, named, key)
			}
		}()
	} else if r, ok := m.refused[id]; ok {
		panic(r)
	}
	m.depth++
	defer func() { m.depth-- }()
	v := l.familyResolve(t, named)
	if key, ok := m.keys[v]; ok {
		return key
	}
	s := &familySCC{l: l, m: m, index: map[familyNode]int{}, low: map[familyNode]int{}, on: map[familyNode]bool{}, descs: map[familyNode]*familyDesc{}}
	s.visit(v, named)
	return m.keys[v]
}

// familyDesc is one node's canonical text over its children's ids.
type familyDesc struct {
	leaf   string
	kids   []familyNode
	render func(ids []string) string
}

func (d *familyDesc) text(ids []string) string {
	if d.render == nil {
		return d.leaf
	}
	return d.render(ids)
}

// familyDescribe mirrors the #186 traversal (familyTypeKeyMerkle186).
func (l *lowerer) familyDescribe(v familyNode) familyDesc {
	familyKeyRawEvaluation.Add(1)
	t := v.t
	switch v.kind {
	case familyNodeMissing:
		return familyDesc{leaf: "missing"}
	case familyNodeAlias:
		m := l.familyKeys
		if m.aliases[v] {
			panic(familyKeyRefusal{familyKeyID{t, true}, "has an alias argument that contains the alias"})
		}
		m.aliases[v] = true
		defer delete(m.aliases, v)
		return familyDesc{leaf: "alias:" + l.familyAliasKey(t)}
	case familyNodeDecl:
		name := l.familySymbolKey(t.Symbol())
		d := familyDesc{}
		for _, a := range l.familyArguments(t) {
			d.kids = append(d.kids, l.familyResolve(a, true))
		}
		d.render = func(ids []string) string {
			s := "decl:" + name
			for _, id := range ids {
				s += "[" + id + "]"
			}
			return s
		}
		return d
	}
	if t.IsUnion() || t.Flags()&checker.TypeFlagsIntersection != 0 {
		d := familyDesc{}
		for _, p := range t.AsUnionOrIntersectionType().Types() {
			d.kids = append(d.kids, l.familyResolve(p, true))
		}
		flags := t.Flags()
		d.render = func(ids []string) string {
			parts := append([]string(nil), ids...)
			sort.Strings(parts)
			return fmt.Sprintf("%d:%s", flags, strings.Join(parts, "|"))
		}
		return d
	}
	if t.Flags()&checker.TypeFlagsObject != 0 {
		if args := l.familyArguments(t); len(args) > 0 {
			d := familyDesc{}
			for _, a := range args {
				d.kids = append(d.kids, l.familyResolve(a, true))
			}
			name := "array"
			if t.Symbol() != nil {
				name = t.Symbol().Name
			}
			d.render = func(ids []string) string { return name + "<" + strings.Join(ids, ",") + ">" }
			return d
		}
		d := familyDesc{}
		var heads []string
		for _, p := range l.ck.GetPropertiesOfType(t) {
			heads = append(heads, fmt.Sprintf("%q:%t:%t:", p.Name, p.Flags&ast.SymbolFlagsOptional != 0, shapePropertyReadonly(p)))
			d.kids = append(d.kids, l.familyResolve(l.ck.GetTypeOfPropertyOfType(t, p.Name), true))
		}
		d.render = func(ids []string) string {
			parts := make([]string, len(ids))
			for i, id := range ids {
				parts[i] = heads[i] + id
			}
			sort.Strings(parts)
			return "{" + strings.Join(parts, ";") + "}"
		}
		return d
	}
	return familyDesc{leaf: l.ck.TypeToString(t)}
}

// familySCC is one Tarjan traversal over nodes without a cached id.
type familySCC struct {
	l     *lowerer
	m     *familyKeyMemo
	index map[familyNode]int
	low   map[familyNode]int
	on    map[familyNode]bool
	stack []familyNode
	descs map[familyNode]*familyDesc
	next  int
	depth int
}

func (s *familySCC) visit(v familyNode, named bool) {
	if s.depth >= familyKeyMaxDepth {
		panic(familyKeyRefusal{familyKeyID{v.t, named}, fmt.Sprintf("is nested deeper than %d levels", familyKeyMaxDepth)})
	}
	s.depth++
	defer func() { s.depth-- }()
	s.index[v], s.low[v] = s.next, s.next
	s.next++
	s.stack = append(s.stack, v)
	s.on[v] = true
	d := s.l.familyDescribe(v)
	s.descs[v] = &d
	for _, w := range d.kids {
		if _, seen := s.index[w]; seen {
			if s.on[w] {
				s.low[v] = min(s.low[v], s.index[w])
			}
			continue
		}
		if _, ok := s.m.keys[w]; ok {
			continue
		}
		s.visit(w, true)
		s.low[v] = min(s.low[v], s.low[w])
	}
	if s.low[v] != s.index[v] {
		return
	}
	i := len(s.stack) - 1
	for s.stack[i] != v {
		i--
	}
	members := append([]familyNode(nil), s.stack[i:]...)
	s.stack = s.stack[:i]
	for _, w := range members {
		s.on[w] = false
	}
	s.assign(members)
}

func (s *familySCC) checked(v familyNode, text string) string {
	if len(text) > familyKeyMaxLen {
		panic(familyKeyRefusal{familyKeyID{v.t, v.kind != familyNodeStruct}, fmt.Sprintf("exceeds %d bytes", familyKeyMaxLen)})
	}
	return text
}

func (s *familySCC) assign(members []familyNode) {
	if len(members) == 1 {
		v := members[0]
		d := s.descs[v]
		loop := false
		ids := make([]string, len(d.kids))
		for i, w := range d.kids {
			if w == v {
				loop = true
				break
			}
			ids[i] = s.m.keys[w]
		}
		if !loop {
			s.m.keys[v] = familyHash(s.checked(v, d.text(ids)))
			return
		}
	}
	slot := make(map[familyNode]int, len(members))
	syms := make([]string, len(members))
	for i, v := range members {
		slot[v] = i
		syms[i] = fmt.Sprintf("%d:%s", v.kind, s.l.familySymbolKey(v.t.Symbol()))
	}
	round := func(sig []string) []string {
		next := make([]string, len(members))
		for i, v := range members {
			d := s.descs[v]
			ids := make([]string, len(d.kids))
			for j, w := range d.kids {
				if k, ok := slot[w]; ok {
					ids[j] = "in:"
					if sig != nil {
						ids[j] += sig[k]
					}
				} else {
					ids[j] = s.m.keys[w]
				}
			}
			own := syms[i]
			if sig != nil {
				own = sig[i]
			}
			next[i] = familyHash(own + "\x00" + s.checked(v, d.text(ids)))
		}
		return next
	}
	classes := func(sig []string) int {
		seen := make(map[string]bool, len(sig))
		for _, x := range sig {
			seen[x] = true
		}
		return len(seen)
	}
	// Members with equal symbol keys (type arguments included, via their ids)
	// whose structure is bisimilar get one signature and so one id: they are
	// the same type. #186 could split them only by the entry context, and a
	// positional discriminator would bring that entry dependence back.
	sig := round(nil)
	for n := classes(sig); ; {
		next := round(sig)
		sig = next
		c := classes(sig)
		if c == n {
			break
		}
		n = c
	}
	sortedSyms := append([]string(nil), syms...)
	sort.Strings(sortedSyms)
	sortedSigs := append([]string(nil), sig...)
	sort.Strings(sortedSigs)
	scc := familyHash("scc\x00" + strings.Join(sortedSyms, "\x01") + "\x00" + strings.Join(sortedSigs, "\x01"))
	for i, v := range members {
		s.m.keys[v] = familyHash("scc-member\x00" + scc + "\x00" + sig[i])
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

// familyKeyCompare recomputes an id from an empty cache: the straightforward
// SCC computation without reuse of earlier results.
func (l *lowerer) familyKeyCompare(t *checker.Type, named bool, memo string) {
	m := l.familyKeys
	saved := m.keys
	m.keys = map[familyNode]string{}
	m.depth++
	plain := func() (key string) {
		defer func() {
			if p := recover(); p != nil {
				key = fmt.Sprint("panic: ", p)
			}
		}()
		s := &familySCC{l: l, m: m, index: map[familyNode]int{}, low: map[familyNode]int{}, on: map[familyNode]bool{}, descs: map[familyNode]*familyDesc{}}
		v := l.familyResolve(t, named)
		s.visit(v, named)
		return m.keys[v]
	}()
	m.depth--
	m.keys = saved
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
