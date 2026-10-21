package tsfront

import (
	"fmt"
	"testing"
)

// gogen's closure retry (frontend-base.mjs notInProgram, frontend-closure.mjs):
// a helper records the class and returns an Error subclass that the caller
// throws; enclosing catches filter it with instanceof, rethrow others, and the
// loop compiles again with the recorded classes.
func TestUnsupportedRetryNodeOracle(t *testing.T) {
	source := `class Unsupported extends Error {}
 class Ctx {missing:Set<string>=new Set();wanted:Set<string>=new Set(["A"]);log:string[]=[];}
 function notInProgram(ctx:Ctx, cls:string, message:string):Unsupported {
  ctx.missing.add(cls);
  return new Unsupported(message);
 }
 function slot(ctx:Ctx, cls:string):string {
  if (cls === "BAD") throw new Error("not a class " + cls);
  if (!ctx.wanted.has(cls)) throw notInProgram(ctx, cls, cls + " is not compiled in this program");
  return cls.toLowerCase();
 }
 function lower(ctx:Ctx, names:string[]):string {
  const parts:string[] = [];
  for (const n of names) {
   try {
    try { parts.push(slot(ctx, n)); } finally { ctx.log.push("visit " + n); }
   } catch (e) {
    if (!(e instanceof Unsupported)) throw e;
    ctx.log.push("skip " + e.message);
    parts.push("?");
   }
  }
  return parts.join(",");
 }
 class Probe {
  static run():string {
   const ctx = new Ctx();
   let out = "";
   for (let round = 1; ; round++) {
    ctx.missing = new Set();
    out = lower(ctx, ["A", "B", "C", "B"]);
    const added = [...ctx.missing].filter((n) => !ctx.wanted.has(n));
    ctx.log.push("round " + round + " " + out + " +" + added.join("+"));
    if (!added.length) break;
    added.forEach((n) => ctx.wanted.add(n));
   }
   let other = "";
   try { lower(ctx, ["A", "BAD"]); } catch (e) { other = (e instanceof Unsupported) + ":" + (e instanceof Error ? e.message : ""); }
   return ctx.log.join(";") + "|" + out + "|" + other;
  }
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
