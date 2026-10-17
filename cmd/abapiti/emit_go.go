package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/hir"
	gohir "github.com/oisee/abapiti/hir/golang"
	"github.com/oisee/abapiti/tsfront"
	"github.com/oisee/abapiti/tsfront/overrides"
	"github.com/spf13/cobra"
)

func newEmitGoCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "emit-go --tsconfig FILE [flags] file...", Short: "Lower a TypeScript project and emit a standalone Go module", Args: cobra.MinimumNArgs(1), RunE: runEmitGo}
	f := cmd.Flags()
	f.String("tsconfig", "tsconfig.json", "Project tsconfig; files are relative to its directory")
	f.String("externs", "", "Extern module JSON configuration")
	f.StringP("output", "o", "out/go", "Generated module directory")
	f.String("initialize", "", "JSON array of source files to initialize in dependency order before entry")
	f.String("entry", "", "Optional static no-argument entry method (file.ts.Class.method)")
	f.Bool("trap-on-failure", false, "Emit unsupported bodies as source-located traps and write lowering evidence")
	f.String("overrides", "none", "Source-pinned overrides: none or registry")
	f.String("reachability", "", "Generic workload coverage manifest; unexecuted bodies become traps")
	f.Bool("build", false, "Build the generated module as program")
	return cmd
}
func init() { rootCmd.AddCommand(newEmitGoCmd()) }

func runEmitGo(cmd *cobra.Command, args []string) error {
	f := cmd.Flags()
	config, _ := f.GetString("tsconfig")
	config, err := filepath.Abs(config)
	if err != nil {
		return err
	}
	externFile, _ := f.GetString("externs")
	modules, err := tsfront.ReadExterns(externFile)
	if err != nil {
		return err
	}
	registry, err := overrides.New()
	mode, _ := f.GetString("overrides")
	switch mode {
	case "none":
	case "registry":
		registry, err = tsfront.RegistryOverrides()
	default:
		return fmt.Errorf("--overrides: use none or registry")
	}
	if err != nil {
		return err
	}
	var coverage *tsfront.Reachability
	if path, _ := f.GetString("reachability"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		coverage, err = tsfront.ParseReachability(data, nil)
		if err != nil {
			return err
		}
		if len(coverage.Inputs) > 0 || coverage.UpstreamPin != "" {
			return fmt.Errorf("--reachability takes a generic workload manifest (no upstreamPin, no inputs)")
		}
	}
	trap, _ := f.GetBool("trap-on-failure")
	r, err := tsfront.LowerProjectWithOptions(config, args, registry, coverage, tsfront.LowerOptions{Externs: modules, TrapOnFailure: trap})
	if err != nil {
		return err
	}
	out, _ := f.GetString("output")
	if trap {
		if err := writeEvidence(filepath.Join(out, "evidence"), r); err != nil {
			return err
		}
	}
	if len(r.Blocking) > 0 {
		return fmt.Errorf("lowering: %s", r.Blocking[0])
	}
	if len(r.Verification) > 0 {
		return r.Verification[0]
	}
	var files map[string]string
	if trap {
		files, err = tsfront.EmitGoWithTraps(r)
		if evidenceErr := writeEvidence(filepath.Join(out, "evidence"), r); evidenceErr != nil {
			return evidenceErr
		}
	} else {
		files, err = gohir.Emit(r.Prog)
	}
	if err != nil {
		return err
	}
	entry, _ := f.GetString("entry")
	call := ""
	if entry != "" {
		found := false
		for _, c := range r.Prog.Classes {
			for _, m := range c.Methods {
				if c.Name+"."+m.Name == entry && m.Static && len(m.Params) == 0 {
					found = true
					call = hir.NewNames().Get("body."+entry) + "()"
				}
			}
		}
		if !found {
			return fmt.Errorf("entry %q must name a static no-argument method", entry)
		}
	}
	if path, _ := f.GetString("initialize"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var order []string
		if err := json.Unmarshal(data, &order); err != nil {
			return err
		}
		selected := map[string]bool{}
		for _, file := range args {
			selected[filepath.ToSlash(filepath.Clean(file))] = true
		}
		modules := map[string]bool{}
		for _, c := range r.Prog.Classes {
			modules[c.Name] = true
		}
		var init strings.Builder
		seen := map[string]bool{}
		for _, file := range order {
			file = filepath.ToSlash(filepath.Clean(file))
			if !selected[file] {
				return fmt.Errorf("initialization file %q outside selected closure", file)
			}
			if modules[file+".module"] && !seen[file] {
				fmt.Fprintf(&init, "%s();", hir.NewNames().Get("init."+file+".module"))
			}
			seen[file] = true
		}
		call = init.String() + call
	}
	files["main.go"] = "package main\nfunc main(){" + call + "}\n"
	files["go.mod"] = tsfront.ExternGoMod("generated", modules, filepath.Dir(config))
	if err = os.MkdirAll(out, 0755); err != nil {
		return err
	}
	for n, s := range files {
		if err = os.WriteFile(filepath.Join(out, n), []byte(s), 0644); err != nil {
			return err
		}
	}
	build, _ := f.GetBool("build")
	if build {
		process := exec.CommandContext(cmd.Context(), "nice", "go", "build", "-o", "program", ".")
		process.Dir = out
		process.Env = append(os.Environ(), "GOWORK=off")
		if b, err := process.CombinedOutput(); err != nil {
			return fmt.Errorf("Go build: %w\n%s", err, strings.TrimSpace(string(b)))
		}
	}
	return nil
}
