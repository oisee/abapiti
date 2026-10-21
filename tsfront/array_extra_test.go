package tsfront

import (
	"fmt"
	"testing"
)

// Array metadata (exec index/groups/missing captures, sparse holes) lives
// behind one lazily allocated pointer; these probes pin the observable values.
func arrayExtraNodeOracle(t *testing.T, source string) {
	t.Helper()
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestArrayExtraRegExpExecNodeOracle(t *testing.T) {
	arrayExtraNodeOracle(t, `class Probe {static run():string {
 const out:string[]=[];
 const m=/(a)(x)?(b)/.exec("zab")!;
 out.push(m.index+":"+m.length+":"+String(m[2])+":"+m[3]+":"+JSON.stringify(m));
 const g=/(a)|(b)/g;let r=g.exec("ab");
 while(r!==null){out.push(r.index+":"+String(r[1])+":"+String(r[2])+":"+g.lastIndex);r=g.exec("ab");}
 for(const n of "ab a".matchAll(/(?<first>a)(?<second>b)?/g)){const groups=n.groups!;out.push(n.index+":"+groups.first+":"+(groups.second??"missing")+":"+JSON.stringify(groups));}
 out.push("x1y".replace(/(\d)/,"[$1]"));
 return out.join("|");}}`)
}

func TestArrayExtraHolesNodeOracle(t *testing.T) {
	arrayExtraNodeOracle(t, `class Probe {static run():string {
 const out:string[]=[];
 const a=[1,2,3,4];delete a[1];
 let seen=0;a.forEach(()=>{seen++;});out.push(seen+":"+JSON.stringify(a)+":"+a.join(","));
 const b=a.slice(0);b.reverse();let sb=0;b.forEach(()=>{sb++;});out.push(sb+":"+JSON.stringify(b));
 const c=a.concat([5]);out.push(JSON.stringify(c)+":"+c.length);
 const d=a.splice(0,2);out.push(JSON.stringify(d)+":"+JSON.stringify(a));
 a.unshift(0);out.push(JSON.stringify(a));a.shift();a.pop();out.push(JSON.stringify(a)+":"+a.length);
 const s=new Array<number>(3);s[1]=7;let ss=0;s.forEach(()=>{ss++;});out.push(ss+":"+JSON.stringify(s));
 const e=[1,2,3];delete e[0];e.reverse();let se=0;e.forEach(()=>{se++;});out.push(se+":"+JSON.stringify(e));
 return out.join("|");}}`)
}
