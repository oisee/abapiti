package tsfront

import "testing"

func TestGogenA16PadEndAtNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const pads=["a".padEnd(6,"xy"),"a".padEnd(2,"😀"),"😀".padEnd(3,"x"),"x".padEnd(4),"x".padEnd(),"x".padEnd(-3,"0"),"x".padEnd(4,""),"x".padEnd(1e100,"")];
 const ats=["abc".at(-1),"abc".at(-3),"abc".at(-4),"abc".at(3),"abc".at(1.9),"abc".at(-1.9),"abc".at(),"😀".at(-1)];
 return JSON.stringify(pads)+"/"+JSON.stringify(ats);
 }}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "string.padEnd")
}

func TestGogenA16MatchAllNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const re=/(a)(b)?/g;re.lastIndex=1;const it="a ab a".matchAll(re);re.lastIndex=0;
 const out:string[]=[];for(const m of it)out.push(m[0]+":"+m.index+":"+(m[2]??"missing"));
 const unicode=[..."😀z".matchAll(/a*/gu)].map(m=>m.index).join(",");
 const units=[..."😀z".matchAll(/(?:)/g)].map(m=>m.index).join(",");
 let thrown="no";try {"a".matchAll(/a/);}catch(e){thrown=e instanceof TypeError?"TypeError":"other";}
 return out.join("|")+"/"+unicode+"/"+units+"/"+re.lastIndex+"/"+thrown;
 }}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "regexp.clone")
}

func TestGogenA16MatchGroupsNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {const out:string[]=[];for(const m of "a ab".matchAll(/(?<first>a)(?<second>b)?/g)){const groups=m.groups!;out.push(groups.first+":"+(groups.second??"missing"));out.push(JSON.stringify(groups)+":"+Object.keys(groups).join(","));}return out.join("|");}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "regexp.clone")
}

func TestGogenA16MatchAllResumeNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const it="😀ab".matchAll(/(?:)/gu);const first=it.next();const out:string[]=[];
 out.push(String(first.value!.index));for(const m of it){out.push(String(m.index));break;}
 for(const m of it)out.push(String(m.index));out.push(String(it.next().done));
 const plain=[..."a".matchAll(/(a)(b)?/g)];out.push(JSON.stringify(plain));out.push(String(plain[0].groups));
 return out.join("|");}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "regexp.clone")
}

func TestGogenA16StringEvaluationNodeOracle(t *testing.T) {
	source := `class Probe {static order="";static recv():string{Probe.order+="R";return "x";}static size():number{Probe.order+="N";return 4.9;}static fill():string{Probe.order+="F";return "ab";}
 static run():string {const padded=Probe.recv().padEnd(Probe.size(),Probe.fill());const at=Probe.recv().at(Probe.size());const nan=parseFloat("bad");const inf=parseFloat("Infinity");return JSON.stringify([padded,at,Probe.order,"abc".at(nan),"abc".at(inf),"abc".at(-inf),"x".padEnd(nan),"x".padEnd(4,undefined),"x".at(undefined)]);}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "string.padEnd")
}

func TestGogenA16UnicodeMatchesNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {
 const out:string[]=[];for(const m of "😀a".matchAll(/(.)/gu))out.push(m.index+":"+m[1]);
 const re=/./gu;re.lastIndex=1;for(const m of "😀a".matchAll(re))out.push(m.index+":"+m[0]);
 for(const m of "aab".matchAll(/a/gy))out.push(String(m.index));
 const long="x".repeat(1100)+"a";const tail=/a/g;tail.lastIndex=1100;for(const m of long.matchAll(tail))out.push(String(m.index));
 return out.join("|");}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "regexp.clone")
}
func TestGogenA16AtABAPRefusal(t *testing.T) {
	source := `class Probe {static run():string {return "abc".at(-1)??"missing";}}`
	gogenOracle(t, source, "c", "string.relativeAt")
}

func TestGogenA16LabelMatchAllNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {const out:string[]=[];for(const text of ["a:b","a:=b","ab::b","one: two: three:=x","a:😀b: x:","a:\nb:",":a"]){for(const m of text.matchAll(/\b([A-Za-z_][A-Za-z_0-9]*)\s*:(?!=)/g))out.push(m.index+":"+m[0]+":"+m[1]+":"+String(m[2]));}return out.join("|");}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "regexp.clone")
}
