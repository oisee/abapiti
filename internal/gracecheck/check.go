package gracecheck

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
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
	if e = rewrite.Evaluate(db, rs); e != nil {
		t.Fatal(e)
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
