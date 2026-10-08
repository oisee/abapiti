package hir

// RuntimeSpecs is the single supported library-operation catalogue. Signatures
// involving generic types are checked by RuntimeSignature, without target types.
type RuntimeSpec struct {
	Receiver Kind
	Arity    int
	Mutates  bool
}

var RuntimeSpecs = map[string]RuntimeSpec{
	"number.remainder2": {Number, 0, false},
	"number.index":      {Number, 0, false}, "number.fromI32": {I32, 0, false}, "number.toString": {Number, 0, false},
	"string.length": {String, 0, false}, "string.concat": {String, 1, false},
	"string.substring": {String, 2, false}, "string.charCodeAt": {String, 1, false},
	"string.charAt": {String, 1, false}, "string.substr": {String, 2, false},
	"string.trim": {String, 0, false}, "string.toUpperCase": {String, 0, false},
	"string.replaceAll": {String, 2, false}, "i32.toString": {I32, 0, false},
	"array.push": {Array, 1, true}, "array.length": {Array, 0, false}, "array.get": {Array, 1, false},
	"map.set": {OrderedMap, 2, true}, "map.get": {OrderedMap, 1, false}, "map.has": {OrderedMap, 1, false}, "map.size": {OrderedMap, 0, false},
	"set.add": {OrderedSet, 1, true}, "set.has": {OrderedSet, 1, false}, "set.size": {OrderedSet, 0, false},
	// Snapshot iteration exposes an Array to the ordinary ForEach node.
	"map.keys": {OrderedMap, 0, false}, "set.values": {OrderedSet, 0, false},
}

func RuntimeSignature(op string, t Type) ([]Type, Type, bool) {
	s, ok := RuntimeSpecs[op]
	if !ok || s.Receiver != t.Kind || (t.Kind != String && len(t.Args) != map[Kind]int{Array: 1, OrderedMap: 2, OrderedSet: 1}[t.Kind]) {
		return nil, Type{}, false
	}
	i, b := T(I32), T(Bool)
	switch op {
	case "string.length", "array.length", "map.size", "set.size":
		return nil, i, true
	case "string.concat":
		return []Type{T(String)}, T(String), true
	case "string.substring":
		return []Type{i, i}, T(String), true
	case "string.charCodeAt":
		return []Type{i}, i, true
	case "string.charAt":
		return []Type{i}, T(String), true
	case "string.trim", "string.toUpperCase":
		return nil, T(String), true
	case "string.substr":
		return []Type{i, i}, T(String), true
	case "string.replaceAll":
		return []Type{T(String), T(String)}, T(String), true
	case "number.index":
		return nil, i, true
	case "number.remainder2", "number.fromI32":
		return nil, T(Number), true
	case "number.toString", "i32.toString":
		return nil, T(String), true
	case "array.push":
		return []Type{t.Args[0]}, i, true
	case "array.get":
		return []Type{i}, T(Optional, t.Args[0]), true
	case "map.set":
		return []Type{t.Args[0], t.Args[1]}, t, true
	case "map.get":
		return []Type{t.Args[0]}, T(Optional, t.Args[1]), true
	case "map.has", "set.has":
		return []Type{t.Args[0]}, b, true
	case "set.add":
		return []Type{t.Args[0]}, t, true
	case "map.keys", "set.values":
		return nil, T(Array, t.Args[0]), true
	}
	return nil, Type{}, false
}
