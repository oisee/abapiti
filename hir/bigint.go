package hir

func init() {
	RuntimeSpecs["bigint.decimal"] = RuntimeSpec{String, 0, false}
	RuntimeSpecs["bigint.neg"] = RuntimeSpec{BigInt, 0, false}
	RuntimeSpecs["bigint.toString"] = RuntimeSpec{BigInt, 0, false}
	RuntimeSpecs["bigint.compare"] = RuntimeSpec{BigInt, 1, false}
	RuntimeSpecs["dynamic.asBigInt"] = RuntimeSpec{Dynamic, 0, false}
}

func bigintRuntimeSignature(op string) ([]Type, Type, bool) {
	switch op {
	case "bigint.decimal", "bigint.neg", "dynamic.asBigInt":
		return nil, T(BigInt), true
	case "bigint.toString":
		return nil, T(String), true
	case "bigint.compare":
		return []Type{T(BigInt)}, T(Number), true
	}
	return nil, Type{}, false
}

// BigInt literals have one spelling: zero or a nonzero signed decimal integer.
func CanonicalBigInt(s string) bool {
	if s == "0" {
		return true
	}
	if len(s) > 0 && s[0] == '-' {
		s = s[1:]
	}
	if len(s) == 0 || s[0] == '0' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
