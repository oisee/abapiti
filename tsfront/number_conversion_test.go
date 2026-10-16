package tsfront

import "testing"

// Number(x) is ECMAScript ToNumber: StringToNumber for strings (whole-string,
// trimmed, radix prefixes, Infinity, NaN for junk, no legacy octal), and the
// primitive conversions for the remaining typed arguments.
func TestNumberConversionNodeOracle(t *testing.T) {
	for _, tc := range []struct{ name, source, want, abapRefusal string }{
		{"string grammar", `function show(x:number,k:number):string {if (isNaN(x)) return "NaN";if (x>1e300||x< -1e300) return String(x);if (x>=1e15) return "k"+String(x/16);return String(x*k);}
class Probe {static run():string {
 const xs:string[]=["","  12  ","0x1F","0b101","0o17","0X","Infinity","-Infinity","+1.5e3","1_000","012","abc"," \n\t ",".5","5.","1e","-0x10","\u00a042\ufeff","0x1fffffffffffff1","inf","NaN","1e400","0b","00x1","+Infinity","infinity","0.0000001","1e-400","-0","0o8","0xg","1 2","0B11","9007199254740993","+.5e-1"];
 const ks:number[]=[1,1,1,1,1,1,1,1,1,1,1,1,1,2,1,1,1,1,1,1,1,1,1,1,1,1,10000000,1,1,1,1,1,1,1,20];
 return xs.map((s:string,i:number):string=>show(Number(s),ks[i])).join(",");}}`,
			"0,12,31,5,15,NaN,Infinity,-Infinity,1500,NaN,12,NaN,0,1,5,NaN,NaN,42,k9007199254740991,NaN,NaN,Infinity,NaN,NaN,Infinity,NaN,1,0,0,NaN,NaN,NaN,3,k562949953421312,1", ""},
		{"finite string sites", `class Probe {static map=new Map<string,number>();static run():string {
 const line="#site:12";const tok=" 3 ";const out="7";
 if (/^\d+$/.test(out)) Probe.map.set("o",Number(out));
 const idx=Number(line.slice(6));const w=Number(tok)+1;
 return idx+":"+w+":"+Probe.map.get("o");}}`, "12:4:7", ""},
		{"primitive arguments", `class Probe {static trace="";static s(x:string):string {Probe.trace+=x;return x;}
 static run():string {const b:boolean=Probe.trace==="";const n=2.5;let o:number|undefined;let p:number|undefined=4;
 const r=[Number(),Number(true),Number(b&&false),Number(n)*2,Number(o),Number(p),Number(Probe.s("8"))+Number(Probe.s("x"))];
 return r.map((x:number):string=>String(x)).join(",")+":"+Probe.trace;}}`, "0,1,0,5,NaN,4,NaN:8x", ""},
		{"boxed literal value", `type LiteralValue = string | number | {readonly [name: string]: LiteralValue | undefined};
interface Constant {readonly value?: LiteralValue;}
function show(x:number):string {return isNaN(x)?"NaN":String(x);}
class Probe {static run():string {
 const cs:Constant[]=[{value:" 0x10 "},{value:7},{},{value:{a:"1"}},{value:""},{value:"1e3"},{value:"x"}];
 return cs.map((c:Constant):string=>show(Number(c.value))).join(",");}}`, "16,7,NaN,NaN,0,1000,NaN", "dynamic.toNumber"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := conversionOracle(t, `"use strict";`+tc.source); got != tc.want+"\n" {
				t.Fatalf("Node %q want %q", got, tc.want)
			}
			if tc.abapRefusal != "" {
				gogenOracle(t, tc.source, tc.want, tc.abapRefusal)
			} else {
				gogenOracle(t, tc.source, tc.want)
			}
		})
	}
}
