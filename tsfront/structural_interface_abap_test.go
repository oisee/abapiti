package tsfront

import (
	"os/exec"
	"strings"
	"testing"
)

func TestStructuralInterfaceABAPNodeOracle(t *testing.T) {
	source := `interface Reader { read(label:string):string; }
class Base { read(label:string):string {return label+"live";} }
class ReaderImpl extends Base {}
class AssertedReader extends Base {}
class Holder { value:Reader; constructor(value:Reader){this.value=value;} }
class Probe {
 static same(left:any,right:any):boolean {return left===right;}
 static take(value:Reader):string {return value.read("pass:");}
 static give(value:ReaderImpl):Reader {return value;}
 static run():string {
  const original=new ReaderImpl();
  const assigned:Reader|undefined=original;
  const values:Reader[]=[original];
  const holder=new Holder(original);
  const assertedOriginal=new AssertedReader();
  const asserted=assertedOriginal as Reader;
  return assigned.read("assign:")+";"+Probe.take(original)+";"+values[0].read("array:")+";"+holder.value.read("field:")+";"+Probe.give(original).read("return:")+";"+(assigned===original)+":"+(values[0]===original)+";"+asserted.read("assert:")+":"+Probe.same(asserted,assertedOriginal);
 }
}`
	structuralInterfaceOracle(t, source)
}

func TestStructuralInterfaceAdapterABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface Reader {read(label:any):any; absent?():string;}
class ReaderImpl {prefix:string="old:"; read(label:string):string {return this.prefix+label;}}
class Probe {
 static same(left:any,right:any):boolean {return left===right;}
 static again(value:any):Reader {return value;}
 static run():string {
  const original=new ReaderImpl();
  const value=original as Reader;
  const twice=original as Reader;
  original.prefix="new:";
  const restored=Probe.again(value);
  const back=value as ReaderImpl;
  const values:Reader[]=[value];
  const set=new Set<object>();
  set.add(value);
  set.add(original);
  const map=new Map<object,string>();
  map.set(value,"stored");
  return value.read("call")+":"+Probe.same(value,original)+":"+Probe.same(value,twice)+":"+restored.read("erased")+":"+Probe.same(back,original)+":"+(value instanceof ReaderImpl)+":"+values.includes(twice)+":"+values.indexOf(twice)+":"+set.size+":"+set.has(twice)+":"+map.get(original)+":"+map.has(twice);
 }
}`)
}

func TestStructuralInterfaceCallableFieldABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface Reader {read(label:string):string;}
class ReaderImpl {prefix:string="field:"; read=(label:string):string=>this.prefix+label;}
class Probe {
 static same(left:any,right:any):boolean {return left===right;}
 static run():string {
  const original=new ReaderImpl();
  const value=original as Reader;
  const before=value.read("one");
  original.read=(label:string):string=>"changed:"+label;
  return before+":"+value.read("two")+":"+Probe.same(value,original);
 }
}`)
}

func TestStructuralInterfaceSubclassAdapterABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface Reader {read():string;}
class Base {}
class Child extends Base {read():string {return "child";}}
class Nominal extends Base implements Reader {read():string {return "nominal";}}
class Probe {
 static view(value:Base):Reader {return value as Reader;}
 static same(left:any,right:any):boolean {return left===right;}
 static run():string {
  const original=new Child();
  const value=Probe.view(original);
  const nominalOriginal=new Nominal();
  const nominal=Probe.view(nominalOriginal);
  return value.read()+":"+Probe.same(value,original)+":"+nominal.read()+":"+Probe.same(nominal,nominalOriginal);
 }
}`)
}

func TestStructuralInterfaceArrayResultABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface ChildView {read():any;}
interface ParentView {children():ChildView[];}
class Child {read():string {return "nested";}}
class Parent {child:Child=new Child(); children():Child[] {return [this.child];}}
class Probe {
 static same(left:any,right:any):boolean {return left===right;}
 static run():string {
  const original=new Parent();
  const parent=original as ParentView;
  const children=parent.children();
  return children[0].read()+":"+Probe.same(children[0],original.child)+":"+Probe.same(parent,original);
 }
}`)
}

// An Identifier-to-method view also emits dispatch for the event sibling.
// Its array result must not be assigned directly to the method's parameter
// interface, even when this execution only calls the method sibling.
func TestStructuralInterfaceSiblingResultABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface MethodParams {size():number;}
interface MethodView {getParameters():MethodParams;}
class Identifier {}
class MethodParameters {size():number {return 2;}}
class MethodDefinition extends Identifier {getParameters():MethodParameters {return new MethodParameters();}}
class EventDefinition extends Identifier {getParameters():string[] {return ["event"];}}
class Probe {
 static view(value:Identifier):MethodView {return value as MethodView;}
 static run():string {return "parameters:"+Probe.view(new MethodDefinition()).getParameters().size();}
}`)
}

func TestStructuralInterfaceAdapterConversionsABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface Conversions {
 round(values?:string[]):string[]|undefined;
 text(value?:string):string|undefined;
 wide(values:string[]):any[];
 narrow(values:any[]):string[];
}
class Converter {
 round(values?:any):any {return values;}
 text(value?:any):any {return value;}
 wide(values:any[]):any[] {return values;}
 narrow(values:string[]):string[] {return values;}
}
class Probe {
 static same(left:any,right:any):boolean {return left===right;}
 static run():string {
  const converter=new Converter() as Conversions;
  const original=["first"];
  const round=converter.round(original);
  const wide=converter.wide(original);
  const narrow=converter.narrow(wide);
  wide.push("second");
  narrow.push("third");
  return (converter.round()===undefined)+":"+(converter.text()===undefined)+":"+(converter.text("")==="")+":"+converter.text("text")+":"+round.join(",")+":"+narrow.join(",")+":"+Probe.same(round,original)+":"+Probe.same(wide,original)+":"+Probe.same(narrow,original);
 }
}`)
}

func TestStructuralInterfaceAdapterUnionABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `type Item={kind:"text";value:string}|{kind:"count";value:number};
interface Conversions {round(value?:Item):Item|undefined; values(value?:Item[]):Item[]|undefined;}
class Converter {round(value?:any):any {return value;} values(value?:any):any {return value;}}
class Probe {
 static same(left:any,right:any):boolean {return left===right;}
 static run():string {
  const converter=new Converter() as Conversions;
  const item:Item={kind:"text",value:"union"};
  const original:Item[]=[item];
  const round=converter.round(item);
  const values=converter.values(original);
  return (converter.round()===undefined)+":"+(converter.values()===undefined)+":"+round.kind+":"+values[0].kind+":"+Probe.same(round,item)+":"+Probe.same(values,original)+":"+Probe.same(values[0],item);
 }
}`)
}

func TestStructuralInterfaceVoidResultABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface Reader {read():any;}
class ReaderImpl {calls:number=0; read():void {this.calls++;}}
class Probe {
 static run():string {
  const original=new ReaderImpl();
  const value=original as Reader;
  return (value.read()===undefined)+":"+original.calls;
 }
}`)
}

func TestStructuralInterfaceAccessorABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface Reader {read(label:string):string;}
class ReaderImpl {prefix:string="getter:"; get read():(label:string)=>string {return label=>this.prefix+label;}}
class Probe {
 static run():string {
  const original=new ReaderImpl();
  const value=original as Reader;
  const before=value.read("one");
  original.prefix="changed:";
  return before+":"+value.read("two");
 }
}`)
}

func TestStructuralInterfaceMissingMemberABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface Reader {read():string; absent?():string;}
class ReaderImpl {read():string {return "present";}}
class Probe {
 static run():string {
  const value=new ReaderImpl() as Reader;
  let missing=false;
  try {value.absent();} catch(e) {missing=e instanceof TypeError;}
  return value.read()+":"+missing;
 }
}`)
}

func TestStructuralInterfaceNoncallableAccessorABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface Reader {read():string;}
class ReaderImpl {calls:number=0; get read():number {this.calls++;return 1;}}
class Probe {
 static run():string {
  const original=new ReaderImpl();
  const value=original as Reader;
  let failed=false;
  try {value.read();} catch(e) {failed=e instanceof TypeError;}
  return failed+":"+original.calls;
 }
}`)
}

func TestStructuralInterfaceNestedArgumentABAPNodeOracle(t *testing.T) {
	structuralInterfaceOracle(t, `interface ArgView {read():any;}
interface ConsumerView {use(value?:ArgView):boolean;}
class Arg {read():string {return "nested";}}
class Consumer {
 static original:any;
 use(value?:any):boolean {return value===Consumer.original;}
}
class Probe {
 static run():string {
  const original=new Arg();
  Consumer.original=original;
  const arg=original as ArgView;
  const consumer=new Consumer() as ConsumerView;
  return consumer.use(arg)+":"+consumer.use()+":"+arg.read();
 }
}`)
}

func structuralInterfaceOracle(t *testing.T, source string) {
	t.Helper()
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "")
}
