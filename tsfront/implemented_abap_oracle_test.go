package tsfront

import (
	"strings"
	"testing"
)

// Keep Go/Node semantics and require ABAP emission for operations whose runtime
// implementation has replaced a former located refusal. The attached ABAP Unit
// oracle is collected by the fixture gate and executed when OSG is configured.
func gogenImplementedABAPOracle(t *testing.T, source, expected string) {
	t.Helper()
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	gogenCheck(t, p, "probe.ts", source, expected)
	if !t.Failed() {
		storageABAPProbe(t, p, "probe.ts.Probe", expected, "")
	}
}

func gogenTSImplementedABAPOracle(t *testing.T, source string) {
	t.Helper()
	want := conversionOracle(t, source)
	gogenImplementedABAPOracle(t, source, strings.TrimSuffix(want, "\n"))
}
