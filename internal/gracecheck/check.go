package gracecheck

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/inlineoracle"
)

func Source(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, e := os.ReadFile(filepath.Join(filepath.Dir(file), "../../hir/rewrite/rules", name+".grace"))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func Clone(t *testing.T, p *hir.Program) *hir.Program {
	t.Helper()
	var b bytes.Buffer
	if e := gob.NewEncoder(&b).Encode(p); e != nil {
		t.Fatal(e)
	}
	var q hir.Program
	if e := gob.NewDecoder(&b).Decode(&q); e != nil {
		t.Fatal(e)
	}
	return &q
}
func Equal(t *testing.T, a, b *rewrite.DB) {
	t.Helper()
	preds := map[string]bool{}
	for _, p := range a.Predicates() {
		preds[p] = true
	}
	for _, p := range b.Predicates() {
		preds[p] = true
	}
	for p := range preds {
		if !reflect.DeepEqual(a.Facts(p), b.Facts(p)) {
			t.Fatalf("facts differ for %s\nengine: %v\nreference: %v", p, a.Facts(p), b.Facts(p))
		}
	}
}
func Evaluate(t *testing.T, base *rewrite.DB, source string) *rewrite.DB {
	t.Helper()
	ref, steps, e := Reference(base, source, 1000000000)
	if e != nil {
		t.Fatal(e)
	}
	db := rewrite.NewDB()
	for _, p := range base.Predicates() {
		for _, a := range base.Facts(p) {
			if e := db.Add(p, a...); e != nil {
				t.Fatal(e)
			}
		}
	}
	_, rs, e := rewrite.Parse(source)
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- rewrite.Evaluate(db, rs) }()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("engine fixed-point evaluation exceeded 30s")
	}

	Equal(t, db, ref)
	t.Logf("reference examined %d premises", steps)
	return db
}

// Check runs analysis, repeat evaluation, and multi-round rewrite invariants on
// copies. The pinned oracle still checks the unchanged input independently.
func Check(t *testing.T, p *hir.Program) {
	t.Helper()
	start := time.Now()
	if es := hir.Verify(p); len(es) > 0 {
		t.Fatal(es)
	}
	src := Source(t, "analysis")
	base := rewrite.Extract(p)
	db := Evaluate(t, base, src)
	actual, e := rewrite.Analyze(p)
	if e != nil {
		t.Fatal(e)
	}
	Equal(t, actual, db)
	_, rs, e := rewrite.Parse(src)
	if e != nil {
		t.Fatal(e)
	}
	if e = rewrite.Evaluate(db, rs); e != nil {
		t.Fatal(e)
	}
	Equal(t, db, actual)
	beforeInput := hir.Dump(p)
	rewriteBase, e := rewrite.ExtractRewriteFacts(p)
	if e != nil {
		t.Fatal(e)
	}
	allFacts := Evaluate(t, rewriteBase, src+"\n"+Source(t, "inline"))
	// Check the phased preparation used by Rewrite, in addition to the combined
	// source: Analyze first, then native syntax facts and inline rule evaluation.
	for _, pred := range rewriteBase.Predicates() {
		for _, tuple := range rewriteBase.Facts(pred) {
			if e := actual.Add(pred, tuple...); e != nil {
				t.Fatal(e)
			}
		}
	}
	_, inlineFacts, e := rewrite.Parse(Source(t, "inline"))
	if e != nil {
		t.Fatal(e)
	}
	if e := rewrite.Evaluate(actual, inlineFacts); e != nil {
		t.Fatal(e)
	}
	Equal(t, actual, allFacts)
	Monotonicity(t, rewriteBase, src+"\n"+Source(t, "inline"))
	Permutations(t, p)
	if hir.Dump(p) != beforeInput {
		t.Fatal("fact extraction mutated HIR")
	}
	inlineoracle.Check(t, p)
	Budgets(t, p)
	q := Clone(t, p)
	first, e := rewrite.Inline(q)
	if e != nil {
		t.Fatal(e)
	}
	once := hir.Dump(q)
	second, e := rewrite.Inline(q)
	if e != nil {
		t.Fatal(e)
	}
	if second.CallSites != 0 || hir.Dump(q) != once {
		t.Fatalf("single-pass inline not idempotent: first=%+v second=%+v", first, second)
	}
	_, inline, e := rewrite.Parse(Source(t, "inline"))
	if e != nil {
		t.Fatal(e)
	}
	stats, e := rewrite.Rewrite(q, inline, rewrite.Limits{Rounds: 16})
	if e != nil {
		t.Fatal(e)
	}
	if stats.Rounds >= 16 {
		t.Fatal("inline did not reach a fixed point in 16 rounds")
	}
	if es := hir.Verify(q); len(es) > 0 {
		t.Fatal(es)
	}
	before := hir.Dump(q)
	again, e := rewrite.Inline(q)
	if e != nil {
		t.Fatal(e)
	}
	if again.CallSites != 0 || hir.Dump(q) != before {
		t.Fatal("inline fixed point is not idempotent")
	}
	t.Logf("Grace checks: %s, %d verified rounds", time.Since(start), stats.Rounds)
}

// CanonicalDump ignores declaration storage order, which has no HIR semantics.
func CanonicalDump(p *hir.Program) string {
	sort.Slice(p.Classes, func(i, j int) bool { return p.Classes[i].Name < p.Classes[j].Name })
	sort.Slice(p.Interfaces, func(i, j int) bool { return p.Interfaces[i].Name < p.Interfaces[j].Name })
	for _, i := range p.Interfaces {
		sort.Slice(i.Methods, func(a, b int) bool { return i.Methods[a].Name < i.Methods[b].Name })
	}
	for _, c := range p.Classes {
		sort.Slice(c.Methods, func(i, j int) bool { return c.Methods[i].Name < c.Methods[j].Name })
	}
	return hir.Dump(p)
}

func counts(p *hir.Program) map[string]int {
	out := map[string]int{}
	var stmt func(*hir.Stmt) int
	var expr func(*hir.Expr) int
	expr = func(e *hir.Expr) int {
		if e == nil {
			return 0
		}
		n := 1 + expr(e.X) + expr(e.Y) + expr(e.Z) + stmt(e.Stmt)
		for _, a := range e.Args {
			n += expr(a)
		}
		return n
	}
	stmt = func(s *hir.Stmt) int {
		if s == nil {
			return 0
		}
		n := 1 + expr(s.X) + expr(s.Y) + stmt(s.Body) + stmt(s.Else)
		for _, x := range s.List {
			n += stmt(x)
		}
		return n
	}
	for _, c := range p.Classes {
		ms := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			ms = append(ms, c.Ctor)
		}
		for _, m := range ms {
			out[c.Name+"::"+m.Name] = stmt(m.Body)
		}
	}
	return out
}

// Budgets checks actual node growth across multiple rounds, independently of
// the runner's private counters. Separate limits exercise each rejection path.
func Budgets(t *testing.T, p *hir.Program) {
	t.Helper()
	_, rs, e := rewrite.Parse(Source(t, "inline"))
	if e != nil {
		t.Fatal(e)
	}
	zero := strings.ReplaceAll(Source(t, "inline"), "depth 64", "depth 0")
	_, zeroRules, e := rewrite.Parse(zero)
	if e != nil {
		t.Fatal(e)
	}
	z := Clone(t, p)
	original := hir.Dump(z)
	stats, e := rewrite.Rewrite(z, zeroRules, rewrite.Limits{Rounds: 4})
	if e != nil {
		t.Fatal(e)
	}
	if stats.CallSites != 0 || stats.Rounds != 1 || hir.Dump(z) != original {
		t.Fatal("zero depth changed HIR")
	}
	for _, limits := range []rewrite.Limits{{Rounds: 4, MethodGrowth: 1, ProgramGrowth: 1000000}, {Rounds: 4, MethodGrowth: 100000, ProgramGrowth: 1}, {Rounds: 4, MethodGrowth: 7, ProgramGrowth: 13}} {
		q := Clone(t, p)
		before := counts(q)
		if _, e := rewrite.Rewrite(q, rs, limits); e != nil {
			t.Fatal(e)
		}
		total := 0
		for m, n := range counts(q) {
			growth := n - before[m]
			if growth > limits.MethodGrowth {
				t.Fatalf("%s grew by %d beyond %+v", m, growth, limits)
			}
			if growth > 0 {
				total += growth
			}
		}
		if total > limits.ProgramGrowth {
			t.Fatalf("program grew by %d beyond %+v", total, limits)
		}
	}
}

// Monotonicity uses the positive alternatives of the embedded rules. The full
// stratified source is checked separately: purity and unresolved dispatch use
// negation and necessarily can retract on fresh evaluation after fact growth.
func Monotonicity(t *testing.T, base *rewrite.DB, src string) {
	t.Helper()
	positive, e := PositiveSource(src)
	if e != nil {
		t.Fatal(e)
	}
	_, rs, e := rewrite.Parse(positive)
	if e != nil {
		t.Fatal(e)
	}
	copyDB := func() *rewrite.DB {
		d := rewrite.NewDB()
		for _, p := range base.Predicates() {
			for _, a := range base.Facts(p) {
				if e := d.Add(p, a...); e != nil {
					t.Fatal(e)
				}
			}
		}
		return d
	}
	before, after := copyDB(), copyDB()
	if e := rewrite.Evaluate(before, rs); e != nil {
		t.Fatal(e)
	}
	for _, a := range base.Facts("defined") {
		after.Add("throws", a[0], "monotonicity-added")
		after.Add("writes_static", a[0], "monotonicity-class", "field")
	}
	if e := rewrite.Evaluate(after, rs); e != nil {
		t.Fatal(e)
	}
	for _, p := range before.Predicates() {
		for _, a := range before.Facts(p) {
			if !after.Has(p, a...) {
				t.Fatalf("adding facts removed %s%v", p, a)
			}
		}
	}
}

// Permutations checks declaration storage order. Generated temporary names are
// alpha-normalised because the pinned oracle numbers them in traversal order.
// Executable statements retain source order; arbitrary permutations change HIR
// semantics and are not a valid determinism property.
func Permutations(t *testing.T, p *hir.Program) {
	t.Helper()
	q := Clone(t, p)
	r := rand.New(rand.NewSource(20261009))
	r.Shuffle(len(q.Classes), func(i, j int) { q.Classes[i], q.Classes[j] = q.Classes[j], q.Classes[i] })
	r.Shuffle(len(q.Interfaces), func(i, j int) { q.Interfaces[i], q.Interfaces[j] = q.Interfaces[j], q.Interfaces[i] })
	for _, c := range q.Classes {
		r.Shuffle(len(c.Methods), func(i, j int) { c.Methods[i], c.Methods[j] = c.Methods[j], c.Methods[i] })
	}
	base, e := rewrite.ExtractRewriteFacts(p)
	if e != nil {
		t.Fatal(e)
	}
	other, e := rewrite.ExtractRewriteFacts(q)
	if e != nil {
		t.Fatal(e)
	}
	Equal(t, base, other)
	_, rs, e := rewrite.Parse(Source(t, "analysis") + Source(t, "inline"))
	if e != nil {
		t.Fatal(e)
	}
	if e := rewrite.Evaluate(base, rs); e != nil {
		t.Fatal(e)
	}
	if e := rewrite.Evaluate(other, rs); e != nil {
		t.Fatal(e)
	}
	Equal(t, base, other)
	a := Clone(t, p)
	if _, e := rewrite.Inline(a); e != nil {
		t.Fatal(e)
	}
	if _, e := rewrite.Inline(q); e != nil {
		t.Fatal(e)
	}
	if alphaDump(a) != alphaDump(q) {
		t.Fatal("declaration permutation changed inline output beyond temporary names")
	}
}

var generatedName = regexp.MustCompile(`^x*inl_[0-9]+_`)

func alphaDump(p *hir.Program) string {
	CanonicalDump(p)
	for _, c := range p.Classes {
		ms := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			ms = append(ms, c.Ctor)
		}
		for _, m := range ms {
			names := map[string]string{}
			name := func(s string) string {
				if !generatedName.MatchString(s) {
					return s
				}
				if n, ok := names[s]; ok {
					return n
				}
				n := fmt.Sprintf("temporary_%d", len(names))
				names[s] = n
				return n
			}
			var stmt func(*hir.Stmt)
			var expr func(*hir.Expr)
			expr = func(e *hir.Expr) {
				if e == nil {
					return
				}
				if e.Kind == hir.Local {
					e.Name = name(e.Name)
				}
				expr(e.X)
				expr(e.Y)
				expr(e.Z)
				for _, a := range e.Args {
					expr(a)
				}
				stmt(e.Stmt)
			}
			stmt = func(s *hir.Stmt) {
				if s == nil {
					return
				}
				if s.Kind == hir.VarDecl || s.Kind == hir.ForEach || s.Kind == hir.Try {
					s.Name = name(s.Name)
				}
				expr(s.X)
				expr(s.Y)
				stmt(s.Body)
				stmt(s.Else)
				for _, x := range s.List {
					stmt(x)
				}
			}
			stmt(m.Body)
		}
	}
	return hir.Dump(p)
}
