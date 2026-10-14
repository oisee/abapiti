package tsfront

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Exercise actual refusals, registration metadata, HIR spans and names.json,
// not a post-emission path replacement. Each target starts from fresh HIR.
func TestSourceLocationsIndependentOfLoweringRoot(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	t.Setenv("ABAPITI_SINGLETON", "0")
	roots := []string{filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "different", "root")}
	sources := map[string]string{
		"test/probe.ts": `import {expect} from "../runtime/chai";
export class Probe {static refused():string {return String({value: 1});}}
describe("locations",()=>{it("refusal",()=>Probe.refused());it("unsupported",()=>new Date());it("pass",()=>expect(1).equal(1));});`,
	}
	for _, name := range []string{"mocha", "chai"} {
		raw, err := os.ReadFile("testdata/abaplint-tests/runtime/" + name + ".ts")
		if err != nil {
			t.Fatal(err)
		}
		sources["runtime/"+name+".ts"] = string(raw)
	}
	order := []string{"runtime/mocha.ts", "runtime/chai.ts", "test/probe.ts"}
	for _, root := range roots {
		var rows []map[string]string
		for _, name := range order {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(sources[name]), 0600); err != nil {
				t.Fatal(err)
			}
			rows = append(rows, map[string]string{"file": name, "sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(sources[name])))})
		}
		manifest, err := json.Marshal(map[string]any{"upstreamPin": RegistryUpstreamPin, "roots": []string{"test/probe.ts"}, "initialization": order, "sources": rows})
		if err != nil {
			t.Fatal(err)
		}
		for name, raw := range map[string][]byte{"closure.json": manifest, "tsconfig.json": []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["runtime/*.ts","test/*.ts"]}`)} {
			if err := os.WriteFile(filepath.Join(root, name), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, target := range []string{"go", "abap", "osg", "a4h"} {
		t.Run(target, func(t *testing.T) {
			t.Setenv("ABAPITI_NAMES", "")
			if target == "a4h" {
				// A4H uses this shared source emitter with readable identities;
				// zip packaging/metadata is additionally checked by the release gate.
				t.Setenv("ABAPITI_NAMES", "readable")
			}
			var first map[string]string
			for _, root := range roots {
				l, err := LowerAbaplintTests(root)
				if err != nil {
					t.Fatal(err)
				}
				var files map[string]string
				switch target {
				case "go":
					files, err = EmitAbaplintTests(root, l)
				case "abap":
					files, err = emitABAP(t, l.Prog)
				case "osg", "a4h":
					files, err = emitABAPTestsOSG(t, root, l)
				}
				if err != nil {
					t.Fatal(err)
				}
				if target == "a4h" {
					support, err := A4HTestSupport()
					if err != nil {
						t.Fatal(err)
					}
					for name, source := range support {
						files[name] = source
					}
				}
				metadata, err := json.Marshal(l.Diagnostics)
				if err != nil {
					t.Fatal(err)
				}
				files["lower-diagnostics.json"] = string(metadata)
				files["hir.txt"] = hir.Dump(l.Prog)
				p, err := Load(filepath.Join(root, "tsconfig.json"))
				if err != nil {
					t.Fatal(err)
				}
				dump, err := p.Dump([]string{"test/probe.ts"})
				if err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(dump)
				if err != nil {
					t.Fatal(err)
				}
				files["tsfront.json"] = string(raw)
				raw, err = json.Marshal(p.CheckerDiagnostics(t.Context()))
				if err != nil {
					t.Fatal(err)
				}
				files["checker.json"] = string(raw)
				located := false
				for name, source := range files {
					if strings.Contains(source, root) {
						t.Fatalf("%s contains lowering root", name)
					}
					located = located || strings.Contains(source, "test/probe.ts:")
				}
				if !located {
					t.Fatal("fixture lost source locations")
				}
				if first == nil {
					first = files
				} else if !reflect.DeepEqual(first, files) {
					for name, source := range first {
						if files[name] != source {
							t.Errorf("path-dependent output: %s", name)
						}
					}
					if len(first) != len(files) {
						t.Error("path-dependent file set")
					}
				}
			}
		})
	}
}

func TestRelativeSource(t *testing.T) {
	for _, tc := range []struct{ root, file, want string }{
		{"/build/project", "/build/project/src/a.ts", "src/a.ts"},
		{"/build/project", "/deps/node_modules/pkg/index.d.ts", "node_modules/pkg/index.d.ts"},
		{"/build/project", "/build/shared/a.ts", "../shared/a.ts"},
		{"", "/build/project/a.ts", "a.ts"},
	} {
		if got := relativeSource(tc.root, tc.file); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
