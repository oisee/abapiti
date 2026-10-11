package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestTrapFailureIsOptIn(t *testing.T) {
	dir := t.TempDir()
	source := `namespace Unused { export const value = 1; }
 export class Probe {
  constructor() { const date = new Date(); }
  bad(): void { const date = new Date(); }
  good(): string { return "ok"; }
  nested(): () => void { return () => { const date = new Date(); }; }
 }
 export function badFunction(): void { const date = new Date(); }
 `
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, strict, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{})
	if err != nil || !hasBlocking(strict) {
		t.Fatalf("strict lowering changed: %v %v", strict, err)
	}
	prog, diags, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(diags) {
		t.Fatalf("partial lowering: %v %v", diags, err)
	}
	if errors := hir.Verify(prog); len(errors) > 0 {
		t.Fatal(errors)
	}
	var probe *hir.Class
	functionTrapped := false
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			if m.Name == "badFunction" {
				functionTrapped = isFailureBody(m.Body)
			}
		}
		if c.Name == "probe.ts.Probe" {
			probe = c
		}
	}
	if !functionTrapped {
		t.Fatal("module function did not trap")
	}
	if probe == nil || !isFailureBody(probe.Ctor.Body) {
		t.Fatal("constructor did not trap")
	}
	for _, m := range probe.Methods {
		if m.Name == "good" && isFailureBody(m.Body) {
			t.Fatal("valid sibling trapped")
		}
		if m.Name == "nested" && isFailureBody(m.Body) {
			t.Fatal("nested closure poisoned outer body")
		}
		if m.Name == "bad" && (!isFailureBody(m.Body) || !strings.Contains(m.Body.List[0].Name, "probe.ts:") || !strings.Contains(m.Body.List[0].Name, "Date")) {
			t.Fatal("missing diagnostic/location")
		}
	}
	n := hir.NewNames()
	main := `func(){defer func(){if x:=recover();x==nil{panic("missing constructor trap")}}();` + n.Get("new.probe.ts.Probe") + `()}();fmt.Println("trapped")`
	if got := runGoHIR(t, prog, main); got != "trapped\n" {
		t.Fatal(got)
	}
}

func TestTrapFailedStaticInitializerAtUse(t *testing.T) {
	num := hir.T(hir.Number)
	init := &hir.Method{Name: "class_constructor", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "Probe", Name: "value", Type: num}, Y: hir.L(hir.T(hir.String), "bad")})}
	use := &hir.Method{Name: "use", Static: true, Result: num, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.StaticGet, Owner: "Probe", Name: "value", Type: num}})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Fields: []hir.Field{{Name: "value", Type: num, Static: true}}, Methods: []*hir.Method{init, use}}}}
	(&lowerer{out: p}).repairFailedBodies()
	if !isFailureBody(init.Body) || !isFailureBody(use.Body) || len(p.Classes[0].Fields) != 0 {
		t.Fatal("failed initializer exposed a fabricated zero value")
	}
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
}

func TestTrapVerificationDoesNotPoisonZeroIDSiblings(t *testing.T) {
	s := hir.T(hir.String)
	good := &hir.Method{Name: "good", Static: true, Result: s, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(s, "ok")})}
	bad := &hir.Method{Name: "bad", Static: true, Result: s, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(hir.T(hir.Number), 1)})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{good, bad}}}}
	l := &lowerer{out: p}
	l.repairFailedBodies()
	if isFailureBody(good.Body) || !isFailureBody(bad.Body) {
		t.Fatal("verifier attribution crossed method boundary")
	}
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
}
