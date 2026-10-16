package hir

func init() {
	for _, op := range []string{"number.isInteger", "number.isSafeInteger", "number.isFinite", "number.strictIsNaN", "dynamic.values"} {
		RuntimeSpecs[op] = RuntimeSpec{Dynamic, 0, false}
	}
	RuntimeSpecs["number.abs"] = RuntimeSpec{Number, 0, false}
	RuntimeSpecs["number.hex16"] = RuntimeSpec{Number, 0, false}
	RuntimeSpecs["host.getenv"] = RuntimeSpec{String, 0, false}
	RuntimeSpecs["string.codePoints"] = RuntimeSpec{String, 0, false}
	RuntimeSpecs["arrayLike.length"] = RuntimeSpec{Dynamic, 0, false}
}
