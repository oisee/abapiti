package tsfront

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// copyRequireFixture copies testdata/requirebinding to a temporary directory;
// the prebuilt CommonJS packages under cjs/ become node_modules/ there (the
// repository ignores node_modules directories).
func copyRequireFixture(t *testing.T, probe string) string {
	t.Helper()
	src := filepath.Join("testdata", "requirebinding")
	dir := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if strings.HasPrefix(rel, "cjs"+string(filepath.Separator)) {
			rel = filepath.Join("node_modules", strings.TrimPrefix(rel, "cjs"+string(filepath.Separator)))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if probe != "" && rel == "probe.mjs" {
			data = []byte(probe)
		}
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// lowerRequireFixture lowers every non-declaration file of the fixture.
func lowerRequireFixture(t *testing.T, dir string) (*hir.Program, []LowerDiagnostic) {
	t.Helper()
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, name := range p.SourceFiles() {
		if !strings.HasSuffix(name, ".d.ts") && strings.HasPrefix(name, dir) {
			files = append(files, name)
		}
	}
	prog, diags, err := p.Lower(files)
	if err != nil {
		t.Fatal(err)
	}
	return prog, diags
}

// gogen loads @abaplint/core and the Rearranger through createRequire with a
// JSDoc `typeof import(...)` of the same literal: lowered as static imports
// (namespace member instanceof, new of a destructured class, static method,
// instance call). Go always runs; Node compares when present.
func TestRequireBindingNodeOracle(t *testing.T) {
	dir := copyRequireFixture(t, "")
	prog, diags := lowerRequireFixture(t, dir)
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Errorf("lower: %s", d)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs[0])
	}
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.mjs.Probe", "run")))
	const want = "char:3|int|C3|'x'|I>C3\n"
	if got != want {
		t.Fatalf("Go %q, want %q", got, want)
	}
	if _, err := emitABAP(t, prog); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Log("Node absent: Go half only")
		return
	}
	cmd := exec.Command("node", "--input-type=module", "-e", `import {Probe} from "./probe.mjs";console.log(Probe.run())`)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if string(out) != got {
		t.Fatalf("Go %q Node %q", got, out)
	}
}

// A require that cannot be proven static, or another use of createRequire,
// stays a blocking diagnostic at its location: never a silent guess.
func TestRequireBindingRefusals(t *testing.T) {
	base, err := os.ReadFile(filepath.Join("testdata", "requirebinding", "probe.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	const coreDecl = `const core = transpilerRequire("@fake/core");`
	const rearrangerDecl = `/** @type {typeof import("@fake/transpiler/build/src/rearranger")} */
const {Rearranger} = require("@fake/transpiler/build/src/rearranger");`
	cases := []struct {
		name, old, new, loc, category string
	}{
		{"mismatched literal", coreDecl, `const core = transpilerRequire("@fake/transpiler/build/src/rearranger");`, "probe.mjs:5:14", "unsupported-require"},
		{"missing annotation", rearrangerDecl, "\n" + `const {Rearranger} = require("@fake/transpiler/build/src/rearranger");`, "probe.mjs:7:22", "unsupported-require"},
		{"non-literal argument", coreDecl, `const core = transpilerRequire("@fake/" + "core");`, "probe.mjs:5:14", "unsupported-require"},
		{"other createRequire use", "export class Probe {", `const other = createRequire("/elsewhere.js");
export class Probe {`, "probe.mjs:10:15", "unsupported-call"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(string(base), c.old) {
				t.Fatalf("fixture lacks %q", c.old)
			}
			dir := copyRequireFixture(t, strings.Replace(string(base), c.old, c.new, 1))
			_, diags := lowerRequireFixture(t, dir)
			var blocking []string
			found := false
			for _, d := range diags {
				if strings.HasPrefix(d.Category, "note-") {
					continue
				}
				blocking = append(blocking, d.String())
				found = found || (d.Category == c.category && d.Loc == c.loc)
			}
			if !found {
				t.Fatalf("no %s at %s; blocking: %s", c.category, c.loc, strings.Join(blocking, "; "))
			}
		})
	}
}
