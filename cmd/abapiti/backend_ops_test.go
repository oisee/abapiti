package main

import (
	_ "embed"
	"sort"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront"
)

//go:embed testdata/default-runtime-ops.txt
var defaultRuntimeOps string

// Runs in the normal short gate: this is the actual pruned zabapgit lowering,
// not a synthetic sample or the full closure without reachability.
func TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	src, err := embeddedAbaplint(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	closure := tsfront.EmbeddedRegistryClosure()
	if err := materializeClosure(src, closure, work); err != nil {
		t.Fatal(err)
	}
	registry, err := tsfront.RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := tsfront.EmbeddedReachability()
	if err != nil {
		t.Fatal(err)
	}
	lowering, err := tsfront.LowerRegistry(work, append(closure.Files(), tsfront.RegistryHarnessPath), registry, coverage)
	if err != nil {
		t.Fatal(err)
	}
	if err := lowering.Err(); err != nil {
		t.Fatal(err)
	}
	if err := checkTargetRuntimeOps(lowering.Prog, map[string]bool{"go": true, "a4h": true, "osg": true, "native": true}); err != nil {
		t.Fatalf("DEFAULT ZABAPGIT BACKEND COVERAGE REGRESSION:\n%v", err)
	}
	ops := []string{}
	for op := range hir.UsedRuntimeOps(lowering.Prog) {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	t.Log("default ops:", strings.Join(ops, " "))
	allowed := map[string]bool{}
	for _, op := range strings.Fields(defaultRuntimeOps) {
		allowed[op] = true
	}
	for _, op := range ops {
		if !allowed[op] {
			t.Fatalf("DEFAULT ZABAPGIT OP SET CHANGED: %s must be reviewed for every backend before updating testdata/default-runtime-ops.txt", op)
		}
	}
}

func TestSelectedTargetRuntimeOpsFailLoudly(t *testing.T) {
	op := &hir.Expr{Node: hir.Node{ID: 42, Source: "probe.ts:3"}, Kind: hir.RuntimeOp, Op: "new.goOnly", X: hir.L(hir.T(hir.I32), 0), Type: hir.T(hir.I32)}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Ctor: &hir.Method{Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.Seq, Stmt: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: op})}})}}}}
	for _, target := range []string{"go", "a4h", "osg", "native"} {
		err := checkTargetRuntimeOps(p, map[string]bool{target: true})
		if err == nil || !strings.Contains(err.Error(), "new.goOnly") || !strings.Contains(err.Error(), target) || !strings.Contains(err.Error(), "probe.ts:3") {
			t.Fatalf("%s silently accepted missing emitter: %v", target, err)
		}
	}
}
