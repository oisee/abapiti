package tsfront

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

const familySCCMutual = `type U = {k:'u';a:V|undefined;n:number} | {k:'w'};
type V = {k:'v';b:U|undefined;s:string} | {k:'z'};
`

// familySCCProbe loads sources and returns a fresh lowerer per call plus the
// alias types by name.
type familySCCProbe struct {
	project *Program
	file    *ast.SourceFile
	ck      *checker.Checker
	types   map[string]*checker.Type
}

func loadFamilySCCProbe(t *testing.T, sources map[string]string) *familySCCProbe {
	t.Helper()
	dir := t.TempDir()
	sources["tsconfig.json"] = `{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`
	for name, body := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	project, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := &familySCCProbe{project: project, types: map[string]*checker.Type{}}
	file, ok := project.File("probe.ts")
	if !ok {
		t.Fatal("missing probe.ts")
	}
	ck, done := project.prog.GetTypeCheckerForFile(context.Background(), file)
	t.Cleanup(done)
	p.file, p.ck = file, ck
	names := []string{}
	for name := range sources {
		if strings.HasSuffix(name, ".ts") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		other, ok := project.File(name)
		if !ok {
			t.Fatal("missing", name)
		}
		prefix := ""
		if name != "probe.ts" {
			prefix = name + ":"
		}
		for _, stmt := range other.Statements.Nodes {
			if stmt.Kind == ast.KindTypeAliasDeclaration {
				p.types[prefix+stmt.Name().Text()] = ck.GetTypeFromTypeNode(stmt.AsTypeAliasDeclaration().Type)
			}
		}
	}
	return p
}

func (p *familySCCProbe) lowerer() *lowerer {
	return &lowerer{prog: p.project, ck: p.ck, file: p.file}
}

// cycle lists the alias, its recursive alternative, the unaliased union
// reached from that alternative and the property types along the cycle.
func (p *familySCCProbe) cycle(t *testing.T, name string) []*checker.Type {
	t.Helper()
	alias := p.types[name]
	if alias == nil {
		t.Fatal("missing", name)
	}
	out := []*checker.Type{alias}
	for _, alt := range alias.AsUnionOrIntersectionType().Types() {
		out = append(out, alt)
		for _, prop := range p.ck.GetPropertiesOfType(alt) {
			out = append(out, p.ck.GetTypeOfPropertyOfType(alt, prop.Name))
		}
	}
	return out
}

func (p *familySCCProbe) ids(l *lowerer, types []*checker.Type) []string {
	var ids []string
	for _, typ := range types {
		ids = append(ids, l.familyTypeKey(typ, false, nil), l.familyTypeKey(typ, true, nil))
	}
	return ids
}

// (a) U and V form one SCC; ids do not depend on which member is keyed first.
func TestFamilySCCEntryIndependent(t *testing.T) {
	p := loadFamilySCCProbe(t, map[string]string{"probe.ts": familySCCMutual})
	all := append(p.cycle(t, "U"), p.cycle(t, "V")...)
	var want []string
	for entry := range all {
		l := p.lowerer()
		order := append(append([]*checker.Type(nil), all[entry:]...), all[:entry]...)
		for _, typ := range order { // warm the cache from this entry
			l.familyTypeKey(typ, false, nil)
		}
		got := p.ids(l, all)
		if len(l.diags) != 0 {
			t.Fatal(l.diags)
		}
		if want == nil {
			want = got
			continue
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("entry %d changed ids:\n%v\n%v", entry, got, want)
		}
	}
}

// Ids are deterministic: 20 fresh computations from rotating entries agree.
func TestFamilySCCDeterministic(t *testing.T) {
	p := loadFamilySCCProbe(t, map[string]string{"probe.ts": familySCCSynthetic(12, 3)})
	var all []*checker.Type
	for i := 0; i < 12; i++ {
		all = append(all, p.cycle(t, fmt.Sprintf("S%d", i))...)
	}
	var want string
	for run := 0; run < 20; run++ {
		l := p.lowerer()
		entry := (run * 7) % len(all)
		l.familyTypeKey(all[entry], run%2 == 0, nil)
		got := strings.Join(p.ids(l, all), ",")
		if run == 0 {
			want = got
		} else if got != want {
			t.Fatalf("run %d (entry %d) changed ids", run, entry)
		}
	}
}

// (b) Isomorphic cycles stay apart when their symbols differ (other file) and
// coincide when their symbols agree (same file), exactly as #186 related them.
func TestFamilySCCSymbolsAnchorIsomorphicCycles(t *testing.T) {
	twin := strings.NewReplacer("type U", "type P", "type V", "type Q", "V|", "Q|", "U|", "P|").Replace(familySCCMutual)
	other := `type X = {k:'u';a:Y|undefined;n:string} | {k:'w'};
type Y = {k:'v';b:X|undefined;s:string} | {k:'z'};
`
	p := loadFamilySCCProbe(t, map[string]string{"probe.ts": familySCCMutual + twin + other, "b.ts": strings.ReplaceAll(twin, "type ", "export type ")})
	l := p.lowerer()
	key := func(typ *checker.Type) (string, string) {
		backs := 0
		return l.familyTypeKey(typ, false, nil), l.familyTypeKeyMerkle186(typ, false, map[*checker.Type]bool{}, time.Now().Add(time.Minute), &backs)
	}
	otherFile := p.types["b.ts:P"]
	u, uOld := key(p.types["U"])
	pSame, pOld := key(p.types["P"])
	x, xOld := key(p.types["X"])
	pOther, pOtherOld := key(otherFile)
	if (u == pSame) != (uOld == pOld) || u != pSame {
		t.Fatalf("same-file isomorphic cycles: scc %t, #186 %t", u == pSame, uOld == pOld)
	}
	if (u == pOther) != (uOld == pOtherOld) || u == pOther {
		t.Fatalf("other-file isomorphic cycles: scc %t, #186 %t", u == pOther, uOld == pOtherOld)
	}
	if (u == x) != (uOld == xOld) || u == x {
		t.Fatalf("non-isomorphic cycles: scc %t, #186 %t", u == x, uOld == xOld)
	}
}

// (c) Without cycles the SCC id is the #186 Merkle id, byte for byte.
func TestFamilySCCAcyclicEqualsMerkle186(t *testing.T) {
	source := familyKeyDAG(8) + `interface Box<T> {value:T}
type G = {k:'g';box:Box<T3|undefined>;list:T2[];map:Map<string,T1>} | {k:'h'};
type I = G & {extra:number};
`
	p := loadFamilySCCProbe(t, map[string]string{"probe.ts": source})
	l := p.lowerer()
	compared := 0
	for name, typ := range p.types {
		for _, named := range []bool{false, true} {
			backs := 0
			want := l.familyTypeKeyMerkle186(typ, named, map[*checker.Type]bool{}, time.Now().Add(time.Minute), &backs)
			if backs != 0 {
				continue
			}
			if got := l.familyTypeKey(typ, named, nil); got != want {
				t.Fatalf("%s named=%t: scc %q, #186 %q", name, named, got, want)
			}
			compared++
		}
	}
	if compared < 20 || len(l.diags) != 0 {
		t.Fatalf("compared %d, diagnostics %v", compared, l.diags)
	}
}

// familySCCSynthetic builds n mutually recursive families with fan-out f.
func familySCCSynthetic(n, f int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "type S%d = {k:'s%d'", i, i)
		for j := 0; j < f; j++ {
			fmt.Fprintf(&b, ";c%d:S%d|undefined", j, (i+1+j*7)%n)
		}
		fmt.Fprintf(&b, "} | {k:'e%d';v:number};\n", i)
	}
	return b.String()
}

// (d) 40 mutually recursive families with fan-out 3: #186 explored every
// simple path; SCC condensation keys them in one linear pass.
func TestFamilySCCSyntheticBounded(t *testing.T) {
	source := familySCCSynthetic(40, 3)
	p := loadFamilySCCProbe(t, map[string]string{"probe.ts": source})
	l := p.lowerer()
	familyKeyRawEvaluation.Store(0)
	start := time.Now()
	for i := 0; i < 40; i++ {
		for _, typ := range p.cycle(t, fmt.Sprintf("S%d", i)) {
			l.familyTypeKey(typ, false, nil)
			l.familyTypeKey(typ, true, nil)
		}
	}
	elapsed := time.Since(start)
	t.Logf("40 families: %v, %d node descriptions", elapsed, familyKeyRawEvaluation.Load())
	if elapsed > 2*time.Second || len(l.diags) != 0 {
		t.Fatalf("40 families took %v, diagnostics %v", elapsed, l.diags)
	}
	backs := 0
	ref := p.lowerer()
	func() {
		defer func() {
			if _, ok := recover().(familyKeyOracleTimeout); !ok {
				t.Fatal("#186 reference finished; synthetic graph is not hard")
			}
		}()
		ref.familyTypeKeyMerkle186(p.types["S0"], false, map[*checker.Type]bool{}, time.Now().Add(2*time.Second), &backs)
	}()
	// Whole lowering of a probe using them stays bounded too.
	blocking, elapsed, _, _ := familyKeyLowerMeasured(t, source+`class Probe {static f(x:S0):number {if(x.k==='e0') return x.v; return 0;}}
`)
	t.Logf("lowering: %v, blocking %v", elapsed, blocking)
	if elapsed > 20*time.Second {
		t.Fatalf("lowering took %v", elapsed)
	}
}

// The equal/unequal relation between ids is the one #186 produced, over every
// outermost key of the family fixtures (#186 side within a 2 s budget).
func TestFamilySCCRelationOracle(t *testing.T) {
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
		"SharedRecursive":       TestFamilyKeyMemoSharedRecursiveFixture,
		"CyclicDAG": func(t *testing.T) {
			lowerFamilyKeyProbe(t, strings.Replace(familyKeyDAG(4), "type T0 = {k:'a';x:number}", "type T0 = {k:'a';x:T0|undefined}", 1))
		},
	} {
		t.Run(name, func(t *testing.T) {
			familyKeyOracle = &familyKeyOracleLog{budget: 2 * time.Second, pairs: map[familyKeyID][2]string{}}
			defer func() { familyKeyOracle = nil }()
			run(t)
			o := familyKeyOracle
			violations, compared, skipped := familyKeyOracleCheck(o.rows)
			t.Logf("compared %d, skipped %d, acyclic %d", compared, skipped, o.acyclic)
			if len(violations) != 0 || len(o.acyclicMismatches) != 0 || compared == 0 {
				t.Fatalf("relation violations %v, acyclic mismatches %v", violations, o.acyclicMismatches)
			}
		})
	}
}

// ABAPITI_FAMILY_KEY_ORACLE_CHECK=<file> checks a log written by a run with
// ABAPITI_FAMILY_KEY_ORACLE=<file> (for example the gogen pilot).
func TestFamilySCCRelationOracleFile(t *testing.T) {
	path := os.Getenv("ABAPITI_FAMILY_KEY_ORACLE_CHECK")
	if path == "" {
		t.Skip("ABAPITI_FAMILY_KEY_ORACLE_CHECK not set")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows [][2]string
	acyclic, acyclicMismatch := 0, 0
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	seen := map[[2]string]bool{}
	for sc.Scan() {
		cols := strings.SplitN(sc.Text(), "\t", 4)
		if len(cols) != 4 {
			t.Fatalf("bad row %q", sc.Text())
		}
		row := [2]string{cols[0], cols[1]}
		if seen[row] {
			continue
		}
		seen[row] = true
		rows = append(rows, row)
		if cols[2] == "acyclic" {
			acyclic++
			if cols[0] != cols[1] {
				acyclicMismatch++
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	violations, compared, skipped := familyKeyOracleCheck(rows)
	t.Logf("distinct pairs compared %d, skipped (budget) %d, acyclic %d, acyclic mismatches %d, relation violations %d", compared, skipped, acyclic, acyclicMismatch, len(violations))
	for i, v := range violations {
		if i == 20 {
			break
		}
		t.Log(v)
	}
	if len(violations) != 0 || acyclicMismatch != 0 {
		t.Fail()
	}
}
