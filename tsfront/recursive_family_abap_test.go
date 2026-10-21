package tsfront

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// Car U1 of the ABAP union ABI: a union reference is the boxed live record
// (REF TO runtime.dynamic). Admission, tag reads, widening, subset projection
// and checked admission of untyped values run on ABAP; payload projection and
// common field reads stay located refusals until car U2.
const recursiveFamilyTagProbe = `type Expr = {e:'num';value:number} | {e:'add';left:Expr;right:Expr;line:number} | {e:'neg';arg:Expr;line:number};
type Bin = Extract<Expr,{e:'add'}|{e:'neg'}>;
class Probe {
 static num(value:number):Expr { return {e:'num',value:value}; }
 static add(left:Expr,right:Expr):Expr { return {e:'add',left:left,right:right,line:1}; }
 static neg(arg:Expr):Bin { return {e:'neg',arg:arg,line:2}; }
 static kind(x:Expr):string { return x.e; }
 static binKind(x:Bin):string { return 'b'+x.e; }
 static widen(x:Bin):Expr { return x; }
 static sub(x:Expr):string { if (x.e!=='num') return Probe.binKind(x); return 'n'; }
 static parse(text:string):Expr { return JSON.parse(text) as Expr; }
 static check(v:any):Expr { return v; }
 static run():string {
  const n=Probe.num(2);
  const a=Probe.add(n,Probe.neg(n));
  const w=Probe.widen(Probe.neg(a));
  const p=Probe.parse('{"e":"add","left":{"e":"num","value":1},"right":{"e":"num","value":4},"line":3}');
  const c=Probe.check(JSON.parse('{"e":"neg","arg":{"e":"num","value":1},"line":4}'));
  return Probe.kind(n)+':'+Probe.kind(a)+':'+Probe.kind(w)+':'+Probe.sub(a)+':'+Probe.sub(n)+':'+Probe.kind(p)+':'+Probe.sub(p)+':'+Probe.sub(c);
 }
 static wrong():string { return Probe.kind(Probe.parse('{"e":"mul","line":1}')); }
}`

func lowerRecursiveFamilyTagProbe(t *testing.T) *hir.Program {
	t.Helper()
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": recursiveFamilyTagProbe}, []string{"probe.ts"})
	// The frontend admits this untyped value through the Dynamic projection;
	// union.check is the same checked admission (see hir.unionOp), so the
	// probe covers both spellings.
	checked := 0
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if m.Name != "check" {
				continue
			}
			visitTestBody(m.Body, func(x *hir.Expr) {
				if x.Kind == hir.RuntimeOp && x.Op == "union.project" && x.X.Type.Kind == hir.Dynamic {
					x.Op = "union.check"
					checked++
				}
			})
		}
	}
	if checked != 1 {
		t.Fatalf("union.check sites: %d", checked)
	}
	if errs := hir.Verify(p); len(errs) != 0 {
		t.Fatal(errs)
	}
	return p
}

func cloneProgram(t *testing.T, p *hir.Program) *hir.Program {
	t.Helper()
	encoded, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var restored hir.Program
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	return &restored
}

func TestRecursiveFamilyTagABAPOracle(t *testing.T) {
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, recursiveFamilyTagProbe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerRecursiveFamilyTagProbe(t)
	ops := map[string]int{}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			visitTestBody(m.Body, func(x *hir.Expr) {
				if x.Kind == hir.RuntimeOp && strings.HasPrefix(x.Op, "union.") {
					ops[x.Op]++
				}
			})
		}
	}
	if ops["union.admit"] != 3 || ops["union.tag"] != 3 || ops["union.widen"] != 2 || ops["union.project"] != 2 || ops["union.check"] != 1 || ops["union.read"] != 0 {
		t.Fatalf("probe no longer covers the U1 operations: %v", ops)
	}
	if got := runGoHIR(t, cloneProgram(t, p), fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}

	files, names, err := emitABAPNamed(t, cloneProgram(t, p))
	if err != nil {
		t.Fatal(err)
	}
	expr, bin := names.Get("union.family.probe.ts.Expr"), ""
	for _, u := range p.Unions {
		if strings.HasPrefix(u.Name, "family.subset.") {
			bin = names.Get("union.family." + u.Name)
		}
	}
	runtime := names.Get("union.runtime")
	for _, class := range []string{expr, bin, runtime} {
		if _, ok := files[class+".clas.abap"]; !ok {
			t.Fatalf("missing union class %s", class)
		}
	}
	// One shape check per payload class, shared by both families.
	layout := regexp.MustCompile(`(?m)^\s*METHOD ` + names.Get("union.layout") + `\.$`)
	layouts := 0
	for name, src := range files {
		layouts += len(layout.FindAllString(src, -1))
		if strings.HasPrefix(name, expr) || strings.HasPrefix(name, bin) || strings.HasPrefix(name, runtime) {
			for _, line := range strings.Split(src, "\n") {
				if len(line) > 255 || strings.HasPrefix(strings.TrimSpace(line), "\"") || strings.HasPrefix(line, "*") {
					t.Fatalf("%s: long line or comment %q", name, line)
				}
			}
		}
	}
	if layouts != 3 {
		t.Fatalf("layout methods: %d", layouts)
	}
	// Alternative checks exist only for admitted alternatives and for the
	// payload guard, which uses the payload's first family, as on Go.
	alternative := func(family string, tag string) string {
		for _, u := range p.Unions {
			if u.Name != family {
				continue
			}
			for _, a := range u.Alternatives {
				if a.Tags[0] == tag {
					return names.Get("union.check." + u.Name + "/" + a.ID)
				}
			}
		}
		return "?"
	}
	subset := ""
	for _, u := range p.Unions {
		if strings.HasPrefix(u.Name, "family.subset.") {
			subset = u.Name
		}
	}
	has := func(class, method string) bool {
		return regexp.MustCompile(`(?m)^\s*METHOD ` + method + `\.$`).MatchString(files[class+".clas.abap"])
	}
	if !has(expr, "check") || !has(expr, alternative("probe.ts.Expr", "num")) || !has(expr, alternative("probe.ts.Expr", "add")) || !has(expr, alternative("probe.ts.Expr", "neg")) {
		t.Fatal("Expr family class: wrong check methods\n" + files[expr+".clas.abap"])
	}
	if !has(bin, "check") || !has(bin, alternative(subset, "neg")) || has(bin, alternative(subset, "add")) {
		t.Fatal("Bin family class: wrong check methods\n" + files[bin+".clas.abap"])
	}

	// A wrong tag refuses like Go's RangeFault; Node does not validate casts.
	storageABAPProbe(t, cloneProgram(t, p), "probe.ts.Probe", strings.TrimSpace(string(want)), "wrong", "cx_sy_range_out_of_bounds")
}

// Car U2: payload projection and common field reads (including optional
// and union-typed reads) run on ABAP, and the audit holds no union refusal.
func TestRecursiveFamilyProjectionReadABAPOracle(t *testing.T) {
	source := `type Tree = {k:'leaf';value:number;note?:string} | {k:'branch';left:Tree;right:Tree;line:number;note?:string} | {k:'deep';left:Tree;line:number};
class Probe {
 static total(x:Tree):number {if(x.k==='leaf') return x.value; return Probe.total(x.left);}
 static line(x:Tree):number {if(x.k==='leaf') return 0; return x.line;}
 static first(x:Tree):string {if(x.k==='leaf') return 'leaf'; return x.left.k;}
 static note(x:Tree):string {if(x.k==='deep') return 'deep'; return x.note===undefined?'none':x.note;}
 static run():string {const t:Tree={k:'branch',left:{k:'leaf',value:2,note:'n'},right:{k:'leaf',value:3},line:1};const d:Tree=JSON.parse('{"k":"deep","left":{"k":"leaf","value":5},"line":4}');
  return Probe.total(t)+':'+Probe.line(t)+':'+Probe.first(t)+':'+Probe.note(t)+':'+Probe.note(t.left)+':'+Probe.total(d)+':'+Probe.line(d)+':'+Probe.first(d)+':'+Probe.note(d);}
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	ops := map[string]bool{}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			visitTestBody(m.Body, func(x *hir.Expr) {
				if x.Kind == hir.RuntimeOp && x.Op == "union.project" && x.Type.Kind == hir.ClassRef {
					ops["project"] = true
				}
				if x.Kind == hir.RuntimeOp && x.Op == "union.read" {
					ops[x.Op+":"+string(x.Type.Kind)] = true
				}
			})
		}
	}
	if !ops["project"] || !ops["union.read:number"] || !ops["union.read:unionref"] {
		t.Fatalf("probe lost its projection or read sites: %v", ops)
	}
	if got := runGoHIR(t, cloneProgram(t, p), fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	hir.AssignSiteIDs(p)
	for _, err := range abap.Audit(cloneProgram(t, p)) {
		if strings.Contains(err.Error(), "union.") {
			t.Fatalf("union refusal: %v", err)
		}
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "")
}

// Checkpoint 4 (live tag-proof lifetime): a retained payload view must refuse
// after its tag changes through an untyped alias, on ABAP as on Go.
func TestRecursiveFamilyRetainedViewABAPRefusal(t *testing.T) {
	for _, tag := range []string{"wrong", "branch"} {
		t.Run(tag, func(t *testing.T) {
			source := `type Tree={k:'leaf';value:number}|{k:'branch';left:Tree;right:Tree};
class Probe {static total(t:Tree):number{if(t.k==='leaf')return t.value;return Probe.total(t.left)+Probe.total(t.right);}
static run():number{const leaf:Tree={k:'leaf',value:7};const cached=leaf;return cached.value+Probe.total(leaf);}
static wrong():number{const leaf:Tree={k:'leaf',value:7};const cached=leaf;const erased:any=leaf;erased.k='TAG';return cached.value;}}`
			source = strings.Replace(source, "'TAG'", "'"+tag+"'", 1)
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			got := runGoHIR(t, cloneProgram(t, p), fmt.Sprintf(`fmt.Println(%s());defer func(){if r:=recover();r!=nil{if _,ok:=r.(rangeFault);!ok{panic(r)};fmt.Println("rejected")}}();fmt.Println(%s())`, goEntry("probe.ts.Probe", "run"), goEntry("probe.ts.Probe", "wrong")))
			if got != "14\nrejected\n" {
				t.Fatalf("Go: %q", got)
			}
			abapUnitProbe(t, p, "probe.ts.Probe", "14", "f", "wrong", "cx_sy_range_out_of_bounds")
		})
	}
}

// abapFamilyOracle runs a family program on ABAP when it emits (compared with
// Node on OSGo under ABAPITI_OSG_HOME). An unsupported runtime operation must
// refuse before any output; any other emitter refusal must name its site.
func abapFamilyOracle(t *testing.T, p *hir.Program, want string) {
	t.Helper()
	supported, actual := true, "string"
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if c.Name == "probe.ts.Probe" && m.Name == "run" && m.Result.Kind == hir.Number {
				actual = "f"
			}
			visitTestBody(m.Body, func(x *hir.Expr) {
				if x.Kind == hir.RuntimeOp && !abap.SupportsRuntimeOp(x.Op) {
					supported = false
				}
			})
		}
	}
	_, err := emitABAP(t, cloneProgram(t, p))
	if !supported {
		if err == nil {
			t.Fatal("ABAP accepted an unsupported union operation")
		}
		if files, _, err := emitABAPTestNamed(t, cloneProgram(t, p), map[string]bool{}); err == nil || len(files) != 0 {
			t.Fatal("ABAP test emission did not refuse before body traps", err)
		}
		return
	}
	if err != nil {
		if !strings.Contains(err.Error(), "probe.ts:") {
			t.Fatalf("unlocated ABAP refusal: %v", err)
		}
		return
	}
	abapUnitProbe(t, cloneProgram(t, p), "probe.ts.Probe", strings.TrimSpace(want), actual, "")
}

// gogenABAPFamilyOracle is gogenTSOracle for family programs that also run on
// ABAP (OSGo under ABAPITI_OSG_HOME), equal to Node.
func gogenABAPFamilyOracle(t *testing.T, source string) {
	t.Helper()
	gogenTSOracle(t, source, "")
	abapFamilyOracle(t, lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"}), conversionOracle(t, source))
}
