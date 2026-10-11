package hir

import "testing"

func TestOptionalMapReadSignature(t *testing.T) {
	for _, value := range []Type{T(Number), T(String), T(Bool), Ref(RootObject), T(OrderedMap, T(String), T(Number))} {
		optional := T(Optional, value)
		for _, payload := range []Type{value, optional} {
			args, result, ok := RuntimeSignature("map.get", T(OrderedMap, T(String), payload))
			if !ok || len(args) != 1 || !args[0].Equal(T(String)) || !result.Equal(optional) {
				t.Fatalf("map payload %s: %v %s %v", payload, args, result, ok)
			}
		}
	}
}
