package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestStorageFinallyNodeOracle(t *testing.T) {
	source := `class Probe {
 static trace="";
 static f(mode:number):number {
 let value=10;
 try {if(mode===0)return value;if(mode===1)throw new Error("caught");return 30;}
 catch(e){Probe.trace+=(e as Error).message;return 20;}
 finally {value=99;Probe.trace+="F";if(mode===2)return 40;}
 }
 static loopReturn():number {for(let i=0;i<3;i++){try{return i;}finally{if(i<2)continue;}}return -1;}
 static nested():number {try{try{throw new Error("old")}finally{return 3;}}finally{Probe.trace+="N";}}
 static run():string {
 let out="";out+=Probe.f(0)+":"+Probe.f(1)+":"+Probe.f(2)+":"+Probe.nested()+":"+Probe.loopReturn()+";";
 for(let i=0;i<5;i++) {
 try {if(i===1)continue;if(i===3)break;out+=String(i);}
 finally {out+="f";}
 }
 for(let i=0;i<3;i++) {try {throw new Error("suppressed");}finally {if(i===0)continue;break;}}
 try {try{return out+":"+Probe.trace;}finally {throw new Error("replacement");}}
 catch(e) {return out+":"+Probe.trace+":"+(e as Error).message;}
 }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	storageOracleABAP(t, p, string(want))
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStorageFinallyForUpdateNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {let out="";for(let i=0;i<3;i++){try{continue;}finally{out+=String(i);if(i===1)break;}}return out;}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	storageOracleABAP(t, p, string(want))
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStorageFinallyLabelRefusal(t *testing.T) {
	requireDiagnostic(t, sourceProbe(t, `class Probe {static run():void {outer:for(let i=0;i<2;i++){try{break outer;}finally{continue;}}}}`), "unsupported-statement")
}
