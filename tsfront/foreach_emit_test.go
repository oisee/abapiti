package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGoUnpinnedForEachNodeOracle(t *testing.T) {
	source := `class Probe {
 static run(): string {
  const a: number[] = [1,2,3]; let log = "";
  a.forEach((v,i,arr) => {
   log += i + ":" + v + ";";
   if (i === 0) { delete arr[1]; arr[2]=9; arr.push(7); }
  });
  const b: number[] = [4,5,6];let count=0;
  b.forEach((v,i) => { count += v; if (i===0) { b.pop();b.pop(); } });
  const c: (number|undefined)[] = [undefined,1];let present=0;
  c.forEach((v,i) => {present += 1;});
  const d: number[] = [];d[2]=8;let sparse="";
  d.forEach((v,i) => {sparse += i + ":" + v;});
  let e=0;const empty: number[]=[];
  const yes=empty.every((v) => {e += 1;return false;});
  const sparseYes=d.every((v) => v===8);
  const short=[1,2,3].every((v) => {e += 1;return v<2;});
  const f: number[]=[1,2,3];
  f.forEach((v,i,arr)=>{if(v===1){arr[1]=12;return;}arr[2]=20;});
  let zero=0;f.forEach(()=>{zero+=1;});
  const g:number[]=[1,2,3];delete g[1];
  const h=g.slice().reverse();h.unshift(4);h.splice(1,1,5);h.shift();let holes="";
  h.forEach((v,i)=>{holes += i+":"+v+";";});
  return log+"/"+count+"/"+present+"/"+sparse+"/"+yes+"/"+sparseYes+"/"+short+"/"+e+"/"+f.join()+"/"+zero+"/"+holes;
 }
}`
	cmd := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source)
	want, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestGoSmallRegistrationExpressions(t *testing.T) {
	source := `class Conf {length: number=80;}
 class Probe {static run(): string {
 const rows=[{cnt:1},{cnt:2}].map((c,i)=>{if(i===0){return {...c,cnt:0};}else{return c;}});
 const a=new Conf();const value=(a.length=400);
 const s="a\nb\n".replace(/\n/g,"\r\n");
 return rows[0].cnt+":"+rows[1].cnt+":"+value+":"+a.length+":"+s.length;
 }} `
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "0:2:400:400:6\n" {
		t.Fatal(got)
	}
}
