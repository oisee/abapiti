package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestLongLineExitCode(t *testing.T) {
	if os.Getenv("ABAPITI_SIZE_HELPER") == "1" {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("allow-long-lines", false, "")
		if os.Getenv("ABAPITI_ALLOW_LONG_LINES") == "1" {
			_ = cmd.Flags().Set("allow-long-lines", "true")
		}
		err := reportAndHint(cmd, map[string]string{"synthetic.abap": strings.Repeat("x", 256) + "\n"}, "Deploy with vsp: synthetic")
		if err != nil {
			os.Exit(exitCode(err))
		}
		os.Exit(0)
	}
	for _, tc := range []struct {
		allow bool
		exit  int
	}{{false, 3}, {true, 0}} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLongLineExitCode$")
		cmd.Env = append(os.Environ(), "ABAPITI_SIZE_HELPER=1")
		if tc.allow {
			cmd.Env = append(cmd.Env, "ABAPITI_ALLOW_LONG_LINES=1")
		}
		out, err := cmd.CombinedOutput()
		got := 0
		if err != nil {
			var ok bool
			exit, yes := err.(*exec.ExitError)
			if ok = yes; ok {
				got = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if got != tc.exit {
			t.Fatalf("allow=%t: exit=%d want=%d: %s", tc.allow, got, tc.exit, out)
		}
		if !tc.allow && !strings.Contains(string(out), "synthetic.abap:1:256") {
			t.Fatalf("missing line error: %s", out)
		}
		// The deploy hint belongs to accepted output only.
		if hint := strings.Contains(string(out), "Deploy with vsp"); hint != tc.allow {
			t.Fatalf("allow=%t: deploy hint printed=%t: %s", tc.allow, hint, out)
		}
	}
}
