// Package inlineoracle contains the pinned reference implementation and test
// helpers. Production packages must not import it.
package inlineoracle

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
)

// Check compares independent copies, leaving the lowering fixture unchanged.
func Check(t *testing.T, p *hir.Program) {
	t.Helper()
	clone := func() *hir.Program {
		var data bytes.Buffer
		if err := gob.NewEncoder(&data).Encode(p); err != nil {
			t.Fatal(err)
		}
		var q hir.Program
		if err := gob.NewDecoder(&data).Decode(&q); err != nil {
			t.Fatal(err)
		}
		return &q
	}
	reference, grace := clone(), clone()
	count, callees := InlineStats(reference)
	stats, err := rewrite.Inline(grace)
	if err != nil {
		t.Fatal(err)
	}
	want, got := hir.Dump(reference), hir.Dump(grace)
	if want != got {
		a, b := strings.Split(want, "\n"), strings.Split(got, "\n")
		for i := 0; i < len(a) || i < len(b); i++ {
			var x, y string
			if i < len(a) {
				x = a[i]
			}
			if i < len(b) {
				y = b[i]
			}
			if x != y {
				t.Fatalf("Grace oracle rule/action bug: first difference line %d\nreference: %s\nGrace:     %s\ncall sites reference=%d Grace=%d", i+1, x, y, count, stats.CallSites)
			}
		}
	}
	if count != stats.CallSites || !reflect.DeepEqual(callees, stats.Callees) {
		t.Fatalf("Grace oracle rule/action bug: stats reference=(%d,%v) Grace=(%d,%v)", count, callees, stats.CallSites, stats.Callees)
	}
	if errs := hir.Verify(reference); len(errs) > 0 {
		t.Fatalf("reference produced invalid HIR: %v", errs)
	}
	t.Logf("Grace oracle identical: %d call sites; callees=%v", count, callees)
}
