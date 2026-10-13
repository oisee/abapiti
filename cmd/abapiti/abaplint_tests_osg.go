package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/tsfront"
	"github.com/spf13/cobra"
)

func runAbaplintTestsOSG(cmd *cobra.Command, dir, out string, lowering *tsfront.RegistryLowering) error {
	files, err := tsfront.EmitAbaplintTestsOSG(dir, lowering)
	if err != nil {
		return err
	}
	abap, err := filepath.Abs(filepath.Join(out, "abap"))
	if err != nil {
		return err
	}
	if err = writeSources(abap, files); err != nil {
		return err
	}
	osg, _ := cmd.Flags().GetString("osg-home")
	tool, _ := cmd.Flags().GetString("osg-runner")
	if osg == "" {
		return fmt.Errorf("--target osg requires --osg-home (clean checkout at .github/ci/osgo.ref)")
	}
	// Release gogen's registry/IR before starting the memory-heavy Go compiler.
	nodeArgs := []string{"--expose-gc"}
	// The gogen frontend peaks near 11 GiB on the largest shard; an explicit
	// NODE_OPTIONS heap size wins.
	if !strings.Contains(os.Getenv("NODE_OPTIONS"), "max-old-space-size") {
		nodeArgs = append(nodeArgs, "--max-old-space-size=12288")
	}
	run := exec.CommandContext(cmd.Context(), "node", append(nodeArgs, tool, abap, filepath.Join(out, "osg"), osg)...)
	run.Stdout, run.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
	return run.Run()
}
