package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestAsyncCovariantInterfaceBridge(t *testing.T) {
	p, ds := asyncProbe(t, `
interface IResult { name():string; }
interface IFactory { make():Promise<IResult>; fail():Promise<IResult>; }
class Result implements IResult { name():string { return "same"; } }
class State { static value=new Result(); static calls=0; }
class Factory implements IFactory {
 async make():Promise<Result> { State.calls=State.calls+1; return State.value; }
 async fail():Promise<Result> { throw State.value; }
}
export class Probe { static async run():Promise<string> {
 const factory:IFactory=new Factory();
 const a=factory.make(); const b=factory.make();
 const value=await a;
 if(a===b) { return "reused async promise"; }
 if((value as Result)!==State.value) { return "copied payload"; }
 try { await factory.fail(); } catch(e) {
  if((e as Result)!==State.value) { return "copied rejection"; }
  return value.name()+":"+State.calls;
 }
 return "lost rejection";
} }
`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()).String())")
	if strings.TrimSpace(out) != "same:2" {
		t.Fatal(out)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

func TestAsyncCovariantInheritedBridge(t *testing.T) {
	p, ds := asyncProbe(t, `
class Base { async make():Promise<Base> { return this; } name():string {return "base";} }
class Child extends Base { async make():Promise<Child> { return this; } name():string {return "child";} }
export class Probe { static async run():Promise<string> {
 const base:Base=new Child(); return (await base.make()).name();
} }
`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()).String())")
	if strings.TrimSpace(out) != "child" {
		t.Fatal(out)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

// A synchronous producer can expose the same promise through another slot.
// Keep refusing its invariant ABI rather than changing that object's identity.
func TestSharedPromiseCovariantBridgeRemainsRefused(t *testing.T) {
	p, ds := asyncProbe(t, `
interface IResult {name():string;}
interface IFactory {make():Promise<IResult>;}
class Result implements IResult {name():string{return "value";}}
class Factory implements IFactory {
 private value=Promise.resolve(new Result());
 make():Promise<Result>{return this.value;}
}
`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	errs := hir.Verify(p)
	found := false
	for _, err := range errs {
		found = found || strings.Contains(err.Error(), "assignment type mismatch")
	}
	if !found {
		t.Fatalf("shared promise bridge unexpectedly accepted: %v", errs)
	}
}
