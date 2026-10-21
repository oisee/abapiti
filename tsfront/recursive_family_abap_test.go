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

// The ABAP union ABI: a union reference is the payload object behind one
// payload interface (kind, sig, tag). Admission, tag reads, widening, subset
// projection and checked admission of untyped values run on ABAP.
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
	runtime, payload := names.Get("union.runtime"), names.Get("union.payload")
	for _, class := range []string{expr, bin, runtime} {
		if _, ok := files[class+".clas.abap"]; !ok {
			t.Fatalf("missing union class %s", class)
		}
	}
	if _, ok := files[payload+".intf.abap"]; !ok {
		t.Fatal("missing union payload interface")
	}
	// Representation C: every payload class implements the one payload
	// interface once; a layout check per payload class serves from_dynamic.
	layout := regexp.MustCompile(`(?m)^\s*METHOD ` + names.Get("union.layout") + `\.$`)
	implements := regexp.MustCompile(`(?m)^\s*INTERFACES ` + payload + `\.$`)
	layouts, implementations := 0, 0
	for name, src := range files {
		layouts += len(layout.FindAllString(src, -1))
		implementations += len(implements.FindAllString(src, -1))
		if strings.HasPrefix(name, expr) || strings.HasPrefix(name, bin) || strings.HasPrefix(name, runtime) || strings.HasPrefix(name, payload) {
			for _, line := range strings.Split(src, "\n") {
				if len(line) > 255 || strings.HasPrefix(strings.TrimSpace(line), "\"") || strings.HasPrefix(line, "*") {
					t.Fatalf("%s: long line or comment %q", name, line)
				}
			}
			if strings.Contains(src, "cx_sy_move_cast_error") {
				t.Fatalf("%s: cast probe in union class", name)
			}
		}
	}
	if layouts != 3 || implementations != 3 {
		t.Fatalf("layout methods: %d, payload interface implementations: %d", layouts, implementations)
	}
	// Exact alternatives need no per-alternative check: admission and widening
	// are assignments, the subset projection tests the membership signature.
	has := func(class, method string) bool {
		return regexp.MustCompile(`(?m)^\s*METHOD ` + method + `\.$`).MatchString(files[class+".clas.abap"])
	}
	for _, u := range p.Unions {
		for _, a := range u.Alternatives {
			class := names.Get("union.family." + u.Name)
			if has(class, names.Get("union.check."+u.Name+"/"+a.ID)) {
				t.Fatalf("%s: check for exact alternative %s", u.Name, a.ID)
			}
		}
	}
	if !has(expr, "from_dynamic") || !has(bin, "check") || has(bin, "from_dynamic") {
		t.Fatal("family classes: wrong methods\n" + files[expr+".clas.abap"] + files[bin+".clas.abap"])
	}
	if strings.Contains(files[runtime+".clas.abap"], "METHOD box.") {
		t.Fatal("union runtime still boxes admitted payloads")
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

// A union-heavy probe for representation C: every union operation (admit,
// tag, payload and subset projection, widen, common reads of scalar and
// optional fields, checked admission of parsed JSON), union references in an
// array, in a Map and in JSON.stringify, identity through an array, and own-key
// order of a payload. ABAP on OSGo equals Node.
const unionHeavyProbe = `type Meta = {line:number;label?:string};
type Expr = Meta & ({e:'num';value:number} | {e:'add';left:Expr;right:Expr;note?:string} | {e:'neg';arg:Expr} | {e:'list';items:Expr[];names:Map<string,Expr>});
type Bin = Extract<Expr,{e:'add'}|{e:'neg'}>;
class Probe {
 static calc(x:Expr):number { switch(x.e){case 'num':return x.value;case 'add':return Probe.calc(x.left)+Probe.calc(x.right);case 'neg':return 0-Probe.calc(x.arg);case 'list':{let s=0;for(const i of x.items){s=s+Probe.calc(i);}for(const v of x.names.values()){s=s+Probe.calc(v);}return s;}} return -1; }
 static label(x:Expr):string { return x.label===undefined ? 'none' : x.label; }
 static lineOf(x:Expr):number { return x.line; }
 static note(x:Expr):string { if (x.e==='add' && x.note!==undefined) return x.note; return '-'; }
 static bin(x:Bin):string { return x.e+x.line; }
 static widen(x:Bin):Expr { return x; }
 static sub(x:Expr):string { if (x.e==='add'||x.e==='neg') return Probe.bin(x); return 'o'; }
 static parse(t:string):Expr { return JSON.parse(t) as Expr; }
 static run():string {
  const n:Expr={e:'num',value:2,line:1};
  const g:Expr={e:'neg',arg:n,line:3,label:'g'};
  const a:Expr={e:'add',left:n,right:g,line:2,note:'hi'};
  const names=new Map<string,Expr>();
  names.set('a',a);
  const l:Expr={e:'list',items:[n,a],names:names,line:4};
  const p=Probe.parse('{"e":"add","left":{"e":"num","value":5,"line":7},"right":{"e":"neg","arg":{"e":"num","value":1,"line":8},"line":9},"line":6}');
  const w=g.e==='neg' ? Probe.widen(g) : n;
  const same=l.e==='list' && l.items[0]===n;
  return Probe.calc(l)+':'+Probe.lineOf(a)+':'+Probe.note(a)+':'+Probe.note(n)+':'+Probe.sub(a)+':'+Probe.sub(n)+':'+w.e+':'+Probe.calc(p)+':'+Probe.lineOf(p)+':'+Probe.sub(p)+':'+Probe.note(p)+':'+Probe.label(g)+':'+Probe.label(a)+':'+JSON.stringify(g)+':'+same+':'+Object.keys(a).join(',');
 }
}`

func TestRecursiveFamilyUnionHeavyABAPOracle(t *testing.T) {
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, unionHeavyProbe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": unionHeavyProbe}, []string{"probe.ts"})
	ops := map[string]bool{}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			visitTestBody(m.Body, func(x *hir.Expr) {
				if x.Kind == hir.RuntimeOp && strings.HasPrefix(x.Op, "union.") {
					key := x.Op
					if x.Op == "union.project" || x.Op == "union.read" {
						key += ":" + string(x.Type.Kind)
					}
					if x.X != nil && x.X.Type.Kind == hir.Dynamic {
						key += ":dynamic"
					}
					ops[key] = true
				}
			})
		}
	}
	for _, op := range []string{"union.admit", "union.tag", "union.widen", "union.project:classref", "union.project:unionref", "union.project:unionref:dynamic", "union.read:number", "union.read:optional"} {
		if !ops[op] {
			t.Fatalf("probe lost %s: %v", op, ops)
		}
	}
	if got := runGoHIR(t, cloneProgram(t, p), fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	hir.AssignSiteIDs(p)
	for _, err := range abap.Audit(cloneProgram(t, p)) {
		t.Fatalf("ABAP refusal: %v", err)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "")
}

// Two alternatives may share one payload class (tsfront gives each literal
// layout its own class today, so the program is rewritten to share one). The
// class then no longer proves the alternative: admission, payload projection
// and the subset check test the tag and the secondary literal on ABAP.
func TestRecursiveFamilySharedPayloadABAPOracle(t *testing.T) {
	source := `type T = {k:'a';m:'x';v:number} | {k:'b';m:'y';v:number} | {k:'c';w:string;next:T[]};
type A = Extract<T,{k:'a'}|{k:'c'}>;
class Probe {
 static f(x:A):string { if (x.k==='a') return 'a'+x.v; return x.w+x.next.length; }
 static g(x:T):string { if (x.k==='a'||x.k==='c') return Probe.f(x); return 'b'+x.v+x.m; }
 static run():string { const a:T={k:'a',m:'x',v:1}; const b:T={k:'b',m:'y',v:2}; return Probe.g(a)+Probe.g(b)+Probe.g({k:'c',w:'w',next:[a,b]}); }
 static wrong():string { const b:T={k:'b',m:'y',v:2}; const e:any=b; e.k='a'; return Probe.g(b); }
}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	var a, b string
	for _, u := range p.Unions {
		if u.Name != "probe.ts.T" {
			continue
		}
		for _, alt := range u.Alternatives {
			switch alt.Tags[0] {
			case "a":
				a = alt.Payload.Name
			case "b":
				b = alt.Payload.Name
			}
		}
	}
	encoded, err := json.Marshal(p)
	if err != nil || a == "" || b == "" {
		t.Fatal(err, a, b)
	}
	var shared hir.Program
	if err := json.Unmarshal([]byte(strings.ReplaceAll(string(encoded), b, a)), &shared); err != nil {
		t.Fatal(err)
	}
	classes := shared.Classes[:0]
	seen := false
	for _, c := range shared.Classes {
		if c.Name == a && seen {
			continue
		}
		seen = seen || c.Name == a
		classes = append(classes, c)
	}
	shared.Classes = classes
	if errs := hir.Verify(&shared); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Go's payload getters guard with the class's first alternative, so the
	// rewritten program is not a Go oracle; ABAP is compared with Node.
	files, names, err := emitABAPNamed(t, cloneProgram(t, &shared))
	if err != nil {
		t.Fatal(err)
	}
	family := files[names.Get("union.family.probe.ts.T")+".clas.abap"]
	if !strings.Contains(family, "->tag( `m` ) = `y`") || !strings.Contains(family, "CASE p0->kind( ).") {
		t.Fatal("shared payload: no value tests\n" + family)
	}
	// After a tag write through an untyped alias the tag test refuses the
	// payload as alternative a, like Go; Node does not validate.
	storageABAPProbe(t, &shared, "probe.ts.Probe", strings.TrimSpace(string(want)), "wrong", "cx_sy_range_out_of_bounds")
}

// Checkpoint 4 (live tag-proof lifetime) is a Go ABI property: Go
// re-validates the record on every payload read and rejects a retained
// payload after its tag changed through an untyped alias. On ABAP a union
// value is the payload object and its class proves membership, so the read
// follows Node, which never validates.
func TestRecursiveFamilyRetainedPayloadABAPFollowsNode(t *testing.T) {
	source := `type Tree={k:'leaf';value:number}|{k:'branch';left:Tree;right:Tree};
class Probe {static total(t:Tree):number{if(t.k==='leaf')return t.value;return Probe.total(t.left)+Probe.total(t.right);}
static run():number{const leaf:Tree={k:'leaf',value:7};const cached=leaf;const erased:any=leaf;erased.k='branch';return cached.value+Probe.total({k:'branch',left:{k:'leaf',value:2},right:{k:'leaf',value:5}});}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil || strings.TrimSpace(string(want)) != "14" {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, cloneProgram(t, p), fmt.Sprintf(`defer func(){if r:=recover();r!=nil{if _,ok:=r.(rangeFault);!ok{panic(r)};fmt.Println("rejected")}}();fmt.Println(%s())`, goEntry("probe.ts.Probe", "run")))
	if got != "rejected\n" {
		t.Fatalf("Go: %q", got)
	}
	abapUnitProbe(t, p, "probe.ts.Probe", "14", "f", "")
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
		if !strings.Contains(err.Error(), "probe.ts:") || strings.Contains(err.Error(), "union.") {
			t.Fatalf("unlocated or union ABAP refusal: %v", err)
		}
		t.Logf("located ABAP refusal: %v", err)
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
