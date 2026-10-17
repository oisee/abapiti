package tsfront

import "testing"

// A type predicate narrows a wider object to a recursive family; reads
// project the object's live storage (emit-table-move.ts walkMove visitors
// over IRNode with isExpr/isStmt).
func TestStep4EmitPredicateObjectToFamilyNodeOracle(t *testing.T) {
	a20Oracle(t, `type S={readonly s:"a";readonly n:number}|{readonly s:"b";readonly body:readonly S[]};
 function isS(v:object):v is S{return "s" in v&&typeof v.s==="string";}
 function walk(node:object,visit:(n:object)=>void):void{visit(node);if(isS(node)&&node.s==="b")for(const c of node.body)walk(c,visit);}
 function run(root:S):string{let out="";walk(root,(node):void=>{if(!isS(node))return;if(node.s==="a")out+=node.n;else if(node.s==="b")out+="b"+node.body.length;});walk({w:"q"},(node):void=>{if(isS(node))out+="!";});return out;}
 class Probe {static run():string{return run({s:"b",body:[{s:"a",n:1},{s:"b",body:[]}]});}}`)
}
