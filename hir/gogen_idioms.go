package hir

func init() {
	RuntimeSpecs["clock.epochMilliseconds"] = RuntimeSpec{I32, 0, false}
	RuntimeSpecs["weak.lookupKey"] = RuntimeSpec{Dynamic, 0, false}
	RuntimeSpecs["weak.writeKey"] = RuntimeSpec{Dynamic, 0, false}
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
	}
	return nil, Type{}, false
}
