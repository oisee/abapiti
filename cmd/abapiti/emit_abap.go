package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
	f.Bool("keep-going", false, "Measurement mode: record type/declaration-level refusals in evidence/unattributable-refusals.json, omit the unemittable declaration and finish emission; body refusals still trap whole owning bodies")
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
	keepGoing, _ := f.GetBool("keep-going")
	gap := tsfront.MeasureABAPTestGap(r)
	if keepGoing {
		gap = tsfront.MeasureABAPTestGapKeepGoing(r)
	}
	data, err := json.MarshalIndent(gap, "", "  ")
	if err != nil {
		return err
	}
	if err := writeSources(evidence, map[string]string{"abap-gap.json": string(data) + "\n"}); err != nil {
		return err
	}
	// Unattributable refusals are written before emission: a later refusal
	// then leaves the record of every construct that has no owning body.
	var unattributable []tsfront.UnattributableRefusal
	if keepGoing {
		if unattributable, err = tsfront.TrapABAPTestRefusalsKeepGoing(r); err != nil {
			return err
		}
		rows, err := json.MarshalIndent(unattributable, "", "  ")
		if err != nil {
			return err
		}
		if err := writeSources(evidence, map[string]string{"unattributable-refusals.json": string(rows) + "\n"}); err != nil {
			return err
		}
	} else if err = tsfront.TrapABAPTestRefusals(r); err != nil {
		return err
	}
	if err := writeEvidence(evidence, r); err != nil {
		return err
	}
	var files map[string]string
	var names *hir.Names
	if keepGoing {
		files, names, err = abap.EmitNamedKeepGoing(r.Prog)
	} else {
		files, names, err = abap.EmitNamed(r.Prog)
	}
	if err != nil {
		return err
	}
	sourceMap, err := abap.SourceMap(r.Prog, names)
	if err != nil {
		return err
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
	fmt.Fprintf(cmd.OutOrStdout(), "emit-abap: %d files, %d lines, %d bytes, %d lines over 255", len(files), lines, bytes, len(report.LongLines))
	if keepGoing {
		fmt.Fprintf(cmd.OutOrStdout(), ", %d unattributable refusals", len(unattributable))
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return nil
}
