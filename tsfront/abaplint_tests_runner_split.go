package tsfront

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oisee/abapiti/hir"
)

var testShardIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func testShardMethod(shard string) string {
	if len(shard) <= 26 && testShardIdentifier.MatchString(shard) {
		return "run_" + shard
	}
	// Reuse the emitter's qualified-identity sanitization/hash, retaining its
	// hash suffix and the ABAP 30-character limit with the longer run_ prefix.
	return "run_" + hir.NewNames().Get(shard)[4:]
}

// Keep complete initialization actions together: splitting inside TRY/IF would
// change exception handling. Scratch variables live on the runner, reset by RUN
// exactly where its former local declarations initialized them. Inline host
// messages remain local to their single action; skip_init is passed unchanged.
func testShardMethods(method string, count int, blocks []string) (string, string) {
	const budget = 200
	var declarations, methods, calls strings.Builder
	var chunk strings.Builder
	statements, index := 0, 0
	flush := func() {
		if chunk.Len() == 0 {
			return
		}
		name := "init_" + hir.NewNames().Get(fmt.Sprintf("%s.chunk.%d", method, index))[5:]
		index++
		fmt.Fprintf(&declarations, "CLASS-METHODS %s IMPORTING skip_init TYPE string.\n", name)
		fmt.Fprintf(&methods, "METHOD %s.\n%sENDMETHOD.\n", name, chunk.String())
		fmt.Fprintf(&calls, "%s( skip_init = skip_init ).\n", name)
		chunk.Reset()
		statements = 0
	}
	for _, block := range blocks {
		// addAction emits exactly one complete ABAP statement per source line;
		// formatting (including the 255-byte wrapping) happens after splitting.
		size := strings.Count(block, "\n")
		if statements+size > budget {
			flush()
		}
		chunk.WriteString(block)
		statements += size
	}
	flush()
	fmt.Fprintf(&declarations, "CLASS-METHODS %s IMPORTING start_index TYPE i skip_init TYPE string RETURNING VALUE(result) TYPE string.\n", method)
	fmt.Fprintf(&methods, "METHOD %s.\ncheckpoint( '{\"kind\":\"plan\",\"initializations\":%d}' ).\n%sENDMETHOD.\n", method, count, calls.String())
	return declarations.String(), methods.String()
}
