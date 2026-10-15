package abap

// The telemetry mapping uses elapsed microseconds; it cannot implement the
// epoch clock. Until a timestamp ABI exists ABAP refuses at the call site.
func init() {
	for _, op := range []string{"clock.epochMilliseconds", "weak.lookupKey", "weak.writeKey"} {
		unsupportedOps[op] = true
	}
}
