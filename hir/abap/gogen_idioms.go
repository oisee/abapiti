package abap

// map.insertions needs an insertion counter in the ABAP map ABI; a live Map
// for-of refuses at its location until then.
// The telemetry mapping uses elapsed microseconds; it cannot implement the
// epoch clock. Until a timestamp ABI exists ABAP refuses at the call site.
func init() {
	for _, op := range []string{"clock.epochMilliseconds", "weak.lookupKey", "weak.writeKey", "map.insertions"} {
		unsupportedOps[op] = true
	}
}
