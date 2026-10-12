package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront"
	"github.com/oisee/abapiti/tsfront/overrides"
	"github.com/spf13/cobra"
)

func newLowerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lower --full --tsconfig path file...",
		Short: "Measure an unpruned TypeScript project without emitting targets",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runLower,
	}
	f := cmd.Flags()
	f.Bool("full", false, "Measure all bodies; required (no pruning or target emission)")
	f.String("tsconfig", "tsconfig.json", "Project tsconfig; file arguments are relative to its directory")
	f.StringP("output", "o", "out", "Evidence output directory")
	f.String("overrides", "none", "Source-pinned overrides: none or registry")
	f.Bool("assume-int", true, "Assume only integer calculations, as in abaplint --full")
	f.String("scope", "", "Report only diagnostics beneath this tsconfig-relative source directory")
	return cmd
}

func init() { rootCmd.AddCommand(newLowerCmd()) }

func sourceInScope(loc, dir string) bool {
	if !locationSuffix.MatchString(loc) {
		return false // Synthetic/global errors have no attributable source file.
	}
	file := locationSuffix.ReplaceAllString(loc, "")
	abs, err := filepath.Abs(file)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(dir, abs)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func scopedLowering(r *tsfront.RegistryLowering, dir string) *tsfront.RegistryLowering {
	s := *r
	s.Blocking = nil
	s.Verification = nil
	for _, d := range r.Blocking {
		if sourceInScope(d.Loc, dir) {
			s.Blocking = append(s.Blocking, d)
		}
	}
	for _, err := range r.Verification {
		if e, ok := err.(hir.Error); ok && sourceInScope(e.Source, dir) {
			s.Verification = append(s.Verification, err)
		}
	}
	return &s
}

func writeMeasurementJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func runLower(cmd *cobra.Command, args []string) error {
	f := cmd.Flags()
	full, _ := f.GetBool("full")
	if !full {
		return fmt.Errorf("lower currently requires --full")
	}
	config, _ := f.GetString("tsconfig")
	config, err := filepath.Abs(config)
	if err != nil {
		return err
	}
	out, _ := f.GetString("output")
	mode, _ := f.GetString("overrides")
	var registry *overrides.Registry
	switch mode {
	case "none":
		registry, err = overrides.New()
	case "registry":
		registry, err = tsfront.RegistryOverrides()
	default:
		return fmt.Errorf("--overrides: use none or registry")
	}
	if err != nil {
		return err
	}
	assumeInt, _ := f.GetBool("assume-int")
	r, err := tsfront.LowerProjectWithOptions(config, args, registry, nil, tsfront.LowerOptions{AssumeOnlyIntegerCalculations: assumeInt})
	if err != nil {
		return err
	}
	dir := filepath.Join(out, "evidence")
	if err := writeEvidence(dir, r); err != nil {
		return err
	}
	if err := writeMeasurementJSON(filepath.Join(dir, "closure-gap-summary.json"), summarizeGaps(r)); err != nil {
		return err
	}
	// Checker errors are evidence, not lowering blockers (e.g. version skew).
	p, err := tsfront.Load(config)
	if err != nil {
		return err
	}
	checker := p.CheckerDiagnostics(cmd.Context())
	scope, _ := f.GetString("scope")
	if scope != "" {
		scope = filepath.Join(filepath.Dir(config), scope)
		r = scopedLowering(r, scope)
		filtered := []tsfront.Diagnostic{}
		for _, d := range checker {
			if sourceInScope(d.Loc, scope) {
				filtered = append(filtered, d)
			}
		}
		checker = filtered
	}
	if err := writeMeasurementJSON(filepath.Join(dir, "checker-diagnostics.json"), checker); err != nil {
		return err
	}
	summary := summarizeGaps(r)
	if err := writeMeasurementJSON(filepath.Join(dir, "gap-summary.json"), summary); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "full: %d blocking (%d root, %d cascade), %d HIR errors in %d files\n", summary.Blocking, summary.RootBlocking, summary.CascadeBlocking, summary.HIRErrors, summary.Files)
	return nil
}
