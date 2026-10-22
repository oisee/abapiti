package tsfront

import (
	"regexp"
	"strings"
	"testing"
)

// Backends read a local's own storage instead of a copy only while no
// local can change before the use. Each case changes a local between the
// read and the use in JavaScript evaluation order; Node decides.
func TestOperandForwardingNodeOracle(t *testing.T) {
	for _, c := range []struct{ name, source, want string }{
		{"increment in a later operand", `function pair(a: number, b: number): string { return a + ":" + b; }
 class Probe { static run(): string { let i = 1; const s = pair(i, i++) + "," + pair(i, ++i); return s + "/" + i; } }`, "1:1,2:3/3"},
		{"receiver replaced by an argument", `class Box { n: number; constructor(n: number) { this.n = n; } add(k: number): number { return this.n + k; } }
 class Probe { static run(): string { let o = new Box(1); const swap = (): number => { o = new Box(10); return 5; }; const r = o.add(swap()); return r + "/" + o.n; } }`, "6/10"},
		{"field target receiver replaced by the value", `class Box { n = 0; }
 class Probe { static run(): string { let o = new Box(); const first = o; const swap = (): number => { o = new Box(); return 7; }; o.n = swap(); return first.n + "/" + o.n; } }`, "7/0"},
		{"return value survives finally", `class Probe { static f(): number { let x = 1; try { return x; } finally { x = 5; } }
 static run(): string { return "" + Probe.f(); } }`, "1"},
		{"closure writes a captured local", `function pair(a: number, b: number): string { return a + ":" + b; }
 class Probe { static run(): string { let n = 1; const inc = (): number => { n = n + 1; return n; }; return pair(n, inc()) + "/" + n; } }`, "1:2/2"},
		{"for-of keeps the original array", `class Probe { static run(): string { let xs = [1, 2, 3]; let s = 0; for (const x of xs) { xs = [100]; s = s + x; } return s + "/" + xs.length; } }`, "6/1"},
		{"while condition rereads the local", `class Probe { static run(): string { let i = 0; let s = ""; while (i < 3) { s = s + i; i = i + 1; } return s; } }`, "012"},
		{"optional argument from a present value", `class Item { v: string; constructor(v: string) { this.v = v; } }
 function show(x?: Item, flag?: boolean): string { return (x === undefined ? "-" : x.v) + (flag === undefined ? "?" : flag ? "T" : "F"); }
 class Probe { static run(): string { const it = new Item("a"); return show(it, true) + show(undefined, false) + show(it); } }`, "aT-Fa?"},
		{"copy then reassign source", `class Probe { static run(): string { let a = "x"; const b = a; a = a + "y"; let c = [1]; const d = c; c = [2, 3]; return a + b + ":" + d.length + c.length; } }`, "xyx:12"},
		{"loop temps start fresh each iteration", `class Item { v: number; constructor(v: number) { this.v = v; } }
 class Probe { static run(): string { const xs: (Item | undefined)[] = [new Item(1), undefined, new Item(3)]; let s = ""; for (const x of xs) { const y = x === undefined ? undefined : x; s = s + (y === undefined ? "u" : "" + y.v); } return s; } }`, "1u3"},
	} {
		t.Run(c.name, func(t *testing.T) { gogenOracle(t, c.source, c.want) })
	}
}

// ABAP: copies of locals, dead CLEARs outside loops and literal conditions
// disappear; a temp declared inside a loop keeps its per-iteration CLEAR.
func TestOperandForwardingABAPEmission(t *testing.T) {
	source := `class Item { v: number; constructor(v: number) { this.v = v; } name(): string { return "n" + this.v; } }
 class Holder { item: Item; constructor(i: Item) { this.item = i; } get(): Item { return this.item; } label(): string { return this.item.name(); } }
 function show(x?: Item): string { return x === undefined ? "-" : x.name(); }
 class Probe { static run(): string {
   const h = new Holder(new Item(2));
   const xs: (Item | undefined)[] = [h.get(), undefined];
   let s = show(h.get()) + h.label();
   for (const x of xs) { const y = x === undefined ? undefined : x; s = s + (y === undefined ? "u" : y.name()); }
   return s;
 } }`
	gogenOracle(t, source, "n2n2n2u")
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	files, err := emitABAP(t, p)
	if err != nil {
		t.Fatal(err)
	}
	all := ""
	for name, src := range files {
		if strings.HasSuffix(name, ".abap") {
			all += src
		}
	}
	method := func(name string) string {
		m := regexp.MustCompile(`(?s)METHOD z_member_` + name + `_[0-9a-f]+\.\n(.*?)ENDMETHOD\.`).FindStringSubmatch(all)
		if m == nil {
			t.Fatalf("no method %s in\n%s", name, all)
		}
		return m[1]
	}
	// Member reads through a local use the local itself as the receiver.
	if show := method("show"); !regexp.MustCompile(`\( t\d+->z_member_v_`).MatchString(show) {
		t.Fatalf("show copies its receiver:\n%s", show)
	}
	run := method("run")
	if !regexp.MustCompile(`\( t\d+->z_member_item_`).MatchString(run) {
		t.Fatalf("run copies its receiver:\n%s", run)
	}
	loop := regexp.MustCompile(`(?s)\n\s*LOOP AT [^\n]*\n.*\n\s*ENDLOOP\.`).FindString(run)
	outside := strings.Replace(run, loop, "", 1)
	if loop == "" || strings.Contains(outside, "CLEAR t") || !strings.Contains(loop, "CLEAR t") {
		t.Fatalf("CLEAR must stay inside the loop only:\n%s", run)
	}
	for _, m := range regexp.MustCompile(`CLEAR (t\d+)\.\n\s*CLEAR (t\d+)\.`).FindAllStringSubmatch(all, -1) {
		if m[1] == m[2] {
			t.Fatalf("double CLEAR %s:\n%s", m[1], run)
		}
	}
	if regexp.MustCompile(`(?m)^\s*IF abap_(true|false) = abap_true\.$|DATA\(t\d+\) = (CONV abap_bool\( )?abap_true( \))?\.\n\s*IF t\d+ = abap_true\.`).MatchString(all) {
		t.Fatalf("literal condition survived:\n%s", all)
	}
}
