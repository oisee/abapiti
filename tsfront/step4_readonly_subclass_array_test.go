package tsfront

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// abaplint shape: scope.addList(classAttributes.getConstants()) passes
// readonly ClassAttribute[] where readonly TypedIdentifier[] is expected.
const readonlySubclassArraySource = `class TypedIdentifier {private readonly name:string;constructor(name:string){this.name=name}getName():string{return this.name}}
class ClassAttribute extends TypedIdentifier {private readonly visibility:string;constructor(name:string,visibility:string){super(name);this.visibility=visibility}getVisibility():string{return this.visibility}}
class ClassConstant extends ClassAttribute {private readonly value:string;constructor(name:string,value:string){super(name,"public");this.value=value}getValue():string{return this.value}}
class Attributes {private readonly constants:ClassConstant[]=[];private readonly statics:ClassAttribute[]=[];
 add(c:ClassConstant,s:ClassAttribute):void{this.constants.push(c);this.statics.push(s)}
 getConstants():readonly ClassConstant[]{return this.constants}getStatic():readonly ClassAttribute[]{return this.statics}maybe(present:boolean):readonly ClassAttribute[]|undefined{return present?this.statics:undefined}}
class Scope {private readonly names:string[]=[];
 addList(list:readonly TypedIdentifier[]):void{for(const v of list)this.names.push(v.getName());if(list.length>0)this.names.push("first="+list[0].getName())}
 addListPrefix(list:readonly TypedIdentifier[],prefix:string):void{for(const v of list)this.names.push(prefix+v.getName())}
 addMaybe(list:readonly TypedIdentifier[]|undefined):void{this.names.push(list===undefined?"none":"n="+list.length)}
 dump():string{return this.names.join(",")}}
class Probe {static run():string {const a=new Attributes();a.add(new ClassConstant("c1","1"),new ClassAttribute("s1","private"));a.add(new ClassConstant("c2","2"),new ClassAttribute("s2","public"));
 const scope=new Scope();scope.addList(a.getConstants());scope.addList(a.getStatic());scope.addListPrefix(a.getConstants(),"i~");scope.addMaybe(a.maybe(true));scope.addMaybe(a.maybe(false));return scope.dump();}}
`

func TestStep4ReadonlySubclassArrayGoABAP(t *testing.T) {
	readonlyStrictCheck(t, map[string]string{"probe.ts": readonlySubclassArraySource})
	want := strings.TrimSuffix(nodePlatformOracle(t, readonlySubclassArraySource), "\n")
	p := structuralUnionProgram(t, readonlySubclassArraySource)
	if _, ok := hir.UsedRuntimeOps(p)["container.readonlyView"]; ok {
		t.Fatal("nominal subclass reference arrays need no readonly container view")
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want+"\n" {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", want, "")
}

// abaplint lsp/help.ts: this.outputNodes(statement.getChildren()) passes
// readonly (ExpressionNode|TokenNode)[] where readonly INode[] is expected.
const readonlyUnionInterfaceArraySource = `interface INode {setChildren(children:INode[]):void;getChildren():readonly INode[];get():string}
abstract class AbstractNode<T extends INode> implements INode {protected children:T[]=[];setChildren(children:T[]):void{this.children=children.slice()}getChildren():readonly T[]{return this.children}getFirstChild():T|undefined{return this.children[0]}abstract get():string}
class TokenNode implements INode {private readonly text:string;constructor(text:string){this.text=text}setChildren(_children:INode[]):void{throw new Error("token")}getChildren():readonly INode[]{return []}get():string{return "T:"+this.text}}
class ExpressionNode extends AbstractNode<ExpressionNode|TokenNode> {private readonly name:string;constructor(name:string){super();this.name=name}
 setChildren(children:(ExpressionNode|TokenNode)[]):ExpressionNode{super.setChildren(children);return this}get():string{return "E:"+this.name}}
class StatementNode extends AbstractNode<ExpressionNode|TokenNode> {setChildren(children:(ExpressionNode|TokenNode)[]):StatementNode{super.setChildren(children);return this}get():string{return "S"}}
class Help {static outputNodes(nodes:readonly INode[]):string{let out="";for(const n of nodes)out+=n.get()+"("+Help.outputNodes(n.getChildren())+")";return out+"#"+nodes.length}}
class Probe {static run():string {const kids:(ExpressionNode|TokenNode)[]=[new ExpressionNode("x").setChildren([new TokenNode("b")]),new TokenNode("a")];const s=new StatementNode().setChildren(kids);
 const view:readonly (ExpressionNode|TokenNode)[]=kids;const first=s.getFirstChild();const before=Help.outputNodes(view);kids.push(new TokenNode("c"));
 return before+"|"+Help.outputNodes(view)+"|"+Help.outputNodes(s.getChildren())+"|"+(first===undefined?"none":first.get());}}
`

func TestStep4ReadonlyUnionInterfaceArrayGoABAP(t *testing.T) {
	readonlyStrictCheck(t, map[string]string{"probe.ts": readonlyUnionInterfaceArraySource})
	want := strings.TrimSuffix(nodePlatformOracle(t, readonlyUnionInterfaceArraySource), "\n")
	p := structuralUnionProgram(t, readonlyUnionInterfaceArraySource)
	if _, ok := hir.UsedRuntimeOps(p)["container.readonlyView"]; ok {
		t.Fatal("class-union interface reference arrays need no readonly container view")
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want+"\n" {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", want, "")
}
