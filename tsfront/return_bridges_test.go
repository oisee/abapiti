package tsfront

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/internal/gracecheck"
)

// Model physical result erasure independently of the external declaration
// package: the implementation still returns the exact WorkspaceEdit instance.
func TestReturnBridgeErasedOptionalNodeOracle(t *testing.T) {
	source := `class WorkspaceEdit {documentChanges?:number[];changes?:string;}
 interface ObjectRenamer {buildEdits(present:boolean):WorkspaceEdit|undefined;}
 class Renamer implements ObjectRenamer {
 calls=0;edit:WorkspaceEdit;
 constructor(edit:WorkspaceEdit) {this.edit=edit;}
 buildEdits(found:boolean):WorkspaceEdit|undefined {this.calls++;return found?this.edit:undefined;}
 }
 class Probe {
 static run():string {
 const edit=new WorkspaceEdit();edit.documentChanges=[1];
 const concrete=new Renamer(edit);const renamer:ObjectRenamer=concrete;
 const missing=renamer.buildEdits(false);
 const returned=renamer.buildEdits(true)!;
 returned.documentChanges!.push(2);
 const again=concrete.buildEdits(true)!;again.documentChanges!.push(3);
 const absent=returned.changes===undefined;returned.changes="live";
 return (missing===undefined)+":"+(returned===edit)+":"+(again===edit)+":"+edit.documentChanges!.join(",")+":"+absent+":"+edit.changes+":"+concrete.calls;
 }
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	var impl, bridge *hir.Method
	for _, c := range prog.Classes {
		if c.Name == "probe.ts.Renamer" {
			for _, m := range c.Methods {
				if m.Name == "buildEdits" {
					bridge = m
				} else if m.Name == "buildEdits_instantiated_probe.ts.Renamer" {
					impl = m
				}
			}
		}
	}
	if impl == nil || bridge == nil {
		t.Fatal("missing ordinary override bridge", hir.Dump(prog))
	}
	// This is the physical erasure observed in the untrapped RenameGlobalInterface
	// closure. A pre-body bridge has the old narrower call result as well.
	impl.Result = hir.T(hir.Optional, hir.Ref(hir.RootObject))
	// Model concrete source calls retaining the checked declaration view.
	for _, c := range prog.Classes {
		for _, m := range numberMethods(c) {
			if m == bridge {
				continue
			}
			walkNumberStmt(m.Body, func(*hir.Stmt) {}, func(x *hir.Expr) {
				if x.Kind == hir.VirtualCall && x.Name == impl.Name && !x.Type.Equal(impl.Result) {
					call := *x
					call.Type = impl.Result
					*x = hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: x.Type, X: &call}
				}
			})
		}
	}
	l := &lowerer{out: prog, bridgeTargets: map[*hir.Method]*hir.Method{bridge: impl}}
	l.completeReturnBridges()
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	if bridge.Body.List[0].X.Kind != hir.Narrow {
		t.Fatal("erased return has no checked view")
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err, hir.Dump(prog))
	}
	if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	gracecheck.Check(t, prog)
	// An incompatible erased object must refuse at the same bridge boundary.
	impl.Body = hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: hir.Ref("probe.ts.Renamer"), Args: []*hir.Expr{{Kind: hir.New, Type: hir.Ref("probe.ts.WorkspaceEdit")}}}})
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	names := hir.NewNames()
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("incompatible bridge result escaped")};fmt.Println("refused")}();r:=%s(%s());r.%s(true)`, names.Get("new.probe.ts.Renamer"), names.Get("new.probe.ts.WorkspaceEdit"), names.Get("member.buildEdits"))
	if got := runGoHIR(t, prog, main); got != "refused\n" {
		t.Fatal(got)
	}
}

func TestReturnBridgeLateTargetAndFixpoint(t *testing.T) {
	child := &hir.Class{Name: "Child", Super: "Base"}
	base := &hir.Class{Name: "Base"}
	leaf := &hir.Method{Name: "read_instantiated_Child_2", Virtual: true, Result: hir.Ref("Child"), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("Child")}})}
	middle := &hir.Method{Name: "read_instantiated_Child", Virtual: true, Result: hir.Ref("Base"), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.VirtualCall, Name: "old", Type: hir.Ref(hir.RootObject), X: &hir.Expr{Kind: hir.This, Type: hir.Ref("Child")}}})}
	top := &hir.Method{Name: "read", Virtual: true, Result: hir.Ref("Base"), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.VirtualCall, Name: "read", Type: hir.Ref("Child"), X: &hir.Expr{Kind: hir.This, Type: hir.Ref("Child")}}})}
	child.Methods = []*hir.Method{leaf, middle, top}
	prog := &hir.Program{Classes: []*hir.Class{base, child}}
	l := &lowerer{out: prog, bridgeTargets: map[*hir.Method]*hir.Method{top: middle, middle: leaf}}
	before := l.unionABIStamp()
	middle.Result = hir.Ref(hir.RootObject)
	if before == l.unionABIStamp() {
		t.Fatal("result-only widening was invisible to fixpoint")
	}
	l.completeReturnBridges()
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	x := top.Body.List[0].X
	if x.Kind != hir.Narrow || x.X.Name != middle.Name || !x.X.Type.Equal(middle.Result) {
		t.Fatal("late target ABI not completed")
	}
	stamp := hir.Dump(prog)
	l.completeReturnBridges()
	if stamp != hir.Dump(prog) {
		t.Fatal("completion is not idempotent")
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	names := hir.NewNames()
	main := fmt.Sprintf("c:=&%s{};fmt.Println(c.%s()==c)", names.Get("struct.Child"), names.Get("member.read"))
	if got := runGoHIR(t, prog, main); got != "true\n" {
		t.Fatal(got)
	}
}

func TestReturnBridgeUnionCovarianceNodeOracle(t *testing.T) {
	source := `interface Named {name():string;}
 interface Detailed extends Named {detail():string;}
 class Value implements Detailed {name():string{return "edit";}detail():string{return "live";}}
 interface Renamer {read(flag?:boolean):Named|undefined;}
 class A implements Renamer {value:Value;calls=0;constructor(value:Value){this.value=value;}read(found?:boolean):Detailed|undefined{this.calls++;return found?this.value:undefined;}}
 class B {value:Value;calls=0;constructor(value:Value){this.value=value;}read(found?:boolean):Named|undefined{this.calls++;return found?this.value:undefined;}}
 class Probe {
 static read(x:A|B):Named|undefined {return x.read(true);}
 static run():string {const value=new Value();const a=new A(value);const b=new B(value);const slot:Renamer=a;const absent=slot.read();const x=Probe.read(a)!;const y=Probe.read(b)!;return (absent===undefined)+":"+(x===value)+":"+(y===value)+":"+x.name()+":"+a.read(true)!.detail()+":"+a.calls+":"+b.calls;}
 }`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err, hir.Dump(prog))
	}
	if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	gracecheck.Check(t, prog)
}
