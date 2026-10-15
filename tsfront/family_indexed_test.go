package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Index-signature fields (Record, Readonly<Record>, {[k:string]:T}) and
// homogeneous tuples inside a recursive family. Records hold both a missing
// key and a key whose value is undefined; they are mutated after sharing.
const familyIndexedSource = `type V = string | number | {readonly [name: string]: V | undefined};
type A = {k:'s'; v: number; pairs?: readonly (readonly [string, string])[]; inits?: Readonly<Record<string, A | undefined>>; counts: {[k: string]: number}}
  | {k:'u'; kids: A[]; lit?: {readonly [name: string]: string | undefined}};
class Probe {
 static rec(r: Readonly<Record<string, A | undefined>>):string {
  const parts:string[] = [];
  parts.push(String('x' in r)); parts.push(String('y' in r)); parts.push(Object.keys(r).join(',')); parts.push(JSON.stringify(r));
  for (const key of Object.keys(r)) { const value = r[key]; parts.push(key+'='+(value === undefined ? 'undef' : Probe.show(value))); }
  parts.push(String(r['x'] === undefined)); parts.push(String(r['y'] === undefined));
  return parts.join(';');
 }
 static show(a:A):string {
  const parts:string[] = [];
  parts.push(JSON.stringify(a));
  if (a.k==='s') {
   if (a.pairs) for (const p of a.pairs) parts.push(p[0]+'>'+p[1]+':'+p.length);
   if (a.inits) parts.push('{'+Probe.rec(a.inits)+'}');
   parts.push(Object.keys(a.counts).join(','));
   a.counts['z'] = 9;
   parts.push(JSON.stringify(a.counts)); parts.push(String('z' in a.counts));
  }
  if (a.k==='u') {
   if (a.lit) { parts.push(String('m' in a.lit)); parts.push(Object.keys(a.lit).join(',')); parts.push(JSON.stringify(a.lit)); parts.push(String(a.lit['m'] === undefined)); }
   for (const k of a.kids) parts.push('['+Probe.show(k)+']');
  }
  return parts.join('|');
 }
 static run():string {
  const leaf:A = {k:'s', v:1, counts:{b:2, a:1}};
  const missing:A = {k:'s', v:2, pairs:[['a','b'],['c','d']], inits:{y: leaf}, counts:{}};
  const undef:A = {k:'s', v:3, inits:{x: undefined, y: leaf}, counts:{q:1}};
  const u:A = {k:'u', kids:[leaf, missing, undef], lit:{m: undefined, n: 'v'}};
  return Probe.show(u);
 }
}`

func TestFamilyIndexedFieldsNodeOracle(t *testing.T) {
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, familyIndexedSource).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	for _, s := range []string{"{false;true;y;", "{true;true;x,y;", "x=undef", "|true|m,n|{\"n\":\"v\"}|true|", "a>b:2"} {
		if !strings.Contains(string(want), s) {
			t.Fatalf("Node output misses %q: %s", s, want)
		}
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": familyIndexedSource}, []string{"probe.ts"})
	if len(p.Unions) != 1 || !p.Unions[0].Complete {
		t.Fatal("family with indexed fields was not published")
	}
	inits := hir.T(hir.Optional, hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.Optional, hir.Type{Kind: hir.UnionRef, Name: "probe.ts.A"})))
	found := false
	for _, a := range p.Unions[0].Alternatives {
		for _, f := range shapeClassOf(p.Classes, a.Payload).Fields {
			if f.Name == "inits" {
				found = f.Type.Equal(inits)
			}
		}
	}
	if !found {
		t.Fatal("inits is not an ordered map of the optional family")
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
	if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "union.") {
		t.Fatalf("ABAP did not explicitly refuse the recursive family ABI: %v", err)
	}
}

func TestFamilyIndexedRefusals(t *testing.T) {
	for _, tc := range []struct{ name, field, body, want string }{
		{"mixed-index-and-properties", "m: {n: number; [k: string]: number}", "", "callable or indexed layout"},
		{"callable", "f: (x: number) => number", "", "callable or indexed layout"},
		{"heterogeneous-tuple", "t: [string, number]", "", "heterogeneous tuple layout"},
		{"optional-tuple", "t: [string, string?]", "", "tuple layout with optional or rest elements"},
		{"entries-undefined-values", "r: Record<string, A | undefined>", "if (a.k==='s') return Object.entries(a.r).length;", "Object.entries optional record values"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "type A = {k:'s'; " + tc.field + "} | {k:'u'; kids: A[]};\nclass Probe {static f(a: A): number {" + tc.body + "return 0;}}"
			for _, d := range lowerFamilyDiagnostics(t, source) {
				if strings.HasPrefix(d.Category, "note-") || !strings.Contains(d.Message, tc.want) {
					continue
				}
				if strings.Contains(tc.want, "layout") && !strings.Contains(d.Message, "for member ") {
					t.Fatalf("refusal does not name the member: %s", d)
				}
				if !strings.HasPrefix(d.Loc, "probe.ts:") {
					t.Fatalf("refusal has no location: %s", d)
				}
				return
			}
			t.Fatalf("missing refusal %q", tc.want)
		})
	}
}
