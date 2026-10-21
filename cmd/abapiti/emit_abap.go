package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/abapsize"
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront"
	"github.com/oisee/abapiti/tsfront/overrides"
	"github.com/spf13/cobra"
)

// emit-abap is the ABAP counterpart of emit-go for self-hosting measurements:
// the same lowering, then the TS-HA refusal policy (whole owning bodies trap).
func newEmitABAPCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "emit-abap --tsconfig FILE [flags] file...", Short: "Lower a TypeScript project and emit ABAP classes with located refusal traps", Args: cobra.MinimumNArgs(1), RunE: runEmitABAP}
	f := cmd.Flags()
	f.String("tsconfig", "tsconfig.json", "Project tsconfig; files are relative to its directory")
	f.String("externs", "", "Extern module JSON configuration")
	f.StringP("output", "o", "out/abap", "Generated ABAP directory")
	f.String("overrides", "none", "Source-pinned overrides: none or registry")
	f.String("reachability", "", "Generic workload coverage manifest; unexecuted bodies become traps")
	f.String("initialize", "", "JSON array of source files; emits ZCL_ABAPITI_MAIN=>RUN calling their module initializers in this order")
	return cmd
}
func init() { rootCmd.AddCommand(newEmitABAPCmd()) }

func runEmitABAP(cmd *cobra.Command, args []string) error {
	f := cmd.Flags()
	config, _ := f.GetString("tsconfig")
	config, err := filepath.Abs(config)
	if err != nil {
		return err
	}
	var order []string
	if path, _ := f.GetString("initialize"); path != "" {
		if order, err = readInitialization(path, args); err != nil {
			return err
		}
	}
	externFile, _ := f.GetString("externs")
	modules, err := tsfront.ReadExterns(externFile)
	if err != nil {
		return err
	}
	registry, err := overrides.New()
	switch mode, _ := f.GetString("overrides"); mode {
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
		if coverage, err = tsfront.ParseReachability(data, nil); err != nil {
			return err
		}
		if len(coverage.Inputs) > 0 || coverage.UpstreamPin != "" {
			return fmt.Errorf("--reachability takes a generic workload manifest (no upstreamPin, no inputs)")
		}
	}
	r, err := tsfront.LowerProjectWithOptions(config, args, registry, coverage, tsfront.LowerOptions{Externs: modules, TrapOnFailure: true})
	if err != nil {
		return err
	}
	out, _ := f.GetString("output")
	evidence := filepath.Join(out, "evidence")
	// Lowering traps shared with Go, before any ABAP refusal is trapped.
	if err := writeEvidence(filepath.Join(out, "evidence-lowering"), r); err != nil {
		return err
	}
	if err := r.Err(); err != nil {
		return err
	}
	gap, err := json.MarshalIndent(tsfront.MeasureABAPTestGap(r), "", "  ")
	if err != nil {
		return err
	}
	if err := writeSources(evidence, map[string]string{"abap-gap.json": string(gap) + "\n"}); err != nil {
		return err
	}
	if err := tsfront.TrapABAPTestRefusals(r); err != nil {
		return err
	}
	if err := writeEvidence(evidence, r); err != nil {
		return err
	}
	files, names, err := abap.EmitNamed(r.Prog)
	if err != nil {
		return err
	}
	sourceMap, err := abap.SourceMap(r.Prog, names)
	if err != nil {
		return err
	}
	if order != nil {
		if files[abapMainClass+".clas.abap"], err = abapMain(r.Prog, order, names); err != nil {
			return err
		}
		var entries map[string]abap.SourceEntry
		if err := json.Unmarshal(sourceMap, &entries); err != nil {
			return err
		}
		entries[strings.ToUpper(abapMainClass)] = abap.SourceEntry{ID: "generated." + strings.ToUpper(abapMainClass), Kind: "runtime", Source: "cmd/abapiti/emit_abap.go"}
		if sourceMap, err = json.MarshalIndent(entries, "", " "); err != nil {
			return err
		}
	}
	if err := writeSources(filepath.Join(out, "src"), files); err != nil {
		return err
	}
	report := abapsize.Report(files)
	lines, bytes := 0, 0
	for _, file := range report.Files {
		lines += file.Lines
		bytes += file.Bytes
	}
	long := append([]string{}, report.Errors()...)
	sort.Strings(long)
	size, err := json.MarshalIndent(map[string]any{"files": len(files), "lines": lines, "bytes": bytes, "longLines": len(report.LongLines), "longLineExamples": long[:min(len(long), 20)], "warnings": report.Warnings(abapsize.DefaultThresholds)}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeSources(evidence, map[string]string{"names.json": string(sourceMap), "abap-size.json": string(size) + "\n"}); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "emit-abap: %d files, %d lines, %d bytes, %d lines over 255\n", len(files), lines, bytes, len(report.LongLines))
	return nil
}

const abapMainClass = "zcl_abapiti_main"

// abapMain is the program entry: ABAP class constructors run lazily, so RUN
// initializes every module explicitly, in the given dependency order (the
// TS-HA runner's order, without test registration).
func abapMain(p *hir.Program, order []string, names *hir.Names) (string, error) {
	modules := map[string]*hir.Class{}
	for _, c := range p.Classes {
		modules[c.Name] = c
	}
	var body strings.Builder
	for _, a := range tsfront.ModuleInitializations(modules, order, false) {
		fmt.Fprintf(&body, "%s=>%s( ).\n", names.Get(a.Class), names.Get(a.Name))
	}
	return abap.Format("CLASS " + abapMainClass + " DEFINITION PUBLIC FINAL CREATE PUBLIC.\nPUBLIC SECTION.\nCLASS-METHODS run.\nENDCLASS.\nCLASS " + abapMainClass + " IMPLEMENTATION.\nMETHOD run.\n" + body.String() + "ENDMETHOD.\nENDCLASS.\n")
}
