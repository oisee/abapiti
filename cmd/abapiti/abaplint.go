package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oisee/abapiti/hir/abap"
	gohir "github.com/oisee/abapiti/hir/golang"
	"github.com/oisee/abapiti/tsfront"
	"github.com/spf13/cobra"
)

// zabapgitRunSHA is the Node oracle's SHA-256 of the issues abaplint prints
// for zabapgit_standalone with abapGit's ci/abaplint.json (the run drivers
// compare against it).
const zabapgitRunSHA = "5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9"

var abaplintCmd = &cobra.Command{
	Use:   "abaplint [path-to-abaplint-checkout] -o <outdir>",
	Short: "Translate abaplint (TypeScript) into ABAP classes or Go",
	Long: `Translate the core of abaplint (github.com/abaplint/abaplint, commit
577f875e, @abaplint/core 2.120.56) into ABAP, without Node.

With a checkout path, the checkout must be at that commit with "npm ci" done
in packages/core. Without one, abapiti downloads the source archive of the
commit and the three npm packages the translation needs (verified against
the lockfile's sha512 integrity) into the user cache directory.

The build is pruned to the code paths that checking zabapgit_standalone with
abapGit's ci/abaplint.json (v702, six rules) executes; other inputs or rules
may be refused with the TypeScript location, never answered silently wrong.

Outputs under <outdir>:
  classes/  the translated classes and interfaces
  a4h/      abapGit zip: classes + ZCL_ABAPITI_REGISTRY_A4H + ZABAPITI_REGISTRY_RUN
  osg/      classes for open-steamgate unit runners (+ ZCL_ABAPITI_REGISTRY_RUN
            with embedded inputs when --input, --deps and --config are given)
  native/   zabaplint.prog.abap + lib/ for open-steamgate's osabap native build
  sites.json with --target go: sites/1 stable HIR identities and Go locations
  go/       with --target go: TS-HG@Go module + zabaplint-go executable when Go is on PATH`,
	Example: `  abapiti abaplint -o out
  abapiti abaplint ~/src/abaplint -o out --target native
  abapiti abaplint -o out --target osg --input zabapgit/in --deps zabapgit/deps/src --config ci-abaplint.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAbaplint,
}

func init() {
	f := abaplintCmd.Flags()
	f.StringP("output", "o", "", "Output directory (required)")
	f.String("target", "all", "Targets to write: all (ABAP targets), a4h, osg, native or go (comma-separated)")
	f.String("package", "$ZABAPLINT", "ABAP package named in the A4H abapGit zip")
	f.String("input", "", "osg: folder of files to check (embedded into ZCL_ABAPITI_REGISTRY_RUN)")
	f.String("deps", "", "osg: folder of dependency files")
	f.String("config", "", "osg: abaplint.json for the embedded run")
	f.String("run-sha", zabapgitRunSHA, "SHA-256 of the expected issue dump that the run drivers compare against (default: zabapgit_standalone with abapGit's ci/abaplint.json)")
	f.String("negative", "", "a4h: seeded negative variant (JSON: sha, extra, append); adds ZABAPITI_REGISTRY_NEG")
	f.Bool("assume-int", true, "Translate TypeScript number as int8 (assume-only-integer-calculations, ABAPITI_ASSUME_INT); false keeps binary64 f")
	f.BoolP("quiet", "q", false, "Do not narrate the steps")
	f.Bool("evidence", false, "Also write the lowering evidence (overrides, traps, blocking diagnostics) to <outdir>/evidence")
	_ = abaplintCmd.MarkFlagRequired("output")
	rootCmd.AddCommand(abaplintCmd)
}

type narrator struct {
	w     io.Writer
	quiet bool
	start time.Time
}

func (n *narrator) say(format string, args ...any) {
	if !n.quiet {
		fmt.Fprintf(n.w, format+"\n", args...)
	}
}

// step prints one line with the time since the previous step.
func (n *narrator) step(format string, args ...any) {
	n.say("%-62s %5.1fs", fmt.Sprintf(format, args...), time.Since(n.start).Seconds())
	n.start = time.Now()
}

func parseTargets(s string) (map[string]bool, error) {
	targets := map[string]bool{}
	for _, t := range strings.Split(s, ",") {
		switch t = strings.TrimSpace(strings.ToLower(t)); t {
		case "all":
			targets["a4h"], targets["osg"], targets["native"] = true, true, true
		case "a4h", "osg", "native", "go":
			targets[t] = true
		default:
			return nil, fmt.Errorf("--target %q: use all, a4h, osg, native or go", t)
		}
	}
	return targets, nil
}

func runAbaplint(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()
	out, _ := flags.GetString("output")
	targetFlag, _ := flags.GetString("target")
	pkg, _ := flags.GetString("package")
	input, _ := flags.GetString("input")
	deps, _ := flags.GetString("deps")
	config, _ := flags.GetString("config")
	runSHA, _ := flags.GetString("run-sha")
	negativePath, _ := flags.GetString("negative")
	quiet, _ := flags.GetBool("quiet")
	assumeInt, _ := flags.GetBool("assume-int")
	if assumeInt {
		os.Setenv("ABAPITI_ASSUME_INT", "1")
	} else {
		os.Setenv("ABAPITI_ASSUME_INT", "")
	}
	evidence, _ := flags.GetBool("evidence")
	targets, err := parseTargets(targetFlag)
	if err != nil {
		return err
	}
	embedRun := input != "" || deps != "" || config != ""
	if embedRun && (input == "" || deps == "" || config == "") {
		return fmt.Errorf("--input, --deps and --config go together")
	}
	if embedRun && !targets["osg"] {
		return fmt.Errorf("--input/--deps/--config only apply to --target osg")
	}
	var negative *tsfront.RegistryNegative
	if negativePath != "" {
		data, err := os.ReadFile(negativePath)
		if err != nil {
			return err
		}
		negative = &tsfront.RegistryNegative{}
		if err := json.Unmarshal(data, negative); err != nil {
			return fmt.Errorf("%s: %v", negativePath, err)
		}
	}
	n := &narrator{w: cmd.ErrOrStderr(), quiet: quiet, start: time.Now()}
	began := n.start
	pin := tsfront.RegistryUpstreamPin[:8]
	destination := "ABAP"
	if targets["go"] && len(targets) == 1 {
		destination = "Go"
	}
	n.say("abapiti %s: translating abaplint %s (@abaplint/core 2.120.56) from TypeScript into %s", version, pin, destination)

	// 1. source
	var src *abaplintSource
	if len(args) == 1 {
		src, err = verifyAbaplintCheckout(args[0])
		if err != nil {
			return err
		}
	} else {
		dir, err := os.MkdirTemp("", "abapiti-abaplint-src-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		src, err = embeddedAbaplint(dir)
		if err != nil {
			return err
		}
	}
	closure := tsfront.EmbeddedRegistryClosure()
	var versions []string
	for _, p := range tsfront.RegistryNodePackages() {
		versions = append(versions, p.Name+"@"+p.Version)
	}
	how := "checkout " + src.Root
	if src.Embedded {
		how = fmt.Sprintf("built into abapiti, %d files unpacked (no network)", src.Files)
	}
	n.step("source: %s", how)
	if assumeInt {
		n.say("  numbers: TypeScript number as int8 (assume-int; --assume-int=false for binary64 f)")
	}
	n.say("  verified commit %s: %d closure files by SHA-256; npm %s", pin, len(closure.Sources), strings.Join(versions, ", "))

	// 2. closure
	work, err := os.MkdirTemp("", "abapiti-abaplint-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err := materializeClosure(src, closure, work); err != nil {
		return err
	}
	files := append(closure.Files(), tsfront.RegistryHarnessPath)
	n.step("closure: %d TypeScript files reachable from registry.ts, config.ts, memory_file.ts + run harness", len(closure.Sources))

	// 3. front end
	registry, err := tsfront.RegistryOverrides()
	if err != nil {
		return err
	}
	coverage, err := tsfront.EmbeddedReachability()
	if err != nil {
		return err
	}
	lowering, err := tsfront.LowerRegistry(work, files, registry, coverage)
	if err != nil {
		return fmt.Errorf("front end: %v", err)
	}
	if evidence {
		if err := writeEvidence(filepath.Join(out, "evidence"), lowering); err != nil {
			return err
		}
	}
	n.step("front end (tsgo checker + lowering): %d classes, %d interfaces, %d blocking diagnostics", len(lowering.Prog.Classes), len(lowering.Prog.Interfaces), len(lowering.Blocking))
	if err := lowering.Err(); err != nil {
		return err
	}
	n.say("  HIR verified: %d errors; %d fingerprinted overrides applied", len(lowering.Verification), len(registry.Inventory()))
	live, recorded := coverage.LiveSpans()
	n.say("  reachability: %d of %d recorded functions run by the zabapgit workloads; %d bodies trapped", live, recorded, lowering.TrapCount())
	for _, note := range lowering.Notes {
		n.say("  %s", note)
	}

	// 4. ABAP
	if targets["go"] {
		files, sites, err := gohir.EmitWithSites(lowering.Prog)
		if err != nil {
			return err
		}
		files["main.go"] = tsfront.RegistryGoCLIWithSites()
		files["go.mod"] = "module zabaplint\n\ngo 1.26.0\n"
		if err := writeSources(filepath.Join(out, "go"), files); err != nil {
			return err
		}
		siteJSON, err := json.MarshalIndent(sites, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, "sites.json"), append(siteJSON, '\n'), 0644); err != nil {
			return err
		}
		n.step("Go emitted: %d classes -> %s", len(lowering.Prog.Classes), filepath.Join(out, "go"))
		n.say("TS-HG@Go generation: %.3fs", time.Since(began).Seconds())
		if goTool, lookupErr := exec.LookPath("go"); lookupErr == nil {
			build := exec.CommandContext(cmd.Context(), goTool, "build", "-o", "zabaplint-go", ".")
			build.Dir = filepath.Join(out, "go")
			build.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
			if output, buildErr := build.CombinedOutput(); buildErr != nil {
				return fmt.Errorf("TS-HG@Go build: %w\n%s", buildErr, output)
			}
			n.step("TS-HG@Go built: %s", filepath.Join(out, "go", "zabaplint-go"))
		} else {
			n.say("Go is absent from PATH; build the module with: cd %s && go build -o zabaplint-go .", filepath.Join(out, "go"))
		}
		if len(targets) == 1 {
			return nil
		}
	}
	emitted, names, err := lowering.Emit()
	if err != nil {
		return err
	}
	classes, intfs := 0, 0
	for name := range emitted {
		if strings.HasSuffix(name, ".intf.abap") {
			intfs++
		} else {
			classes++
		}
	}
	if err := writeSources(filepath.Join(out, "classes"), emitted); err != nil {
		return err
	}
	n.step("ABAP emitted: %d objects (%d classes, %d interfaces) -> %s", len(emitted), classes, intfs, filepath.Join(out, "classes"))
	sourceMap, err := abap.SourceMap(lowering.Prog, names)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "names.json"), sourceMap, 0644); err != nil {
		return err
	}
	n.say("  names.json: every ABAP name -> TypeScript identity and file:line (for traces, dumps and reading)")

	// 5. targets
	var next []string
	if targets["a4h"] {
		dir := filepath.Join(out, "a4h")
		sources := copyMap(emitted)
		a4h := "zcl_abapiti_registry_a4h"
		sources[a4h+".clas.abap"] = tsfront.RegistryRunCorpusClass(a4h, runSHA, negative, names)
		sources["zabapiti_registry_run.prog.abap"] = tsfront.RegistryRunReport("zabapiti_registry_run", a4h, false)
		if negative != nil {
			sources["zabapiti_registry_neg.prog.abap"] = tsfront.RegistryRunReport("zabapiti_registry_neg", a4h, true)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		zipFile := filepath.Join(dir, "abaplint-"+pin+"-a4h.zip")
		objects, err := writeAbapGitZip(zipFile, pkg, "abaplint "+pin+" translated by abapiti", sources)
		if err != nil {
			return err
		}
		line := fmt.Sprintf("a4h/     abapGit zip, %d objects. Import it with abapGit (New Offline, package %s), then run ZABAPITI_REGISTRY_RUN as a background job (SM36 or F9 in SE38); it reads zabapgit from the ZABAPITI_CORPUS table (ZCL_ABAPITI_CORPUS, ZCL_ABAPITI_LOG must be installed) and prints ok=X when the issues match Node's.", objects, pkg)
		if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte(line+"\n"), 0644); err != nil {
			return err
		}
		next = append(next, line)
		n.step("packaged a4h: %s", zipFile)
	}
	if targets["osg"] {
		dir := filepath.Join(out, "osg")
		sources := copyMap(emitted)
		what := "the classes"
		if embedRun {
			inputs, err := tsfront.ReadRegistryInputs(input, deps)
			if err != nil {
				return err
			}
			cfg, err := os.ReadFile(config)
			if err != nil {
				return err
			}
			run := "zcl_abapiti_registry_run"
			sources[run+".clas.abap"] = tsfront.RegistryRunClass(run, inputs, string(cfg), runSHA, 0, names)
			sources[run+".clas.testclasses.abap"] = tsfront.RegistryRunTest(run, "")
			what = fmt.Sprintf("the classes + ZCL_ABAPITI_REGISTRY_RUN with %d embedded files (its unit test fails with the report line; ok=X is the verdict)", len(inputs))
		}
		if err := writeSources(dir, sources); err != nil {
			return err
		}
		line := fmt.Sprintf("osg/     %s. In an open-steamgate checkout: npm run osgo:unit -- %s (or osgjs:unit)", what, absPath(dir))
		if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte(line+"\n"), 0644); err != nil {
			return err
		}
		next = append(next, line)
		n.step("packaged osg: %d files in %s", len(sources), dir)
	}
	if targets["native"] {
		dir := filepath.Join(out, "native")
		if err := writeSources(filepath.Join(dir, "lib"), emitted); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "zabaplint.prog.abap"), []byte(tsfront.RegistryCLIReport("zabaplint", names)), 0644); err != nil {
			return err
		}
		abs := absPath(dir)
		line := fmt.Sprintf("native/  zabaplint.prog.abap + lib/. In an open-steamgate checkout: node tools/gogen/osabap.mjs %s --lib %s (GOOS/GOARCH for cross builds), then: .out/osabap --file zabapgit_standalone.prog.abap --config abaplint.json -allow-read .",
			filepath.Join(abs, "zabaplint.prog.abap"), filepath.Join(abs, "lib"))
		readme := line + "\n\nGeneric form: node tools/gogen/osabap.mjs native/zabaplint.prog.abap --lib native/lib\n"
		if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte(readme), 0644); err != nil {
			return err
		}
		next = append(next, line)
		n.step("packaged native: %s", dir)
	}
	n.say("done in %.1fs. Next:", time.Since(began).Seconds())
	for _, line := range next {
		n.say("  %s", line)
	}
	n.say("Scope: the build covers the code paths of checking zabapgit_standalone with abapGit's ci/abaplint.json (v702, six rules);")
	n.say("outside them a check is refused with the TypeScript location of the missing code.")
	return nil
}

// materializeClosure lays out work/ as tools/registry-closure.mjs does:
// src/ (the closure files), node_modules/ (the needed packages), the
// harness and a tsconfig.json equivalent to the closure's plus harness/.
func materializeClosure(src *abaplintSource, closure *tsfront.RegistryClosure, work string) error {
	core := coreDir(src.Root)
	for _, f := range closure.Files() {
		if err := copyFile(filepath.Join(core, filepath.FromSlash(f)), filepath.Join(work, filepath.FromSlash(f))); err != nil {
			return err
		}
	}
	for _, pkg := range tsfront.RegistryNodePackages() {
		for _, f := range pkg.Files {
			if err := copyFile(filepath.Join(src.Packages[pkg.Name], filepath.FromSlash(f.File)), filepath.Join(work, "node_modules", pkg.Name, filepath.FromSlash(f.File))); err != nil {
				return err
			}
		}
	}
	// copies are verified again: the closure must be exactly what was pinned
	if err := closure.Verify(work); err != nil {
		return err
	}
	harness := filepath.Join(work, filepath.FromSlash(tsfront.RegistryHarnessPath))
	if err := os.MkdirAll(filepath.Dir(harness), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(harness, tsfront.RegistryHarness(), 0644); err != nil {
		return err
	}
	tsconfig := `{
 "compilerOptions": {
  "module": "commonjs",
  "target": "es2020",
  "lib": [
   "es2020"
  ],
  "noEmit": true,
  "skipLibCheck": true,
  "strictNullChecks": true,
  "strictFunctionTypes": true,
  "noImplicitAny": true,
  "strictPropertyInitialization": false
 },
 "include": [
  "src/**/*.ts",
  "harness/**/*.ts"
 ]
}`
	return os.WriteFile(filepath.Join(work, "tsconfig.json"), []byte(tsconfig), 0644)
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		return err
	}
	return os.WriteFile(to, data, 0644)
}

func writeSources(dir string, sources map[string]string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(sources[name]), 0644); err != nil {
			return err
		}
	}
	return nil
}

func writeEvidence(dir string, lowering *tsfront.RegistryLowering) error {
	files, err := lowering.Evidence(false)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}
