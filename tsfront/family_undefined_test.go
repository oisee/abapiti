package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// familyUndefinedSource builds both runtime variants of an exactly-undefined
// discriminant field (key absent, key present with undefined) inside a
// recursive family and observes them through in, Object.keys, JSON.stringify,
// narrowing by === undefined, truthiness, typeof and spread copies.
const familyUndefinedSource = `type A = {k:'s'; unsupported?: undefined; v: number} | {k:'r'; unsupported: undefined; w: number} | {k:'u'; unsupported: string; kids: A[]};
class Probe {
 static show(a:A):string {
  const parts:string[] = [];
  parts.push(String('unsupported' in a));
  parts.push(Object.keys(a).join(','));
  parts.push(JSON.stringify(a));
  if (a.unsupported === undefined) parts.push('undef'); else parts.push('def:'+a.unsupported);
  if (a.unsupported) parts.push('truthy'); else parts.push('falsy');
  parts.push(typeof a.unsupported);
  if (a.k==='s') {
   parts.push(String('unsupported' in a));
   parts.push(Object.keys(a).join(','));
   parts.push(JSON.stringify(a));
   parts.push(typeof a.unsupported);
   parts.push(String(a.unsupported === undefined));
   const c = {...a};
   parts.push(Object.keys(c).join(','));
   parts.push(String('unsupported' in c));
   parts.push(JSON.stringify(c));
  }
  if (a.k==='r') {
   parts.push(String('unsupported' in a));
   parts.push(Object.keys(a).join(','));
   parts.push(JSON.stringify(a));
  }
  if (a.k==='u') { for (const k of a.kids) parts.push('['+Probe.show(k)+']'); }
  return parts.join('|');
 }
 static run():string {
  const absent:A = {k:'s', v:1};
  const present:A = {k:'s', unsupported: undefined, v:2};
  const required:A = {k:'r', unsupported: undefined, w:3};
  const u:A = {k:'u', unsupported:'why', kids:[absent,present,required]};
  const empty:A = {k:'u', unsupported:'', kids:[]};
  return Probe.show(u)+'#'+Probe.show(empty);
 }
}`

func TestFamilyUndefinedFieldNodeOracle(t *testing.T) {
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, familyUndefinedSource).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	// Both runtime variants must be observable, or the oracle proves nothing.
	for _, s := range []string{"[false|k,v|", "[true|k,unsupported,v|", "[true|k,unsupported,w|"} {
		if !strings.Contains(string(want), s) {
			t.Fatalf("Node output misses variant %q: %s", s, want)
		}
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": familyUndefinedSource}, []string{"probe.ts"})
	if len(p.Unions) != 1 || !p.Unions[0].Complete {
		t.Fatal("family with exactly-undefined fields was not published")
	}
	for _, a := range p.Unions[0].Alternatives {
		c := shapeClassOf(p.Classes, a.Payload)
		for _, f := range c.Fields {
			if f.Name != "unsupported" {
				continue
			}
			if want := hir.T(hir.Optional, hir.T(hir.String)); a.Tags[0] != "u" && !f.Type.Equal(want) {
				t.Fatalf("alternative %v: unsupported is %s, want %s", a.Tags, f.Type, want)
			}
		}
	}
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
	if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "union.") {
		t.Fatalf("ABAP did not explicitly refuse the recursive family ABI: %v", err)
	}
}

func lowerFamilyDiagnostics(t *testing.T, source string) []LowerDiagnostic {
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
	return diags
}

// A union spread over an exactly-undefined field copies the JavaScript own
// keys (absent and present-undefined apart) into a snapshot record, and a
// family-typed spread result is admitted by its tag.
func TestFamilyUndefinedUnionSpreadNodeOracle(t *testing.T) {
	source := `type A = {k:'s'; unsupported?: undefined; v: number} | {k:'u'; unsupported: string; kids: A[]};
class Probe {static copy(a:A):string {const c = {...a}; return Object.keys(c).join(',')+'='+JSON.stringify(c);}
 static typed(a:A):A {return {...a};}
 static run():string {const x:A={k:'s',v:1};const y:A={k:'s',unsupported:undefined,v:2};const z:A={k:'u',unsupported:'w',kids:[x,y]};
  return [Probe.copy(x),Probe.copy(y),Probe.copy(z),Object.keys(Probe.typed(y)).join(',')].join(';');}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
}

func TestFamilyFailedComponentCascadeDiagnostic(t *testing.T) {
	source := `type Expr = {e:'n'; v:number} | {e:'s'; body: Stmt};
type Stmt = {s:'x'; expr: Expr; f: (x:number)=>number} | {s:'b'; list: Stmt[]};
class Probe {
 static first(s:Stmt):number {return 1;}
 static later(e:Expr):number {return 2;}
 static again(s:Stmt):number {return 3;}
}`
	var root, member, rootAgain string
	for _, d := range lowerFamilyDiagnostics(t, source) {
		switch {
		case strings.HasPrefix(d.Loc, "probe.ts:4:"):
			root += d.Message + "\n"
		case strings.HasPrefix(d.Loc, "probe.ts:5:"):
			member += d.Message + "\n"
		case strings.HasPrefix(d.Loc, "probe.ts:6:"):
			rootAgain += d.Message + "\n"
		}
	}
	const reason = "callable or indexed layout "
	if !strings.Contains(root, "recursive family probe.ts.Stmt has an unsupported member ("+reason) || !strings.Contains(root, "component not published") {
		t.Fatalf("root diagnostic: %q", root)
	}
	if !strings.Contains(member, "recursive family probe.ts.Expr has an unsupported member (member of failed component of probe.ts.Stmt: "+reason) {
		t.Fatalf("member diagnostic does not name the root and reason: %q", member)
	}
	if !strings.Contains(rootAgain, "recursive family probe.ts.Stmt has an unsupported member ("+reason) {
		t.Fatalf("repeated root diagnostic lost its reason: %q", rootAgain)
	}
}

// A bigint field has no HIR representation: the refusal names the member.
func TestFamilyBigintMemberNamed(t *testing.T) {
	source := `type Sql = {k:'lit'; value:bigint} | {k:'or'; l:Sql; r:Sql};
class Probe {static first(s:Sql):string {return s.k;}}`
	for _, d := range lowerFamilyDiagnostics(t, source) {
		if strings.Contains(d.Message, "unsupported checker type bigint for member") && strings.Contains(d.Message, ".value at probe.ts:1:") {
			return
		}
	}
	t.Fatalf("bigint member not named: %v", lowerFamilyDiagnostics(t, source))
}
