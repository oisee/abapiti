package tsfront

import "testing"

func TestStep4EmitOverloadOptionalResultNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Has {
 has(x: {kind: "full"; value: string}): x is {kind: "full"; value: string};
 has(x: {kind: "probe"}): boolean | undefined;
 }
 class Checker implements Has {
 has(x: {kind: "full"; value: string}): x is {kind: "full"; value: string};
 has(x: {kind: "probe"}): boolean | undefined;
 has(x: {kind: string}): boolean | undefined {return x.kind === "full" ? true : undefined;}
 }
 class Probe {static run(): string {const h: Has=new Checker();return String(h.has({kind:"probe"}))+":"+h.has({kind:"full",value:"ok"});}}`, "")
}

func TestStep4EmitOverloadContextAliasNodeOracle(t *testing.T) {
	gogenTSOracle(t, `interface Context {readonly log:string[]}
 interface Has {has(x:{kind:"full";value:string},ctx:Context):x is {kind:"full";value:string};has(x:{kind:"probe"},ctx:Context):boolean|undefined;}
 class Checker implements Has {
 has(x:{kind:"full";value:string},ctx:Context):x is {kind:"full";value:string};has(x:{kind:"probe"},ctx:Context):boolean|undefined;
 has(x:{kind:string},ctx:Context):boolean|undefined{ctx.log.push(x.kind);return x.kind==="full"?true:undefined;}}
 class Probe {static run():string{const h:Has=new Checker(),ctx:Context={log:[]};const first=h.has({kind:"probe"},ctx),second=h.has({kind:"full",value:"ok"},ctx);return String(first)+":"+second+":"+ctx.log.join(",");}}`, "")
}
