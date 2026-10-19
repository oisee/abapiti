package hir

import "testing"

func TestNullableMatchBoxingSignature(t *testing.T) {
	for _, typ := range []Type{T(Optional, T(Array, T(String))), T(Array, T(String)), T(Optional, T(String)), T(Optional, T(Array, T(Dynamic))), T(Optional), T(Dynamic)} {
		args, result, ok := RuntimeSignature("regexp.boxMatch", typ)
		want := typ.Equal(T(Optional, T(Array, T(String))))
		if ok != want || ok && (len(args) != 0 || !result.Equal(T(Dynamic))) {
			t.Fatalf("%s: %v %v %v", typ, args, result, ok)
		}
	}
	if RuntimeSpecs["regexp.boxMatch"].Mutates {
		t.Fatal("boxing must not mutate regex or array storage")
	}
}
