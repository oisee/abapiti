package tsfront

import (
	"fmt"
	"testing"
)

func TestAbaplint73InferredCollectionsAndDynamicReplacement(t *testing.T) {
	source := `class BaseConf { severity: string = "Error"; }
class Conf extends BaseConf { values: string[] = []; }
interface IRule { setConfig(conf: any): void; }
class Rule implements IRule { conf: Conf = new Conf(); setConfig(conf: Conf): void { this.conf = Object.assign(new Conf(), conf); } }
class Probe {
 private static readonly names = new Set(["A", "B"]);
 private readonly values = new Set([1, 2]);
 static divide(a: number, b: number): number { return a / b; }
 static run(): string {
  const rule = new Rule();
  const original = new Conf();
  original.values.push("a");
  const clone = Object.assign(new Conf(), original);
  clone.values.push("b");
  const wrapped: IRule = rule;
  wrapped.setConfig(clone);
  const pattern = "&1";
  const regex = new RegExp(pattern, "g");
  const literal = "$\x60$'$$";
  const replaced = "&1-&1".replace(regex, () => literal);
  regex.lastIndex++;
  const previous = regex.lastIndex++;
  const next = ++regex.lastIndex;
  const items: (string | undefined)[] = [undefined, "z", "a", undefined, ""];
  const alias = items;
  items.sort();
  return rule.conf.values.join(",") + ":" + previous + ":" + next + ":" + clone.values.join(",") + ":" + original.values.join(",") + ":" + (clone !== original) + ":" + clone.severity + ":" + replaced + ":" + items.map(v => v === undefined ? "absent" : v).join("|") + ":" + (alias === items) + ":" + Probe.names.has("A") + ":" + new Probe().values.has(2) + ":" + (Probe.divide(7, 2) === 3.5);
 }
}`
	want := regexpOptionalCapturesNode(t, source)
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageOracleABAP(t, prog, want)
}

func TestAbaplint73RegExpZeroWidthAndSyntaxError(t *testing.T) {
	source := `class Probe { static run(): string {
  const pattern = "(?=TODO)";
  const r = new RegExp(pattern,"gi");
  const out: string[] = [];
  for (;;) { const m=r.exec("TODO todo"); if(m===null)break; out.push(m.index+":"+m[0].length);r.lastIndex++; }
  const bad="[";
  try { new RegExp(bad,"gi"); } catch(e) { out.push(String(e instanceof SyntaxError)); out.push((e as Error).message); }
  return out.join("|");
 } }`
	want := regexpOptionalCapturesNode(t, source)
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageOracleABAP(t, prog, want)
}

func TestAbaplint73NullishArrayReadsAndReferenceIncludes(t *testing.T) {
	source := `class A { a=1; } class B { b=1; }
 class Probe { static run(): string {
  const rows: readonly string[] = ["first"];
  let fallback=0;
  const read=(i:number)=>(rows[i] ?? (fallback++, "missing"));
  const a=new A(),b=new B(); const all=[a];
  return [read(-1),read(0),read(4),String(fallback),String(all.includes(b as unknown as A)),String(all.includes(a as A))].join("|");
 } }`
	want := regexpOptionalCapturesNode(t, source)
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageOracleABAP(t, prog, want)
}
