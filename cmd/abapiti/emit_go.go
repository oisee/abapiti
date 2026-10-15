package main

import (
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
	f.String("entry", "", "Optional static no-argument entry method (file.ts.Class.method)")
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
	registry, _ := overrides.New()
	r, err := tsfront.LowerProjectWithOptions(config, args, registry, nil, tsfront.LowerOptions{Externs: modules})
	if err != nil {
		return err
	}
	if len(r.Blocking) > 0 {
		return fmt.Errorf("lowering: %s", r.Blocking[0])
	}
	if len(r.Verification) > 0 {
		return r.Verification[0]
	}
	files, err := gohir.Emit(r.Prog)
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
	files["main.go"] = "package main\nfunc main(){" + call + "}\n"
	files["go.mod"] = tsfront.ExternGoMod("generated", modules, filepath.Dir(config))
	out, _ := f.GetString("output")
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
