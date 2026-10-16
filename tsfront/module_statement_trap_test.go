package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Under the fail-closed policy a module-level declaration or statement that
// does not lower refuses where JavaScript would evaluate it. Before, the
// step-4 driver's `const skippedPaths = new Set(inputs.skippedPaths)` (an
// untyped Set<any>) and every later statement using it were dropped: the
// program exited 0 without writing anything.
func TestModuleStatementFailureTraps(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "probe.mjs"), []byte(`/** @type {string[]} */ const out = [];
/** @param {string} s @returns {void} */ function write(s) { out.push(s); }
const inputs = JSON.parse('{"skipped":["b"]}');
write("before");
const skipped = new Set(inputs.skipped);
write("after:" + skipped.size);
export class Probe {
  /** @returns {string} */ static run() {
    return out.join(",");
  }
}
`), 0600)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`), 0600)
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, ds, err := p.LowerWithOptions([]string{"probe.mjs"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	main := fmt.Sprintf(`defer func(){fmt.Println("refused:", recover())}();fmt.Println(%s())`, goEntry("probe.mjs.Probe", "run"))
	got := runGoHIR(t, prog, main)
	if !strings.HasPrefix(got, "refused:") || !strings.Contains(got, "probe.mjs:5:7") || strings.Contains(got, "after") {
		t.Fatalf("got %q", got)
	}
}

// A dropped declaration without effects (gogen's
// `const enumerate = core.Registry.prototype.getObjects`) does not refuse
// the initializer: JavaScript only reads a value there; uses trap instead.
func TestModuleEffectFreeFailureKeepsInitializer(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "probe.mjs"), []byte(`const inputs = JSON.parse('{"skipped":["b"]}');
const enumerate = Map.prototype.get;
/** @type {string[]} */ const out = ["init:" + inputs.skipped.length];
export class Probe {
  /** @returns {string} */ static run() {
    return out.join(",");
  }
}
`), 0600)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`), 0600)
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, ds, err := p.LowerWithOptions([]string{"probe.mjs"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	dropped := false
	for _, d := range ds {
		dropped = dropped || d.Category == "note-dropped-declaration" && strings.Contains(d.Message, "probe.mjs:2:")
	}
	if !dropped {
		t.Fatal("expected the dropped declaration at probe.mjs:2", ds)
	}
	if got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.mjs.Probe", "run"))); got != "init:1\n" {
		t.Fatalf("got %q", got)
	}
}
