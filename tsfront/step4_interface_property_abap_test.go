package tsfront

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// ArtifactObject's optional unknown slots are reads of the actual instance,
// including fields outside the interface and optional prototype methods.
func TestStep4InterfacePropertyABAPNodeOracle(t *testing.T) {
	source := `interface ArtifactObject { getName(): string; syntaxResult?: unknown; definition?: unknown }
class Empty { getName(): string { return "empty"; } }
class Built extends Empty {
 syntaxResult: string | undefined = undefined;
 def: string = "definition";
 parsedType: boolean = false;
 isDirty(): boolean { return this.syntaxResult === undefined; }
}
class Derived extends Built { isDirty(): boolean { return false; } }
class Probe {
 static reads = 0;
 static recv(o: ArtifactObject & {isDirty?(): boolean}): ArtifactObject & {isDirty?(): boolean} { Probe.reads++; return o; }
 static value(o: ArtifactObject, key: string): unknown { return o[key]; }
 static describe(o: ArtifactObject & {isDirty?(): boolean}): string {
  return o.getName() + ":" + String(o.syntaxResult) + ":" + String(o.def) + ":" + String(o.definition) + ":" + String(o.parsedType) + ":" + typeof Probe.value(o, "isDirty") + ":" + String(Probe.recv(o).isDirty?.()) + ":" + String(Boolean(o.syntaxResult || o.def || o.definition || o.parsedType) || o.isDirty?.() === false);
 }
 static run(): string {
  const b = new Built();
  const before = Probe.describe(new Empty()) + "|" + Probe.describe(b) + "|" + Probe.describe(new Derived());
  b.syntaxResult = "built";
  return before + "|" + Probe.describe(b) + "|" + String(Probe.value(b, "def")) + ":" + String(Probe.value(b, "absent")) + ":" + Probe.reads;
 }
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, ok := hir.UsedRuntimeOps(p)["object.get"]; !ok {
		t.Fatal("probe did not exercise object.get")
	}
	want := strings.TrimSpace(conversionOracle(t, source))
	storageABAPProbe(t, p, "probe.ts.Probe", want, "")
}

func TestStep4InterfaceGetterABAPNodeOracle(t *testing.T) {
	source := `interface ArtifactObject { getName(): string; definition?: unknown }
class Empty { getName(): string { return "empty"; } }
class Base extends Empty {
 calls = 0;
 get definition(): string { this.calls++; return "base" + this.calls; }
 get optional(): string | undefined { this.calls++; return undefined; }
}
class Child extends Base { get definition(): string { this.calls++; return "child" + this.calls; } }
class Probe {
 static read(o: ArtifactObject, key: string): unknown { return o[key]; }
 static describe(o: ArtifactObject): string { return String(o.definition) + "/" + String(Probe.read(o, "optional")) + "/" + String(Probe.read(o, "definition")) + "/" + String(Probe.read(o, "calls")); }
 static run(): string { return Probe.describe(new Empty()) + "|" + Probe.describe(new Base()) + "|" + Probe.describe(new Child()); }
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}

func TestStep4InterfaceLivePropertyABAPNodeOracle(t *testing.T) {
	source := `interface ArtifactObject { getName(): string; syntaxResult?: unknown }
class Item { syntaxResult = "native"; getName(): string { return this.syntaxResult; } }
class Probe {
 static describe(o: ArtifactObject): string { return String(o.syntaxResult) + "/" + String(o.extra) + "/" + String(o.missing); }
 static run(): string {
  const row: any = JSON.parse('{"syntaxResult":"graph","extra":false}');
  const item: Item = row;
  const before = Probe.describe(item);
  row.syntaxResult = "changed";
  return before + "|" + Probe.describe(item);
 }
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(conversionOracle(t, source)), "")
}
