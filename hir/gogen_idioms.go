package hir

func init() {
	RuntimeSpecs["clock.epochMilliseconds"] = RuntimeSpec{I32, 0, false}
	RuntimeSpecs["weak.lookupKey"] = RuntimeSpec{Dynamic, 0, false}
	RuntimeSpecs["weak.writeKey"] = RuntimeSpec{Dynamic, 0, false}
	// Lifetime count of new-key insertions: a live Map for-of proves no entry
	// was appended (or re-appended) behind its key snapshot.
	RuntimeSpecs["map.insertions"] = RuntimeSpec{OrderedMap, 0, false}
}

func gogenRuntimeSignature(op string) ([]Type, Type, bool) {
	if ps, r, ok := nodePlatformSignature(op); ok {
		return ps, r, true
	}
	switch op {
	case "clock.epochMilliseconds":
		return nil, T(Number), true
	case "weak.lookupKey", "weak.writeKey":
		return nil, Ref(RootObject), true
	case "map.insertions":
		return nil, T(I32), true
	}
	return nil, Type{}, false
}
