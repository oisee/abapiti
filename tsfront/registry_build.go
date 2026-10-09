package tsfront

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// RegistryUpstreamPin is the abaplint commit (packages/core 2.120.56) whose
// sources the closure manifest, the reachability manifest and the
// fingerprinted overrides were recorded against.
const RegistryUpstreamPin = "577f875ebec44cfaf64841cfe71c8ab8dc32622e"

// RegistryHarnessPath is where the deployment harness sits next to src/.
const RegistryHarnessPath = "harness/registry_run.ts"

//go:embed testdata/registrycorpus/harness/registry_run.ts
var registryHarness []byte

//go:embed testdata/registrycorpus/reachability.json
var registryReachability []byte

//go:embed testdata/registrycorpus/closure.json
var registryClosure []byte

//go:embed testdata/registrycorpus/node-packages.json
var registryNodePackages []byte

//go:embed all:testdata/registrycorpus/node-packages
var registryNodePackageFiles embed.FS

// RegistryHarness returns the embedded deployment harness source.
func RegistryHarness() []byte { return append([]byte(nil), registryHarness...) }

// RegistryClosure is the static import closure of registry.ts, config.ts and
// files/memory_file.ts at the pin (tools/registry-closure.mjs): every file
// with its SHA-256, paths relative to packages/core.
type RegistryClosure struct {
	Pin      string   `json:"upstreamPin"`
	Roots    []string `json:"roots"`
	External []string `json:"external"`
	Sources  []struct {
		File   string `json:"file"`
		SHA256 string `json:"sha256"`
	} `json:"sources"`
}

// ParseRegistryClosure reads a closure manifest and checks it names the pin.
func ParseRegistryClosure(raw []byte) (*RegistryClosure, error) {
	var c RegistryClosure
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if c.Pin != RegistryUpstreamPin || len(c.Sources) == 0 {
		return nil, fmt.Errorf("Registry closure must name the original upstream pin and contain sources")
	}
	return &c, nil
}

// EmbeddedRegistryClosure is the closure manifest built into the binary.
func EmbeddedRegistryClosure() *RegistryClosure {
	c, err := ParseRegistryClosure(registryClosure)
	if err != nil {
		panic(err)
	}
	return c
}

// Files lists the closure's source paths in manifest order.
func (c *RegistryClosure) Files() []string {
	files := make([]string, len(c.Sources))
	for i, s := range c.Sources {
		files[i] = s.File
	}
	return files
}

// Verify checks every closure file under root (a directory holding src/)
// against its pinned SHA-256 and reports the first difference.
func (c *RegistryClosure) Verify(root string) error {
	for _, s := range c.Sources {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(s.File)))
		if err != nil {
			return fmt.Errorf("pinned source %s: %w", s.File, err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != s.SHA256 {
			return fmt.Errorf("changed pinned source: %s (sha256 %s, pinned %s)", s.File, got, s.SHA256)
		}
	}
	return nil
}

// RegistryNodePackage is one npm package the front end resolves from
// packages/core: lockfile coordinates and the files of its tarball the
// translation reads (type declarations and package.json) plus its license.
type RegistryNodePackage struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Resolved  string `json:"resolved"`
	Integrity string `json:"integrity"`
	Files     []struct {
		File   string `json:"file"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

// RegistryNodePackages are the packages the translation needs: the type
// declarations of fast-xml-parser, json5 and vscode-languageserver-types
// (the program loads no other file from node_modules; @types are not used).
func RegistryNodePackages() []RegistryNodePackage {
	var pkgs []RegistryNodePackage
	if err := json.Unmarshal(registryNodePackages, &pkgs); err != nil {
		panic(err)
	}
	return pkgs
}

// Verify checks the package directory dir (node_modules/<name>) file by file.
func (p RegistryNodePackage) Verify(dir string) error {
	for _, f := range p.Files {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.File)))
		if err != nil {
			return fmt.Errorf("npm package %s@%s: %w", p.Name, p.Version, err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.SHA256 {
			return fmt.Errorf("npm package %s@%s: changed file %s (sha256 %s, pinned %s)", p.Name, p.Version, f.File, got, f.SHA256)
		}
	}
	return nil
}

// WriteEmbedded writes the package's pinned files (type declarations,
// package.json, license) from the abapiti binary into dir and verifies them,
// so a fetch needs no npm registry.
func (p RegistryNodePackage) WriteEmbedded(dir string) error {
	for _, f := range p.Files {
		raw, err := registryNodePackageFiles.ReadFile("testdata/registrycorpus/node-packages/" + p.Name + "/" + f.File)
		if err != nil {
			return fmt.Errorf("npm package %s@%s: %w", p.Name, p.Version, err)
		}
		target := filepath.Join(dir, filepath.FromSlash(f.File))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, raw, 0644); err != nil {
			return err
		}
	}
	return p.Verify(dir)
}

// RegistryOverrides is the override registry of the Registry build: the
// abaplint inventory without the parser-only scope exclusions, plus the
// deployment and syntax overrides.
func RegistryOverrides() (*overrides.Registry, error) {
	var entries []overrides.Entry
	for _, e := range overrides.Abaplint().Inventory() {
		switch e.ID {
		case "abaplint-registry-scope", "abaplint-external-include", "abaplint-registry-input-progress":
			continue
		}
		entries = append(entries, e)
	}
	entries = append(entries, overrides.RegistryDeployment()...)
	entries = append(entries, overrides.Syntax()...)
	return overrides.New(entries...)
}

// ParseReachability reads a reachability manifest for the given workload
// inputs (nil: the manifest is trusted as recorded, see InputsRecorded).
func ParseReachability(data []byte, inputs *CoverageInputs) (*Reachability, error) {
	r := &Reachability{CurrentInputs: inputs}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, err
	}
	if inputs == nil {
		r.InputsRecorded = true
	}
	return r, nil
}

// EmbeddedReachability is the reachability manifest built into the binary.
// Its workload inputs were validated when it was recorded; the generator
// does not need them.
func EmbeddedReachability() (*Reachability, error) {
	return ParseReachability(registryReachability, nil)
}

// RegistryLowering is the lowered Registry closure with its evidence.
type RegistryLowering struct {
	Files        []string
	Overrides    *overrides.Registry
	Prog         *hir.Program
	Diagnostics  []LowerDiagnostic
	Blocking     []LowerDiagnostic
	Notes        []string // note-declaration-reachability messages
	Verification []error
	// TrappedBases names constructed classes that extend a type-only class
	// (trapping constructor): super() would run the trap.
	TrappedBases []string
}

// LowerRegistry loads dir/tsconfig.json and lowers files (relative to dir)
// with the given overrides and optional reachability manifest.
func LowerRegistry(dir string, files []string, registry *overrides.Registry, coverage *Reachability) (*RegistryLowering, error) {
	if coverage != nil && coverage.UpstreamPin != RegistryUpstreamPin {
		return nil, fmt.Errorf("coverage upstream pin mismatch")
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		return nil, err
	}
	prog, diags, err := p.LowerWithReachability(files, registry, coverage)
	if err != nil {
		return nil, err
	}
	r := &RegistryLowering{Files: files, Overrides: registry, Prog: prog, Diagnostics: diags}
	for _, d := range diags {
		if d.Category == "note-declaration-reachability" {
			r.Notes = append(r.Notes, d.Message)
		}
		if !strings.HasPrefix(d.Category, "note-") {
			r.Blocking = append(r.Blocking, d)
		}
	}
	r.Verification = hir.Verify(prog)
	trapped := map[string]bool{}
	for _, c := range prog.Classes {
		if c.Ctor != nil && c.Ctor.Body != nil && len(c.Ctor.Body.List) == 1 && c.Ctor.Body.List[0].Kind == hir.Trap {
			trapped[c.Name] = true
		}
	}
	for _, c := range prog.Classes {
		if c.Super != "" && trapped[c.Super] && !trapped[c.Name] {
			r.TrappedBases = append(r.TrappedBases, fmt.Sprintf("constructed class %s extends type-only %s", c.Name, c.Super))
		}
	}
	return r, nil
}

// TrapCount counts the method bodies the reachability manifest replaced
// with traps (code the recorded workloads never execute).
func (r *RegistryLowering) TrapCount() int {
	n := 0
	for _, d := range r.Diagnostics {
		if d.Category == "note-reachability" {
			n++
		}
	}
	return n
}

// Evidence returns the JSON/text evidence files of the lowering
// (registry-overrides.json, -blocking.json, -traps.json, -verify.txt and,
// with dump, registry-hir.txt).
func (r *RegistryLowering) Evidence(dump bool) (map[string][]byte, error) {
	out := map[string][]byte{}
	var inventory []struct {
		ID, File, Symbol, Kind, SHA256, Rationale string
	}
	for _, e := range r.Overrides.Inventory() {
		inventory = append(inventory, struct{ ID, File, Symbol, Kind, SHA256, Rationale string }{e.ID, e.Key.File, e.Key.Symbol, e.Key.Kind, e.SHA256, e.Rationale})
	}
	data, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return nil, err
	}
	out["registry-overrides.json"] = append(data, '\n')
	data, err = json.MarshalIndent(r.Blocking, "", "  ")
	if err != nil {
		return nil, err
	}
	out["registry-blocking.json"] = append(data, '\n')
	var trapEvidence []struct{ Class, Method, Location string }
	for _, c := range r.Prog.Classes {
		for _, m := range c.Methods {
			if m.Body != nil && m.Body.Kind == hir.Block && len(m.Body.List) == 1 && m.Body.List[0].Kind == hir.Trap {
				trapEvidence = append(trapEvidence, struct{ Class, Method, Location string }{c.Name, m.Name, m.Body.List[0].Name})
			}
		}
	}
	data, err = json.MarshalIndent(trapEvidence, "", "  ")
	if err != nil {
		return nil, err
	}
	out["registry-traps.json"] = append(data, '\n')
	var lines []string
	for _, v := range r.Verification {
		lines = append(lines, v.Error())
	}
	out["registry-verify.txt"] = []byte(strings.Join(lines, "\n"))
	if dump {
		out["registry-hir.txt"] = []byte(hir.Dump(r.Prog))
	}
	return out, nil
}

// Err refuses a lowering that is not translatable as a whole.
func (r *RegistryLowering) Err() error {
	if len(r.Blocking) != 0 || len(r.Verification) != 0 {
		first := ""
		if len(r.Blocking) > 0 {
			first = ": " + r.Blocking[0].String()
		} else {
			first = ": " + r.Verification[0].Error()
		}
		return fmt.Errorf("Registry closure is not translatable (%d blocking diagnostics, %d HIR errors)%s", len(r.Blocking), len(r.Verification), first)
	}
	if len(r.TrappedBases) != 0 {
		return fmt.Errorf("%s", r.TrappedBases[0])
	}
	return nil
}

// Emit runs the ABAP backend over the lowered program.
func (r *RegistryLowering) Emit() (map[string]string, *hir.Names, error) {
	emitted, names, err := abap.EmitNamed(r.Prog)
	if err != nil {
		return nil, nil, err
	}
	if len(emitted) == 0 {
		return nil, nil, fmt.Errorf("Registry closure emitted no files")
	}
	return emitted, names, nil
}

// LiveSpans counts the recorded function spans and those the deployment,
// negative or observation workloads executed (schema 2; schema 1: Executed).
func (r *Reachability) LiveSpans() (live, total int) {
	for _, span := range r.Spans {
		ok := span.Executed
		if r.Schema == 2 {
			ok = false
			for _, w := range span.Workloads {
				ok = ok || w == "DEPLOYMENT" || w == "NEGATIVE" || w == "OBSERVATION"
			}
		}
		if ok {
			live++
		}
	}
	return live, len(r.Spans)
}

// ReadRegistryInputs reads the run driver's inputs: files under inputDir,
// then dependencies under depsDir, each set in path order.
func ReadRegistryInputs(inputDir, depsDir string) ([]RegistryFile, error) {
	var inputs []RegistryFile
	for _, root := range []struct {
		dir string
		dep bool
	}{{inputDir, false}, {depsDir, true}} {
		var names []string
		if err := filepath.WalkDir(root.dir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				names = append(names, path)
			}
			return err
		}); err != nil {
			return nil, err
		}
		sort.Strings(names)
		for _, path := range names {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			rel, _ := filepath.Rel(root.dir, path)
			inputs = append(inputs, RegistryFile{Name: filepath.ToSlash(rel), Raw: string(data), Dependency: root.dep})
		}
	}
	return inputs, nil
}
