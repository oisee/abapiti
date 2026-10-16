package abap

// Node platform calls have no ABAP host ABI: located refusal at the call.
func init() {
	for _, op := range []string{"string.sha256Hex", "path.basename", "fs.readText", "fs.readdirNames", "fs.readdirEntries", "util.isDeepStrictEqual", "path.join"} {
		unsupportedOps[op] = true
	}
}
