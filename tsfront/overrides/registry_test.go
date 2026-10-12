package overrides

import (
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestRegistryFingerprint(t *testing.T) {
	key := Key{"src/probe.ts", "Probe.run", "KindNumericLiteral"}
	entry := Entry{ID: "probe", Key: key, SHA256: Fingerprint("42"), Rationale: "fixture", Expression: func() *hir.Expr { return hir.L(hir.T(hir.I32), 42) }}
	r, err := New(entry)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.Lookup(key, "42", "probe.ts:1"); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if _, _, err := r.Lookup(key, "43", "probe.ts:1"); err == nil || !strings.Contains(err.Error(), "override probe is stale: source changed at probe.ts:1") {
		t.Fatal(err)
	}
	wrong := key
	wrong.Symbol = "Other.run"
	if _, ok, _ := r.Lookup(wrong, "42", ""); ok {
		t.Fatal("matched another symbol")
	}
	retired := entry
	retired.FullSource = true
	legacy, err := New(retired)
	if err != nil {
		t.Fatal(err)
	}
	pruned, ok, err := legacy.WithReachability(true).Lookup(key, "42", "probe.ts:1")
	if err != nil || !ok || pruned.Expression == nil {
		t.Fatal("workload adapter changed", ok, err)
	}
	full, ok, err := legacy.WithReachability(false).Lookup(key, "42", "probe.ts:1")
	if err != nil || !ok || full.Expression != nil || full.Assume != "source" {
		t.Fatal("full closure did not retain checked original source", ok, err)
	}
	if _, _, err := legacy.WithReachability(false).Lookup(key, "43", "probe.ts:1"); err == nil {
		t.Fatal("retirement bypassed fingerprint validation")
	}
	if len(r.Inventory()) != 1 {
		t.Fatal("missing inventory")
	}
	if _, err := New(entry, entry); err == nil {
		t.Fatal("accepted duplicate")
	}
}
