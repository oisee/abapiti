package tsfront

import "testing"

func TestStep4EmitReadonlyNestedViewNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Item {readonly name:string;readonly type?:string} interface FullItem {name:string;readonly type:string;readonly extra:string}
 interface Context {readonly cls:Partial<Item>; readonly method?:Partial<Item>} interface FullContext {readonly cls:FullItem;readonly method:FullItem}
 class Probe {static inspect(ctx:Context):string{return ctx.cls.name+":"+ctx.method?.type;}
 static run():string{const item:FullItem={name:"before",type:"T",extra:"kept"};const full:FullContext={cls:item,method:item};const view:Context=full;item.name="after";return Probe.inspect(full)+":"+(view===full)+":"+(view.cls===item)+":"+(view.method===item)+":"+item.extra;}}`, "")
}
