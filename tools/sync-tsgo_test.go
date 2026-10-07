package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSyncScriptRejectsDirtySource checks the reproducibility guard of
// sync-tsgo.sh: a source checkout that is at the pinned commit but has
// local modifications or untracked files must be refused before anything
// is copied into internal/tsgo.
//
// The test builds a tiny git repository, rewrites the script's pin to that
// repository's HEAD, and runs a copy of the script rooted in a temporary
// directory, so the real internal/tsgo is never touched.
func TestSyncScriptRejectsDirtySource(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	root := t.TempDir()
	upstream := filepath.Join(root, "upstream")
	if err := os.MkdirAll(filepath.Join(upstream, "internal", "ast"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Hermetic git: ignore the user's and system's config files.
	gitEnv := append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = upstream
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(upstream, "LICENSE"), []byte("license\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upstream, "internal", "ast", "ast.go"), []byte("package ast\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "-q", "-m", "init")
	head := git("rev-parse", "HEAD")

	// Copy the script, pointing its pin at the test repository's HEAD so
	// the dirty check (which runs after the pin check) is what fires.
	src, err := os.ReadFile("sync-tsgo.sh")
	if err != nil {
		t.Fatalf("read sync-tsgo.sh: %v", err)
	}
	pin := regexp.MustCompile(`(?m)^TSGO_COMMIT=[0-9a-f]+$`)
	if !pin.Match(src) {
		t.Fatal("no TSGO_COMMIT pin line found in sync-tsgo.sh")
	}
	script := filepath.Join(root, "tools", "sync-tsgo.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, pin.ReplaceAll(src, []byte("TSGO_COMMIT="+head)), 0o755); err != nil {
		t.Fatal(err)
	}

	runScript := func() (int, string) {
		t.Helper()
		cmd := exec.Command("bash", script)
		cmd.Env = append(os.Environ(), "TSGO_SRC="+upstream)
		out, err := cmd.CombinedOutput()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatalf("run script: %v\n%s", err, out)
		}
		return code, string(out)
	}

	for _, tc := range []struct {
		name  string
		dirty func()
		clean func()
	}{
		{
			name: "tracked modification",
			dirty: func() {
				write(t, filepath.Join(upstream, "LICENSE"), "dirty\n")
			},
			clean: func() {
				write(t, filepath.Join(upstream, "LICENSE"), "license\n")
			},
		},
		{
			name: "untracked file",
			dirty: func() {
				write(t, filepath.Join(upstream, "stray.txt"), "stray\n")
			},
			clean: func() {
				os.Remove(filepath.Join(upstream, "stray.txt"))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.dirty()
			defer tc.clean()
			code, out := runScript()
			if code == 0 {
				t.Errorf("script accepted a dirty source, exit 0")
			}
			if !strings.Contains(out, "dirty") {
				t.Errorf("script output does not mention the dirty tree:\n%s", out)
			}
			if _, err := os.Stat(filepath.Join(root, "internal", "tsgo")); !os.IsNotExist(err) {
				t.Errorf("dirty source was synced anyway (internal/tsgo exists: err=%v)", err)
			}
		})
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
