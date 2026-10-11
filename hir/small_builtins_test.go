package hir

import "testing"

func TestSmallRuntimeSignatures(t *testing.T) {
	for _, op := range []string{"string.trimStart", "string.trimEnd"} {
		args, result, ok := RuntimeSignature(op, T(String))
		if !ok || len(args) != 0 || !result.Equal(T(String)) || RuntimeSpecs[op].Mutates {
			t.Fatalf("%s: %v %v %v", op, args, result, ok)
		}
		if _, _, ok := RuntimeSignature(op, T(I32)); ok {
			t.Fatal("trim accepted numeric receiver")
		}
	}
	for _, key := range []Type{T(String), T(I32), Ref("Key")} {
		args, result, ok := RuntimeSignature("map.delete", T(OrderedMap, key, T(Number)))
		if !ok || len(args) != 1 || !args[0].Equal(key) || !result.Equal(T(Bool)) || !RuntimeSpecs["map.delete"].Mutates {
			t.Fatalf("map.delete: %v %v %v", args, result, ok)
		}
	}
	if _, _, ok := RuntimeSignature("map.delete", T(OrderedSet, T(String))); ok {
		t.Fatal("map.delete accepted set receiver")
	}
}
