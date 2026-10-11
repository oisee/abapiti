package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/oisee/abapiti/tsfront"
	"github.com/spf13/cobra"
)

var abaplintTestsCmd = &cobra.Command{
	Use:   "abaplint-tests <materialized-test-closure> -o <outdir>",
	Short: "Build and run abaplint's own tests on TS-HG@Go (trap failed bodies)",
	Args:  cobra.ExactArgs(1),
	RunE:  runAbaplintTests,
}

func init() {
	abaplintTestsCmd.Flags().StringP("output", "o", "", "Output module and lowering evidence (required)")
	_ = abaplintTestsCmd.MarkFlagRequired("output")
	rootCmd.AddCommand(abaplintTestsCmd)
}
func runAbaplintTests(cmd *cobra.Command, args []string) error {
	out, _ := cmd.Flags().GetString("output")
	lowering, err := tsfront.LowerAbaplintTests(args[0])
	if err != nil {
		return err
	}
	// Evidence is always written, including when a structural gate fails.
	if err = writeEvidence(filepath.Join(out, "evidence"), lowering); err != nil {
		return err
	}
	type count struct {
		Category string `json:"category"`
		Count    int    `json:"count"`
	}
	counts := map[string]int{}
	for _, d := range lowering.Blocking {
		counts[d.Category]++
	}
	ranked := []count{}
	for category, n := range counts {
		ranked = append(ranked, count{category, n})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Count == ranked[j].Count {
			return ranked[i].Category < ranked[j].Category
		}
		return ranked[i].Count > ranked[j].Count
	})
	report := struct {
		Files, Classes, Interfaces, Blocking, HIRErrors, CoverageTraps int
		Ranked                                                         []count
	}{
		len(lowering.Files), len(lowering.Prog.Classes), len(lowering.Prog.Interfaces), len(lowering.Blocking), len(lowering.Verification), lowering.TrapCount(), ranked,
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = writeSources(filepath.Join(out, "evidence"), map[string]string{"test-gate.json": string(raw) + "\n"}); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "test closure: %d files, %d blocking diagnostics, %d HIR errors, %d coverage traps\n", report.Files, report.Blocking, report.HIRErrors, report.CoverageTraps)
	files, err := tsfront.EmitAbaplintTests(args[0], lowering)
	// The Go emitter can add target-specific traps (e.g. unreviewed regexps).
	if evidenceErr := writeEvidence(filepath.Join(out, "evidence"), lowering); evidenceErr != nil {
		return evidenceErr
	}
	diagnostics, diagnosticErr := json.MarshalIndent(lowering.Diagnostics, "", "  ")
	if diagnosticErr != nil {
		return diagnosticErr
	}
	if diagnosticErr = writeSources(filepath.Join(out, "evidence"), map[string]string{"test-diagnostics.json": string(diagnostics) + "\n"}); diagnosticErr != nil {
		return diagnosticErr
	}
	if err != nil {
		return err
	}
	dir, err := filepath.Abs(filepath.Join(out, "go"))
	if err != nil {
		return err
	}
	if err = writeSources(dir, files); err != nil {
		return err
	}
	build := exec.CommandContext(cmd.Context(), "go", "build", "-o", "abaplint-tests-go", ".")
	build.Dir = dir
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("test module build: %w\n%s", err, output)
	}
	run := exec.CommandContext(cmd.Context(), filepath.Join(dir, "abaplint-tests-go"))
	run.Dir = dir
	run.Stdout, run.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
	return run.Run()
}
