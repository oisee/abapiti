package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func initializerFixture() (*hir.Class, *hir.Method) {
	i32 := hir.T(hir.I32)
	set := func(id int, field string) *hir.Stmt {
		return &hir.Stmt{Node: hir.Node{ID: id}, Kind: hir.Assign, X: &hir.Expr{Node: hir.Node{ID: id + 1}, Kind: hir.StaticGet, Owner: "m.module", Name: field, Type: i32}, Y: &hir.Expr{Node: hir.Node{ID: id + 2}, Kind: hir.Lit, Type: i32, Value: 1}}
	}
	init := &hir.Method{Node: hir.Node{ID: 1}, Name: "class_constructor", Static: true, Result: hir.T(hir.Void), Body: hir.B(set(10, "a"), set(20, "b"), set(30, "c"), set(40, "a"))}
	c := &hir.Class{Name: "m.module", Fields: []hir.Field{{Name: "a", Type: i32, Static: true}, {Name: "b", Type: i32, Static: true}, {Name: "c", Type: i32, Static: true}, {Name: "d", Type: i32, Static: true}}, Methods: []*hir.Method{init}}
	return c, init
}

// A refusal inside one top-level statement of a module initializer traps at
// that statement: earlier statements stay, the unreachable rest goes, and the
// initializer is marked so its failure is rethrown on every later access.
func TestTrapModuleInitializerAtFirstRefusedStatement(t *testing.T) {
	_, init := initializerFixture()
	trapBody(init, []bodyRefusal{{32, -1, "node 32 (m.ts:3:1): later"}, {21, -1, "node 21 (m.ts:2:1): first"}})
	list := init.Body.List
	if len(list) != 2 || list[0].ID != 10 || list[1].Kind != hir.Trap || list[1].ID != 20 {
		t.Fatalf("body %+v", list)
	}
	if list[1].Name != "lowering trap: node 21 (m.ts:2:1): first" {
		t.Fatalf("trap %q", list[1].Name)
	}
	if !init.InitFailure {
		t.Fatal("truncated initializer not marked for rethrow")
	}
}

// A refusal outside every top-level statement keeps the whole-body trap.
func TestTrapModuleInitializerUnlocatedRefusalTrapsWholeBody(t *testing.T) {
	_, init := initializerFixture()
	trapBody(init, []bodyRefusal{{21, -1, "node 21: first"}, {0, -1, "unlocated"}})
	if !isFailureBody(init.Body) || !strings.Contains(init.Body.List[0].Name, "node 21: first; unlocated") || init.InitFailure {
		t.Fatalf("body %+v", init.Body.List)
	}
}

// Only module initializers are trapped per statement.
func TestTrapOrdinaryMethodTrapsWholeBody(t *testing.T) {
	_, init := initializerFixture()
	init.Name = "run"
	trapBody(init, []bodyRefusal{{21, -1, "node 21: first"}})
	if !isFailureBody(init.Body) {
		t.Fatalf("body %+v", init.Body.List)
	}
}

// The first statement refused leaves a single trap: repairFailedBodies then
// applies the existing failed-initialization rule (every static field goes).
func TestTrapModuleInitializerFirstStatement(t *testing.T) {
	_, init := initializerFixture()
	trapBody(init, []bodyRefusal{{11, -1, "node 11: first"}})
	if !isFailureBody(init.Body) || init.Body.List[0].ID != 10 {
		t.Fatalf("body %+v", init.Body.List)
	}
}

// Synthetic ID-zero refusals located by their statement (the catalogue gate
// visits each top-level statement) trap from that statement on.
func TestTrapModuleInitializerSyntheticRefusal(t *testing.T) {
	_, init := initializerFixture()
	trapBody(init, []bodyRefusal{{0, 2, "node 0 (): synthetic"}, {0, 3, "node 0 (): later"}})
	if list := init.Body.List; len(list) != 3 || list[2].Kind != hir.Trap || list[2].Name != "lowering trap: node 0 (): synthetic" {
		t.Fatalf("body %+v", list)
	}
}

// End to end through TrapABAPTestRefusals: an unsupported runtime operation
// with a synthetic ID in the third statement keeps the first two.
func TestTrapABAPTestRefusalsModuleInitializer(t *testing.T) {
	c, init := initializerFixture()
	init.Body.List[2] = &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "no.such.operation", Type: hir.T(hir.Void)}}
	lowering := &RegistryLowering{Prog: &hir.Program{Classes: []*hir.Class{c}}}
	if err := TrapABAPTestRefusals(lowering); err != nil {
		t.Fatal(err)
	}
	list := init.Body.List
	if len(list) != 3 || list[0].ID != 10 || list[1].ID != 20 || list[2].Kind != hir.Trap || !strings.Contains(list[2].Name, "runtime operation no.such.operation has no ABAP emitter") {
		t.Fatalf("body %+v", list)
	}
}
