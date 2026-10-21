package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// An async body is outside the number-range analysis. A loop index bounded by
// one array reads another, shorter array and passes the element to `any`:
// absence flows as undefined without an integer range proof for the index.
const arrayBoundsAsyncAnyTS = `interface Stat {count:number}
function same(actual:any,expected:any):string {return actual===expected?"=":"!"+String(expected);}
async function check(stats:Stat[],lengths:number[]):Promise<string> {
 let result="";
 await Promise.resolve();
 for(let i=0;i<stats.length;i++) {result+=same(stats[i].count,lengths[i]);}
 return result;
}
export class Probe {static async run():Promise<string> {
 const stats=[{count:0},{count:1}];
 return await check(stats,[0,1])+";"+await check(stats,[0])+";"+await check(stats,[]);
}}
`

func TestStep4ArrayBoundsAsyncAnyArgument(t *testing.T) {
	p, ds := asyncProbe(t, arrayBoundsAsyncAnyTS)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	out := runGoHIR(t, p, "fmt.Println(drainSettled("+goEntry("input.ts.Probe", "run")+"()).String())")
	if strings.TrimSpace(out) != "==;=!undefined;!undefined!undefined" {
		t.Fatal(out)
	}
}
