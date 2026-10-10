package tsfront

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestSiteIDsStableAcrossLoweringAndUnrelatedEdit(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("tsconfig.json", `{"compilerOptions":{"strict":true},"files":["other.ts","probe.ts"]}`)
	write("probe.ts", `export class Probe { run(xs: number[]): number { let n = 0; for (const x of xs) { n += x; } return n; } call(): number { return this.run([1,2]); } make(): () => void { return () => { this.run([1]); }; } }`)
	write("other.ts", `export class Other { run(): number { return 1; } }`)
	lower := func() map[string]string {
		t.Helper()
		p, err := Load(filepath.Join(dir, "tsconfig.json"))
		if err != nil {
			t.Fatal(err)
		}
		h, ds, err := p.Lower([]string{"other.ts", "probe.ts"})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range ds {
			if d.Category == "unsupported" {
				t.Fatal(d)
			}
		}
		sites := map[string]string{}
		var expr func(*hir.Expr)
		var stmt func(*hir.Stmt)
		expr = func(x *hir.Expr) {
			if x == nil {
				return
			}
			sites[x.SiteID] = x.SiteSource
			expr(x.X)
			expr(x.Y)
			expr(x.Z)
			for _, a := range x.Args {
				expr(a)
			}
			stmt(x.Stmt)
		}
		stmt = func(s *hir.Stmt) {
			if s == nil {
				return
			}
			sites[s.SiteID] = s.SiteSource
			expr(s.X)
			expr(s.Y)
			stmt(s.Body)
			stmt(s.Else)
			for _, c := range s.List {
				stmt(c)
			}
		}
		foundClosure := false
		for _, c := range h.Classes {
			if strings.HasPrefix(c.SiteOwner, "probe.ts.Probe.make.[closure@") {
				foundClosure = true
			}
			if c.Name == "probe.ts.Probe" || strings.HasPrefix(c.SiteOwner, "probe.ts.Probe.") {
				if c.Ctor != nil {
					stmt(c.Ctor.Body)
				}
				for _, m := range c.Methods {
					stmt(m.Body)
				}
			}
		}
		if !foundClosure {
			t.Fatal("no closure sites")
		}
		if len(sites) == 0 {
			t.Fatal("no probe sites")
		}
		return sites
	}
	a, b := lower(), lower()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two lowerings differ")
	}
	write("other.ts", `export class Other { run(): number { const extra = [1,2,3]; let n = 0; for (const x of extra) { n += x; } return n; } another(): number { return this.run(); } }`)
	if c := lower(); !reflect.DeepEqual(a, c) {
		t.Fatal("unrelated edit changed probe sites")
	}
	previous := dir
	dir = t.TempDir()
	for _, name := range []string{"tsconfig.json", "probe.ts", "other.ts"} {
		raw, err := os.ReadFile(filepath.Join(previous, name))
		if err != nil {
			t.Fatal(err)
		}
		write(name, string(raw))
	}
	if relocated := lower(); !reflect.DeepEqual(a, relocated) {
		t.Fatal("temporary root changed IDs")
	}
}
