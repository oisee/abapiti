package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const familyKeyTree = `type Tree = {k:'leaf';value:number} | {k:'branch';left:Tree;right:Tree};
`

const familyKeyProbeClass = `class Probe {
 static total(x:Tree):number {if(x.k==='leaf') return x.value; return Probe.total(x.left)+Probe.total(x.right);}
 static deep(x:{v:%s|undefined}):number {return 0;}
}
`

func lowerFamilyKeyProbe(t *testing.T, source string) []string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"probe.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	project, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, diags, err := project.Lower([]string{"probe.ts"})
	if err != nil {
		t.Fatal(err)
	}
	var blocking []string
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			blocking = append(blocking, d.Loc+" "+d.Message)
		}
	}
	return blocking
}

// familyKeyDAG builds a DAG of depth levels with fan-out 2. `T|undefined` is
// an unaliased union, so each level's structural key spells the level below
// on every path: without a bound the key grows as 3^depth.
func familyKeyDAG(depth int) string {
	var b strings.Builder
	b.WriteString(familyKeyTree)
	b.WriteString("type T0 = {k:'a';x:number} | {k:'b'};\n")
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, "type T%d = {k:'c';l:T%d|undefined;r:T%d|undefined} | {k:'d';l:T%d|undefined};\n", i, i-1, i-1, i-1)
	}
	fmt.Fprintf(&b, familyKeyProbeClass, fmt.Sprintf("T%d", depth))
	return b.String()
}

func familyKeyLowerMeasured(t *testing.T, source string) ([]string, time.Duration, uint64, int64) {
	t.Helper()
	familyKeyRawEvaluation.Store(0)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	blocking := lowerFamilyKeyProbe(t, source)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	return blocking, elapsed, after.TotalAlloc - before.TotalAlloc, familyKeyRawEvaluation.Load()
}

func TestFamilyKeyMemoDAG(t *testing.T) {
	// Below the bound the memo keeps raw evaluations linear (unmemoized: 3^4 paths).
	blocking, elapsed, alloc, evals := familyKeyLowerMeasured(t, familyKeyDAG(4))
	if len(blocking) != 0 || evals > 200 {
		t.Fatalf("depth 4: blocking %v, %d raw evaluations", blocking, evals)
	}
	t.Logf("depth 4: %v, %d MB, %d raw evaluations", elapsed, alloc>>20, evals)
	// Depth 30 grew past 43 GB before; now the size bound refuses at the type node.
	blocking, elapsed, alloc, evals = familyKeyLowerMeasured(t, familyKeyDAG(30))
	t.Logf("depth 30: %v, %d MB, %d raw evaluations", elapsed, alloc>>20, evals)
	if elapsed > 10*time.Second || alloc > 1<<30 {
		t.Fatalf("depth 30 not bounded: %v, %d MB", elapsed, alloc>>20)
	}
	want := "probe.ts:35:16 recursive family structural key exceeds 65536 bytes for type "
	if len(blocking) != 1 || !strings.HasPrefix(blocking[0], want) || !strings.HasSuffix(blocking[0], "; family lowering refused") {
		t.Fatalf("depth 30 refusal %q", blocking)
	}
}

func TestFamilyKeyDepthGuard(t *testing.T) {
	var b strings.Builder
	b.WriteString(familyKeyTree)
	b.WriteString("type L0 = {k:'a'} | {k:'z'};\n")
	const depth = 150 // union + object per level: 300 nested keys, each small
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, "type L%d = {k:'b';n:L%d|undefined} | {k:'z'};\n", i, i-1)
	}
	fmt.Fprintf(&b, familyKeyProbeClass, fmt.Sprintf("L%d", depth))
	blocking := lowerFamilyKeyProbe(t, b.String())
	want := fmt.Sprintf("probe.ts:%d:16 recursive family structural key is nested deeper than 256 levels for type ", depth+5)
	if len(blocking) != 1 || !strings.HasPrefix(blocking[0], want) {
		t.Fatalf("depth guard %q", blocking)
	}
	if text := strings.TrimSuffix(strings.TrimPrefix(blocking[0], want), "; family lowering refused"); len(text) > 203 {
		t.Fatalf("type text not truncated: %d", len(text))
	}
}

// A recursive subtype whose key contains its own back-reference, shared at two
// stack depths, plus a mutual cycle entered from different members.
func TestFamilyKeyMemoSharedRecursiveFixture(t *testing.T) {
	familyKeyCrossCheckRun(t, func(t *testing.T) {
		source := familyKeyTree + `type R = {k:'a';next:R|undefined;n:number} | {k:'b';v:number};
type U = {k:'u';a:V|undefined} | {k:'w'};
type V = {k:'v';b:U|undefined} | {k:'z'};
type Root = {k:'root';a:R|undefined;deep:{x:{y:{z:R|undefined}}}};
class Probe {
 static total(x:Tree):number {if(x.k==='leaf') return x.value; return Probe.total(x.left)+Probe.total(x.right);}
 static v(x:{p:{k:'v';b:U|undefined}}):number {return 0;}
 static u(x:{q:U|undefined}):number {return 0;}
 static w(x:{q:V|undefined}):number {return 0;}
 static root(x:{r:Root|undefined;s:R|undefined}):number {return 0;}
 static r(x:R):number {if(x.k==='b') return x.v; return x.n;}
}
`
		lowerFamilyKeyProbe(t, source)
	})
}

// Every outermost key in the existing family fixtures (those that compute
// family keys at all) is computed
// again without the memo and must be byte-identical.
func TestFamilyKeyMemoMatchesPlain(t *testing.T) {
	for name, run := range map[string]func(*testing.T){
		"TreeNodeOracle":        TestRecursiveFamilyTreeNodeOracle,
		"MutualNodeOracle":      TestRecursiveFamilyMutualNodeOracle,
		"AtomicRefusal":         TestRecursiveFamilyAtomicRefusal,
		"AnonymousLayoutsNode":  TestRecursiveFamilyAnonymousLayoutsNodeOracle,
		"GenericIdentitiesNode": TestRecursiveFamilyGenericIdentitiesNodeOracle,
		"WrongTagRefusal":       TestRecursiveFamilyWrongTagRefusal,
		"PayloadRefusals":       TestRecursiveFamilyPayloadRefusals,
		"SubsetsNodeOracle":     TestRecursiveSubsetsNodeOracle,
		"SubsetsInvalidPayload": TestRecursiveSubsetsInvalidPayloadRefusal,
		"SubsetsLivePayload":    TestRecursiveSubsetsLivePayloadRefusalNodeOracle,
		"SubsetFeatureRefusals": TestRecursiveSubsetFeatureRefusalsNodeOracle,
		"DAGBelowBound":         func(t *testing.T) { lowerFamilyKeyProbe(t, familyKeyDAG(4)) },
	} {
		t.Run(name, func(t *testing.T) { familyKeyCrossCheckRun(t, run) })
	}
}

func familyKeyCrossCheckRun(t *testing.T, run func(*testing.T)) {
	t.Helper()
	familyKeyCrossCheck.Store(true)
	familyKeyCrossChecked.Store(0)
	familyKeyMismatchMu.Lock()
	familyKeyMismatches = nil
	familyKeyMismatchMu.Unlock()
	defer familyKeyCrossCheck.Store(false)
	run(t)
	familyKeyMismatchMu.Lock()
	defer familyKeyMismatchMu.Unlock()
	if len(familyKeyMismatches) != 0 {
		t.Fatalf("%d memo/plain key mismatches, first: %.2000s", len(familyKeyMismatches), familyKeyMismatches[0])
	}
	if familyKeyCrossChecked.Load() == 0 {
		t.Fatal("fixture computed no family keys")
	}
}
