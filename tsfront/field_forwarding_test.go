package tsfront

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// The ABAP emitter reads a plain field at its use instead of copying it
// first when nothing is evaluated in between, and runtime operations read
// their receiver/argument locals directly when they only read them. Each
// case changes the field or local between the read and the use in
// JavaScript evaluation order, or passes it where ABAP aliases it. Node
// decides; the ABAP Unit class checks the emitted ABAP on OSGO.
const fieldForwardingProbe = `class Box {
  n = 1; s = "a"; items: number[] = [1];
  static count = 0;
  bump(): number { this.n = this.n + 10; return 5; }
  swapItems(): number { this.items = [100, 200]; return 7; }
  grow(): string { this.s = this.s + "!"; return "g"; }
  plus(k: number): number { return this.n + k; }
}
class BigBox extends Box { plus(k: number): number { return this.n + k + 1000; } }
class Holder { box: Box = new Box(); maybe: Box | undefined; }
function pair(a: number, b: number): string { return a + ":" + b; }
function setAndReturn(x: number, o: Box): number { o.n = 99; return x; }
class Probe {
  static fin(o: Box): number { try { return o.n; } finally { o.n = 50; } }
  static run(): string {
    const r: string[] = [];
    let o = new Box();
    r.push(pair(o.n, o.bump()));
    o = new Box();
    r.push("" + (o.n + o.bump()));
    o = new Box();
    const keep = o.items;
    o.items.push(o.swapItems());
    r.push(keep.length + "/" + o.items.length);
    o = new Box();
    r.push("" + setAndReturn(o.n, o));
    o = new Box();
    const v = o.n;
    o.n = 9;
    r.push(v + "," + o.n);
    Box.count = 3;
    const c = Box.count;
    Box.count = Box.count + 1;
    r.push(c + "," + Box.count);
    o = new Box();
    r.push(Probe.fin(o) + "," + o.n);
    o = new Box();
    r.push(o.s + o.grow() + o.s);
    const h = new Holder();
    h.box.n = 3;
    const swap = (): number => { const nb = new Box(); nb.n = 4; h.box = nb; return 0; };
    r.push(h.box.plus(swap()) + "," + h.box.n);
    const s = "abcdef";
    const lo = -3;
    const hi = -1;
    r.push(s.slice(lo, hi) + lo + hi);
    const st = -2;
    r.push(s.substr(st, 5) + st);
    let w = 1;
    r.push("" + w + (w++) + w);
    h.maybe = h.box;
    const f = h.maybe;
    if (f !== undefined) { r.push("" + f.n); }
    let k = 0;
    while (h.box.n < 7) { h.box.n = h.box.n + 1; k = k + 1; }
    r.push("" + k);
    r.push(h.maybe === undefined ? "u" : "d");
    r.push("" + new BigBox().plus(0));
    return r.join("|");
  }
}`

func TestFieldForwardingNodeOracle(t *testing.T) {
	all := abapOracleFixture(t, fieldForwardingProbe, "1:5|6|2/2|1|1,9|3,4|1,50|aga!|3,4|de-3-1|ef-2|112|4|3|d|1001")
	// The probe exercises the forwarded shapes, not only their guards.
	for _, shape := range []string{`->value = t\d+->z_member_n_`, `xsdbool\( t\d+->z_member_box_\w+->z_member_n_`, `lines\( t\d+->z_member_items_\w+->items \)`} {
		if !regexp.MustCompile(shape).MatchString(all) {
			t.Fatalf("no %s in the emitted ABAP:\n%s", shape, all)
		}
	}
}

// A declaration or an assignment from a call builds the value in the
// variable's own storage. Nothing may observe it half-built: loops start
// each iteration fresh, and a call that throws leaves the target as it was.
const destinationProbe = `class Item { v: number; constructor(v: number) { this.v = v; } }
function fail(k: number): string { if (k > 0) { throw new Error("no"); } return "ok"; }
function twice(s: string): string { return s + s; }
class Probe {
  static run(): string {
    const r: string[] = [];
    const xs: (Item | undefined)[] = [new Item(1), undefined, new Item(3)];
    for (const x of xs) {
      const u = x === undefined;
      const w = x !== undefined ? x.v : -1;
      r.push(u ? "u" + w : "d" + w);
    }
    let flag = true;
    for (const x of xs) { flag = x === undefined; r.push(flag ? "U" : "D"); }
    let s = "a";
    try { s = fail(1); } catch (e) { s = s + "!"; }
    r.push(s);
    s = fail(0);
    r.push(s);
    s = twice(s);
    r.push(s);
    let k = 0;
    while (k < 3) { const it = new Item(k); const tag = it.v > 0 ? "p" : "z"; r.push(tag + it.v); k = k + 1; }
    return r.join("|");
  }
}`

func TestDestinationPassingNodeOracle(t *testing.T) {
	all := abapOracleFixture(t, destinationProbe, "d1|u-1|d3|D|U|D|a!|ok|okok|z0|p1|p2")
	// The loop's `const it = new Item(k)` and `s = twice(s)` write their own storage.
	for _, shape := range []string{`(?m)^\s*(t\d+) = NEW z_probe_ts_ite_\w+\( z_param_v_\w+ = t\d+ \)\.\n\s*DATA\(t\d+\) = VALUE string`, `(?m)^\s*(t\d+) = z_probe_ts_mod_\w+=>z_member_twice_\w+\( z_param_s_\w+ = t\d+ \)\.\n\s*APPEND `} {
		if !regexp.MustCompile(shape).MatchString(all) {
			t.Fatalf("no %s in the emitted ABAP:\n%s", shape, all)
		}
	}
}

// abapOracleFixture checks source against Node and Go, emits its ABAP with
// an ABAP Unit class that expects want from Probe.run (compiled by the
// fixture gate; run it with osgo:unit), and returns the emitted ABAP.
func abapOracleFixture(t *testing.T, source, want string) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err == nil {
		got, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
		if err != nil || string(got) != want+"\n" {
			t.Fatalf("Node %q (%v), want %q", got, err, want)
		}
	}
	gogenOracle(t, source, want)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	class := names.Get("probe.ts.Probe")
	driver := []string{"CLASS ltcl_oracle DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.", "PRIVATE SECTION.", "METHODS run FOR TESTING.", "ENDCLASS.", "CLASS ltcl_oracle IMPLEMENTATION.", "METHOD run.", "DATA expected TYPE string.", "DATA ch TYPE c LENGTH 1."}
	driver = append(driver, abapStringBuild("expected", "ch", want)...)
	driver = append(driver, fmt.Sprintf("cl_abap_unit_assert=>assert_equals( act = %s=>%s( ) exp = expected ).", class, names.Get("member.run")), "ENDMETHOD.", "ENDCLASS.")
	files[class+".clas.testclasses.abap"] = strings.Join(driver, "\n") + "\n"
	all := ""
	for name, src := range files {
		if strings.HasSuffix(name, ".abap") {
			all += src
		}
	}
	return all
}
