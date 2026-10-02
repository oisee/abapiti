package wasm

import (
	"os"
	"testing"
)

func TestCompileMultiClassQuickJS(t *testing.T) {
	data, err := os.ReadFile("testdata/quickjs_eval.wasm")
	if err != nil {
		t.Skipf("QuickJS WASM not found: %v", err)
	}

	mod, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := mustCompileMultiClass(t, mod, "zcl_qjs", DefaultClassLines)

	t.Logf("Stats:")
	t.Logf("  Total functions:    %d", result.Stats.TotalFunctions)
	t.Logf("  Duplicate functions: %d (%.1f%%)", result.Stats.DuplicateFunctions,
		100*float64(result.Stats.DuplicateFunctions)/float64(result.Stats.TotalFunctions))
	t.Logf("  Saved instructions: %d", result.Stats.SavedInstructions)
	t.Logf("  Chunk count:        %d", result.Stats.ChunkCount)
	t.Logf("  Funcs per chunk:    %d", result.Stats.FuncsPerChunk)
	t.Logf("  Classes: %d, max lines: %d, cross-chunk calls: %d", result.Stats.ChunkCount+2, result.Stats.MaxClassLines, result.Stats.CrossChunkCalls)
	t.Logf("  Total lines:        %d", result.Stats.TotalLines)

	t.Logf("\nMain class: %d bytes", len(result.MainClass))
	t.Logf("State class: %d bytes", len(result.StateClass))

	for name, src := range result.ChunkClasses {
		t.Logf("Chunk %s: %d bytes", name, len(src))
	}

	// Write all files for inspection
	outDir := testOutDir(t)

	for name, src := range result.Files("zcl_qjs") {
		os.WriteFile(outDir+"/"+name, []byte(src), 0644)
	}

	t.Logf("\nWritten to %s/", outDir)
}
