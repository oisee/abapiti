package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestStep4FreshArrayWidensNestedFamily(t *testing.T) {
	source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + `
function widen(rows: readonly IRMethod[]): void {const wide:(Method|Unsupported)[] = rows.map((row) => row); Probe.result = wide.map((row)=>row.kind).join(",")}
class Probe {static result:string;static run():string {const a:IRMethod={kind:"method",name:"a",extra:true};widen([a]);return Probe.result}}
`
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, false)
}

func TestStep4ReadonlyFieldAcceptsMutableNarrowArray(t *testing.T) {
	source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + `
class Probe {static run():string {const rows:IRMethod[]=[{kind:"method",name:"a",extra:true}];const cls:Class={signatures:new Map(),methods:new Set(),rows};return cls.rows.map((row)=>row.kind).join(",")}}
`
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, true)
}

func TestStep4ReassignedArrayLosesFreshWidening(t *testing.T) {
	source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + `
function read(rows:(Method|Unsupported)[]):string {return rows.map((row)=>row.kind).join(",")}
class Probe {static run():string {const row:IRMethod={kind:"method",name:"a",extra:true};const alias:IRMethod[]=[row];let rows:IRMethod[]=[row].map((item)=>item);rows=alias;return read(rows)}}
`
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"files":["probe.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	program, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	lowered, _, err := program.Lower([]string{"probe.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hir.Verify(lowered)) == 0 {
		t.Fatal("reassigned array retained fresh-container widening")
	}
}

func TestStep4NestedReadonlyFieldCheckedClassArgument(t *testing.T) {
	source := strings.ReplaceAll(readonlyCovarianceTypes, "export ", "") + `
interface NarrowCarrier {readonly rows:readonly IRMethod[]}
interface WideCarrier {readonly rows:readonly (Method|Unsupported)[]}
function read(carrier:WideCarrier):string {return carrier.rows.map((row)=>row.kind).join(",")}
class Probe {static run():string {const row:IRMethod={kind:"method",name:"a",extra:true};return read({rows:[row]})}}
`
	readonlyStrictCheck(t, map[string]string{"probe.ts": source})
	readonlyGoOracle(t, source, false)
}
