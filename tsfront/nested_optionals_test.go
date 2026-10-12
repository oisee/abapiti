package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestOptionalMapValuesNodeOracle(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	const source = `
 class Item { child?: Item; label?: string; constructor(label?: string) { this.label = label; } }
 class Probe {
 static trace = "";
 static receiver(m?: Map<string, Item | undefined>): Map<string, Item | undefined> | undefined { Probe.trace += "r"; return m; }
 static key(): string { Probe.trace += "k"; return "value"; }
 static arg(): string { Probe.trace += "a"; return "!"; }
 static text(value?: string): string { return value === undefined ? "absent" : "[" + value + "]"; }
 static run(): string {
 const refs = new Map<string, Item | undefined>();
 const nums = new Map<string, number | undefined>();
 const texts = new Map<string, string | undefined>();
 const flags = new Map<string, boolean | undefined>();
 refs.set("undefined", undefined); nums.set("undefined", undefined); texts.set("undefined", undefined); flags.set("undefined", undefined);
 nums.set("zero", 0); texts.set("empty", ""); flags.set("false", false);
 const item = new Item(""); item.child = new Item("child"); refs.set("value", item);
 const missing = refs.get("missing"); const stored = refs.get("undefined");
 const a = Probe.receiver()?.get(Probe.key())?.label;
 const b = Probe.receiver(refs)?.get(Probe.key())?.child?.label;
 const c = Probe.receiver(refs)?.get("missing")?.label?.concat(Probe.arg());
 const d = refs.get("value")?.label?.concat(Probe.arg());
 const n = nums.get("zero"); const s = texts.get("empty"); const f = flags.get("false");
 return [refs.has("missing") ? "bad" : "missing", refs.has("undefined") ? "stored" : "bad",
 missing === undefined && stored === undefined ? "undefined" : "bad",
 refs.get("value") === item ? "identity" : "bad",
 nums.get("missing") === undefined && nums.get("undefined") === undefined ? "numbers" : "bad",
 texts.get("missing") === undefined && texts.get("undefined") === undefined ? "strings" : "bad",
 flags.get("missing") === undefined && flags.get("undefined") === undefined ? "booleans" : "bad",
 n === 0 && s === "" && f === false ? "present" : "bad", Probe.text(a), Probe.text(b), Probe.text(c), Probe.text(d), Probe.trace].join("|");
 }
 }`
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if strings.Contains(hir.Dump(prog), "optional<optional<") {
		t.Fatal("nested optional lookup")
	}
	files, err := abap.Emit(prog)
	if err != nil {
		t.Fatal(err)
	}
	maps, consumers := 0, 0
	for _, text := range files {
		if strings.Contains(text, "READ TABLE entries WITH KEY by_k COMPONENTS k = p0 INTO DATA(row).") && strings.Contains(text, "result->has = abap_true.") {
			maps++
			if !strings.Contains(text, "result->has = abap_true.") || !strings.Contains(text, "result->value = row-v.") {
				t.Fatal("physical map getter must retain membership", text)
			}
		}
		if strings.Contains(text, "->get( p0 = ") && strings.Contains(text, " IS BOUND.") && strings.Contains(text, "->value.") {
			consumers++
		}
	}
	if consumers == 0 {
		t.Fatal("optional map reads must unwrap a bound lookup box")
	}
	if maps < 4 {
		t.Fatalf("emitted %d optional maps, want at least 4", maps)
	}
	cmd := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]) + "\nconsole.log(Probe.run());")`, source)
	want, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	const expected = "missing|stored|undefined|identity|numbers|strings|booleans|present|absent|[child]|absent|[!]|rrkra\n"
	if string(want) != expected {
		t.Fatalf("unexpected Node oracle %q", want)
	}
	if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestOptionalRecordFieldsNodeOracle(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "1")
	const source = `
 class Row { fields?: {[key: string]: string | undefined}; }
 class Probe {
 static text(value?: string): string { return value === undefined ? "absent" : "[" + value + "]"; }
 static read(row?: Row): string { return [Probe.text(row?.fields?.missing), Probe.text(row?.fields?.empty), Probe.text(row?.fields?.["empty"])].join("/"); }
 static run(): string { const row = new Row(); const empty = new Row(); row.fields = {}; row.fields["empty"] = ""; row.fields["undefined"] = undefined;
 return [Probe.read(), Probe.read(empty), Probe.read(row), Probe.text(row.fields.undefined)].join("|"); }
 }`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if strings.Contains(hir.Dump(p), "optional<optional<") {
		t.Fatal("nested record optional")
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1]) + "\nconsole.log(Probe.run());")`, source)
	want, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if string(want) != "absent/absent/absent|absent/absent/absent|absent/[]/[]|absent\n" {
		t.Fatal(string(want))
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
