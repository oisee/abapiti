package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestGoErasedNodeArrayMethodDispatch(t *testing.T) {
	source := `interface INode {get():object;getChildren():readonly INode[];}
 class Value {}
 abstract class AbstractNode<T extends INode> implements INode {
 children:T[]=[];abstract get():object;
 getChildren():readonly T[] {return this.children;}
 addChild(n:T):void {this.children.push(n);}
 }
 class TokenNode extends AbstractNode<TokenNode> {get():Value {return new Value();}}
 class StatementNode extends AbstractNode<TokenNode> {get():Value {return new Value();}}
 class Dummy extends AbstractNode<TokenNode> {get():object {return this;}}
 class Probe {static run():boolean {
 const d=new Dummy();(d as any).addChild(new StatementNode());
 return d.getChildren()[0].get() instanceof Value;
 }} `
	p := integerProbe(t, source)
	prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if es := hir.Verify(prog); len(es) > 0 {
		t.Fatal(es)
	}
	if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "true\n" {
		t.Fatal(got)
	}
}
