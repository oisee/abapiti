package hir

// Typed Node platform operations (tsfront/lower_node_platform.go). The
// receiver is the path or hashed text; results are fresh values.
var nodePlatformOps = []string{"host.stderrWrite", "string.sha256Hex", "path.basename", "fs.readText", "fs.readdirNames", "fs.readdirEntries"}

func init() {
	for _, op := range nodePlatformOps {
		RuntimeSpecs[op] = RuntimeSpec{String, 0, false}
	}
	// path.join over a string array (the segments, already evaluated).
	RuntimeSpecs["path.join"] = RuntimeSpec{Array, 0, false}
	// util.isDeepStrictEqual(a, b): both operands boxed, a fresh boolean.
	RuntimeSpecs["util.isDeepStrictEqual"] = RuntimeSpec{Dynamic, 1, false}
}

func nodePlatformSignature(op string) ([]Type, Type, bool) {
	switch op {
	case "host.stderrWrite":
		return nil, T(Void), true
	case "string.sha256Hex", "path.basename", "fs.readText":
		return nil, T(String), true
	case "path.join":
		return nil, T(String), true
	case "util.isDeepStrictEqual":
		return []Type{T(Dynamic)}, T(Bool), true
	case "fs.readdirNames":
		return nil, T(Array, T(String)), true
	case "fs.readdirEntries":
		return nil, T(Array, Ref("builtin.Dirent")), true
	}
	return nil, Type{}, false
}
