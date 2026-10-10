package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/inlineoracle"
)

func TestGraceUninitializedCalleeLoop(t *testing.T) {
	// The declaration requires a fresh frame on each iteration (70394ff).
	source := `export class Probe {
 late(k: number): number { let r: number | undefined; if (k > 0) r = k; return r === undefined ? -1 : r; }
 run(): number[] { const out: number[] = []; for (const k of [5, -1]) out.push(this.late(k)); return out; }
 }`
	dir := t.TempDir()
	config := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(config, []byte(`{"compilerOptions":{"strictNullChecks":true,"target":"es2020"},"files":["probe.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	// The source oracle executes the same function body in Node.
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for the loop source oracle")
	}
	js := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(source, "export ", ""), ": number | undefined", ""), ": number[]", ""), ": number", "")
	out, err := exec.Command(node, "-e", js+`; console.log(JSON.stringify(new Probe().run()));`).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "[5,-1]" {
		t.Fatalf("source result %q: %v", out, err)
	}
	p, err := Load(config)
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.Lower([]string{"probe.ts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Fatal(d)
		}
	}
	inlineoracle.Check(t, prog)
	stats, err := rewrite.Inline(prog)
	if err != nil {
		t.Fatal(err)
	}
	for name, n := range stats.Callees {
		if strings.HasSuffix(name, ".late") && n != 0 {
			t.Fatalf("late inlined: %+v", stats)
		}
	}
	if !strings.Contains(hir.Dump(prog), "late") {
		t.Fatal("loop callee disappeared")
	}
	db, err := rewrite.ExtractRewriteFacts(prog)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range db.Facts("inline_uninitialized") {
		if strings.HasSuffix(row[0], "::late") {
			found = true
		}
	}
	if !found {
		t.Fatal("fixture no longer contains an uninitialized declaration in late")
	}
}
