package tsfront

import "testing"

func TestMutualCaptureNodeOracle(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"declarations and hoisting", `class Probe {static run():string {let value=1;const first=even(2);value=10;function even(n:number):number {value++;return n===0?value:odd(n-1);}function odd(n:number):number {value+=2;return n===0?value:even(n-1);}return first+":"+odd(2)+":"+value;}}`, "5:15:15"},
		{"arrows and transitive captures", `class Probe {static run():string {let value=1;let trace="";const a=(n:number):number=>{trace+="a";return n===0?value:b(n-1);};const b=(n:number):number=>{value+=2;trace+="b";return n===0?value:c(n-1);};const c=(n:number):number=>{trace+="c";return n===0?value:a(n-1);};value=10;return a(4)+":"+value+":"+trace;}}`, "14:14:abcab"},
		{"escaped declarations", `class Probe {static run():number {let value=1;function a(n:number):number {value++;return n===0?value:b(n-1);}function b(n:number):number {value+=2;return n===0?value:a(n-1);}const f=a;value=10;return f(2);}}`, "14"},
		{"capture only through peer", `class Probe {static run():number {let value=1;function a(n:number):number{return n===0?b(0):b(n-1);}function b(n:number):number{value++;return n===0?value:a(n-1);}value=10;return a(2);}}`, "12"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := conversionOracle(t, tc.source); got != tc.want+"\n" {
				t.Fatalf("Node %q want %q", got, tc.want)
			}
			gogenOracle(t, tc.source, tc.want)
		})
	}
}

func TestDestructuredCallbackCaptureNodeOracle(t *testing.T) {
	gogenOracle(t, `function signature(name:string):string {const [intf,meth]=name.includes("~")?name.split("~"):[null,name];const param=(x:string):string=>(intf??"local")+":"+meth+":"+x;return ["x","y"].map(x=>param(x)).join("/");}class Probe {static run():string{return signature("I~M")+"/"+signature("M");}}`, "I:M:x/I:M:y/local:M:x/local:M:y")
}

func TestDestructuredSharedCaptureNodeOracle(t *testing.T) {
	gogenOracle(t, `class Probe {static run():string {let [value]=[1];let {text}={text:"a"};function a(n:number):number {text+="a";return n===0?value:b(n-1);}function b(n:number):number {value++;text+="b";return n===0?value:a(n-1);}value=10;text="x";return a(3)+":"+value+":"+text;}}`, "12:12:xabab")
}
