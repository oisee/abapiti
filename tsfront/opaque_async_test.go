package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveAsyncRemainsBlocking(t *testing.T) {
	for _, source := range []string{
		`export class Probe { async run(): Promise<number> { return 1; } }`,
		`export async function run(): Promise<number> { return 1; }`,
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"es2020"},"files":["input.ts"]}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "input.ts"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		_, diags := lowerProbe(t, dir)
		blocked := false
		for _, d := range diags {
			blocked = blocked || d.Category == "unsupported-async"
		}
		if !blocked {
			t.Fatalf("live async body accepted: %v", diags)
		}
	}
}

func TestNullCannotMasqueradeAsUndefined(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true},"files":["input.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input.ts"), []byte(`export class Probe { run(): boolean { const value: number | null = null; return value === undefined; } }`), 0600); err != nil {
		t.Fatal(err)
	}
	prog, diags := lowerProbe(t, dir)
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	if !strings.Contains(hir.Dump(prog), "dynamic.null") {
		t.Fatal("null lost its distinct tag", hir.Dump(prog))
	}
}
