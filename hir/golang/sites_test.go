package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestSiteMapAndProfileSemantics(t *testing.T) {
	num := hir.T(hir.I64)
	a := &hir.Class{Name: "probe.ts.A", Ctor: &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B()}}
	get := &hir.Method{Name: "get", Virtual: true, Result: num, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(num, 7)})}
	a.Methods = []*hir.Method{get}
	b := &hir.Class{Name: "probe.ts.B", Super: a.Name}
	xs := hir.T(hir.Array, num)
	recv := hir.Ref(a.Name)
	call := &hir.Expr{Node: hir.Node{Source: "probe.ts:4:3"}, Kind: hir.VirtualCall, Name: "get", X: hir.V("receiver", recv), Type: num}
	loop := &hir.Stmt{Node: hir.Node{Source: "probe.ts:5:3"}, Kind: hir.ForEach, Name: "x", Type: num, X: hir.V("xs", xs), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: call})}
	run := &hir.Method{Name: "run", Static: true, Params: []hir.Param{{Name: "receiver", Type: recv}, {Name: "xs", Type: xs}}, Result: num, Body: hir.B(loop, &hir.Stmt{Kind: hir.Return, X: hir.L(num, 7)})}
	earlyLoop := &hir.Stmt{Node: hir.Node{Source: "probe.ts:6:3"}, Kind: hir.ForEach, Name: "x", Type: num, X: hir.V("xs", xs), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(num, 9)})}
	early := &hir.Method{Name: "early", Static: true, Params: []hir.Param{{Name: "xs", Type: xs}}, Result: num, Body: hir.B(earlyLoop, &hir.Stmt{Kind: hir.Return, X: hir.L(num, 0)})}
	alloc := &hir.Expr{Node: hir.Node{Source: "probe.ts:7:3"}, Kind: hir.New, Type: hir.Ref(b.Name)}
	makeMethod := &hir.Method{Name: "make", Static: true, Result: hir.Ref(b.Name), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: alloc})}
	i := func() *hir.Expr { return hir.V("i", num) }
	equal := func(n int64) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: i(), Y: hir.L(num, n)}
	}
	whileLoop := &hir.Stmt{Node: hir.Node{Source: "probe.ts:8:3"}, Kind: hir.While, X: &hir.Expr{Kind: hir.Binary, Op: "<", Type: hir.T(hir.Bool), X: i(), Y: hir.V("n", num)}, Body: hir.B(
		&hir.Stmt{Kind: hir.Assign, X: i(), Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: num, X: i(), Y: hir.L(num, 1)}},
		&hir.Stmt{Kind: hir.Try, Name: "err", Type: hir.T(hir.String), Body: hir.B(
			&hir.Stmt{Kind: hir.If, X: equal(2), Body: hir.B(&hir.Stmt{Kind: hir.Continue})},
			&hir.Stmt{Kind: hir.If, X: equal(3), Body: hir.B(&hir.Stmt{Kind: hir.Break})},
		), Else: hir.B()},
	)}
	whileMethod := &hir.Method{Name: "whileRun", Static: true, Params: []hir.Param{{Name: "n", Type: num}}, Result: num, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "i", Type: num, X: hir.L(num, 0)}, whileLoop, &hir.Stmt{Kind: hir.Return, X: i()})}
	a.Methods = append(a.Methods, run, early, makeMethod, whileMethod)
	p := &hir.Program{Classes: []*hir.Class{a, b}}
	plain, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	files, sites, err := EmitWithSites(p)
	if err != nil {
		t.Fatal(err)
	}
	if files["hir.go"] != "//go:build !profile_sites\n\n"+plain["hir.go"] {
		t.Fatal("default Go body changed")
	}
	again, againSites, err := EmitWithSites(p)
	if err != nil || !reflect.DeepEqual(files, again) || !reflect.DeepEqual(sites, againSites) {
		t.Fatal("nondeterministic emission", err)
	}
	for _, s := range sites.Sites {
		for backend, l := range s.Locations {
			lines := strings.Split(files[l.File], "\n")
			if l.Line < 1 || l.Line > len(lines) || strings.TrimSpace(lines[l.Line-1]) == "" {
				t.Fatalf("bad %s location %+v", backend, l)
			}
		}
	}
	dir := t.TempDir()
	names := hir.NewNames()
	main := fmt.Sprintf(`package main
 import("fmt";"os")
 func main(){siteProfileStart(os.Args[1]);defer siteProfileFinish();if siteProfileEnabled{siteProfileRead("fixture","input")};a:=%s();b:=%s();for _,n:=range []int{0,1,2,3,4,7,8,10}{xs:=&array[int64]{Items:make([]int64,n)};fmt.Println(%s(a,xs),%s(b,xs),%s(xs),%s(int64(n)))}}
 `, names.Get("new."+a.Name), names.Get("body."+a.Name+".make"), names.Get("body."+a.Name+".run"), names.Get("body."+a.Name+".run"), names.Get("body."+a.Name+".early"), names.Get("body."+a.Name+".whileRun"))
	files["main.go"] = main
	files["go.mod"] = "module fixture\n\ngo 1.26.0\n"
	for n, s := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	buildRun := func(tag string) []byte {
		t.Helper()
		binary := filepath.Join(dir, "run"+tag)
		args := []string{"build", "-o", binary}
		if tag != "" {
			args = append(args, "-tags", tag)
		}
		args = append(args, ".")
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
		profile := ""
		if tag != "" {
			profile = filepath.Join(dir, "profile.json")
		}
		out, err := exec.Command(binary, profile).CombinedOutput()
		if err != nil {
			t.Fatalf("run: %v\n%s", err, out)
		}
		return out
	}
	off, on := buildRun(""), buildRun("profile_sites")
	if string(off) != string(on) {
		t.Fatalf("profile changed output\noff:%s\non:%s", off, on)
	}
	var report struct {
		Schema, InputSHA, BinarySHA string
		Sites                       map[string]struct {
			Calls, Allocations, Invocations, Trips uint64
			Receivers, Histogram                   map[string]uint64
		}
	}
	// Use explicit wire tags for the certificate fields.
	var wire struct {
		Schema    string          `json:"schema"`
		InputSHA  string          `json:"input_sha256"`
		BinarySHA string          `json:"binary_sha256"`
		Sites     json.RawMessage `json:"sites"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	report.Schema, report.InputSHA, report.BinarySHA = wire.Schema, wire.InputSHA, wire.BinarySHA
	if err := json.Unmarshal(wire.Sites, &report.Sites); err != nil {
		t.Fatal(err)
	}
	c := report.Sites[call.SiteID]
	if c.Calls != 70 || c.Receivers[a.Name] != 35 || c.Receivers[b.Name] != 35 {
		t.Fatalf("receiver counts %+v", c)
	}
	l := report.Sites[loop.SiteID]
	if l.Invocations != 16 || l.Trips != 70 || !reflect.DeepEqual(l.Histogram, map[string]uint64{"0": 2, "1": 2, "2-3": 4, "4-7": 4, "8+": 4}) {
		t.Fatalf("loop counts %+v", l)
	}
	el := report.Sites[earlyLoop.SiteID]
	if el.Invocations != 8 || el.Trips != 7 || el.Histogram["0"] != 1 || el.Histogram["1"] != 7 {
		t.Fatalf("return lost loop count %+v", el)
	}
	wl := report.Sites[whileLoop.SiteID]
	if wl.Invocations != 8 || wl.Trips != 18 || wl.Histogram["0"] != 1 || wl.Histogram["1"] != 1 || wl.Histogram["2-3"] != 6 {
		t.Fatalf("while break/continue counts %+v", wl)
	}
	if report.Sites[alloc.SiteID].Allocations != 1 {
		t.Fatal("allocation count")
	}
	binary, err := os.ReadFile(filepath.Join(dir, "runprofile_sites"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(binary)
	if report.Schema != "site-profile/1" || report.BinarySHA != hex.EncodeToString(sum[:]) || len(report.InputSHA) != 64 {
		t.Fatal("invalid certificate")
	}
}

func TestSiteMapDefaultBodies(t *testing.T) {
	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) {
			plain, err := Emit(f.p)
			if err != nil {
				t.Fatal(err)
			}
			mapped, sites, err := EmitWithSites(f.p)
			if err != nil {
				t.Fatal(err)
			}
			if mapped["hir.go"] != "//go:build !profile_sites\n\n"+plain["hir.go"] || mapped["runtime.go"] != plain["runtime.go"] {
				t.Fatal("ordinary source changed")
			}
			for _, site := range sites.Sites {
				if site.SiteID == "" || site.Method == "" {
					t.Fatalf("missing identity %+v", site)
				}
				if _, ok := site.Locations["go_profile"]; !ok {
					t.Fatalf("missing profile location %+v", site)
				}
			}
		})
	}
}

func TestCopyPropSiteIDs(t *testing.T) {
	build := func() *hir.Program {
		num := hir.T(hir.I32)
		length := func() *hir.Expr {
			return &hir.Expr{Node: hir.Node{Source: "probe.ts:1:1"}, Kind: hir.RuntimeOp, Op: "string.length", X: hir.L(hir.T(hir.String), "abc"), Type: num}
		}
		return &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: num, Body: hir.B(
			&hir.Stmt{Kind: hir.VarDecl, Name: "dead", Type: num, X: length()},
			&hir.Stmt{Kind: hir.Return, X: length()},
		)}}}}}
	}
	t.Setenv("ABAPITI_COPYPROP", "0")
	_, before, err := EmitWithSites(build())
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Sites) != 2 {
		t.Fatalf("baseline sites: %+v", before.Sites)
	}
	t.Setenv("ABAPITI_COPYPROP", "1")
	_, after, err := EmitWithSites(build())
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Sites) != 1 || after.Sites[0].SiteID != before.Sites[1].SiteID {
		t.Fatalf("removed store must drop its site and preserve the survivor: before=%+v after=%+v", before.Sites, after.Sites)
	}
}
