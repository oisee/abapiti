package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	gohir "github.com/oisee/abapiti/hir/golang"
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

func TestTrapMissingInterfaceImplementation(t *testing.T) {
	s := hir.T(hir.String)
	signature := &hir.Method{Name: "run", Params: []hir.Param{{Name: "input", Type: s}}, Result: s}
	good := &hir.Method{Name: "good", Result: s, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(s, "ok")})}
	c := &hir.Class{Node: hir.Node{ID: 1, Source: "probe.ts:7:3"}, Name: "Probe", Implements: []string{"Runner", "AlsoRunner"}, Methods: []*hir.Method{good}}
	p := &hir.Program{Interfaces: []*hir.Interface{{Name: "Runner", Methods: []*hir.Method{signature}}, {Name: "AlsoRunner", Methods: []*hir.Method{signature}}}, Classes: []*hir.Class{c}}
	if errors := hir.Verify(p); len(errors) == 0 {
		t.Fatal("fixture must require a missing method")
	}
	(&lowerer{out: p, serial: 1}).repairFailedBodies()
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	if len(c.Methods) != 2 || c.Methods[0] != good || isFailureBody(good.Body) {
		t.Fatal("valid sibling replaced or duplicate stub added")
	}
	stub := c.Methods[1]
	if !isFailureBody(stub.Body) || !strings.Contains(stub.Body.List[0].Name, "probe.ts:7:3: missing interface implementation run") {
		t.Fatal("missing located refusal", stub)
	}
	main := `defer func(){x:=recover();if x==nil{panic("missing interface trap")};fmt.Print(x)}();` + hir.NewNames().Get("body.Probe.run") + `(` + hir.NewNames().Get("new.Probe") + `(),str("input"))`
	if got := runGoHIR(t, p, main); !strings.Contains(got, "probe.ts:7:3: missing interface implementation run") {
		t.Fatal(got)
	}
}

func TestTrapMissingInterfacePreservesInheritedImplementation(t *testing.T) {
	s := hir.T(hir.String)
	m := &hir.Method{Name: "run", Virtual: true, Result: s, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(s, "ok")})}
	base := &hir.Class{Name: "Base", Methods: []*hir.Method{m}}
	child := &hir.Class{Name: "Child", Super: "Base", Implements: []string{"Runner"}}
	p := &hir.Program{Interfaces: []*hir.Interface{{Name: "Runner", Methods: []*hir.Method{{Name: "run", Result: s}}}}, Classes: []*hir.Class{base, child}}
	(&lowerer{out: p}).repairFailedBodies()
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	if len(child.Methods) != 0 || len(base.Methods) != 1 || base.Methods[0] != m || isFailureBody(m.Body) {
		t.Fatal("existing inherited implementation changed")
	}
}

func TestTrapIncompatibleInterfaceConversionPreservesMethodABI(t *testing.T) {
	s, n := hir.T(hir.String), hir.T(hir.Number)
	good := &hir.Method{Name: "run", Virtual: true, Result: s, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(s, "ok")})}
	use := &hir.Method{Node: hir.Node{ID: 2, Source: "probe.ts:9:3"}, Name: "use", Static: true, Result: hir.Type{Kind: hir.InterfaceRef, Name: "Runner"}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: hir.Ref("Probe")}})}
	c := &hir.Class{Node: hir.Node{ID: 1, Source: "probe.ts:7:3"}, Name: "Probe", Implements: []string{"Runner"}, Methods: []*hir.Method{good, use}}
	p := &hir.Program{Interfaces: []*hir.Interface{{Name: "Runner", Methods: []*hir.Method{{Name: "run", Result: n}}}}, Classes: []*hir.Class{c}}
	(&lowerer{out: p, serial: 2}).repairFailedBodies()
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	if len(c.Implements) != 0 || isFailureBody(good.Body) || !good.Result.Equal(s) {
		t.Fatal("invalid interface ABI was guessed")
	}
	if !isFailureBody(use.Body) || !strings.Contains(use.Body.List[0].Name, "probe.ts:9:3") {
		t.Fatal("conversion did not become a located refusal")
	}
	if got := runGoHIR(t, p, `defer func(){x:=recover();if x==nil{panic("missing conversion trap")};fmt.Print(x)}();`+hir.NewNames().Get("body.Probe.use")+`()`); !strings.Contains(got, "probe.ts:9:3") {
		t.Fatal(got)
	}
}

func TestEmitGoBackendRefusalBecomesLocatedTrap(t *testing.T) {
	n := hir.Node{ID: 1, Source: "probe.ts:17:6"}
	regex := &hir.Expr{Node: n, Kind: hir.New, Type: hir.T(hir.RegExp), Args: []*hir.Expr{hir.L(hir.T(hir.String), "(?=a)a"), hir.L(hir.T(hir.String), "")}}
	bad := &hir.Method{Node: hir.Node{ID: 2, Source: "probe.ts:16:3"}, Name: "bad", Static: true, Result: hir.T(hir.RegExp), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: regex})}
	good := &hir.Method{Name: "good", Static: true, Result: hir.T(hir.String), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(hir.T(hir.String), "ok")})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{bad, good}}}}
	if _, err := gohir.Emit(p); err == nil {
		t.Fatal("fixture must have a backend-only refusal")
	}
	if _, err := EmitGoWithTraps(&RegistryLowering{Prog: p}); err != nil {
		t.Fatal(err)
	}
	if !isFailureBody(bad.Body) || isFailureBody(good.Body) {
		t.Fatal("backend refusal crossed body boundary")
	}
	if got := runGoHIR(t, p, `defer func(){x:=recover();if x==nil{panic("missing backend trap")};fmt.Print(x)}();`+hir.NewNames().Get("body.Probe.bad")+`()`); !strings.Contains(got, "probe.ts:17:6") || !strings.Contains(got, "JavaScript regexp") {
		t.Fatal(got)
	}
}
