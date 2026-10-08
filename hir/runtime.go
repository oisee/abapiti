package hir

// RuntimeSpecs is the single supported library-operation catalogue. Signatures
// involving generic types are checked by RuntimeSignature, without target types.
// Ops whose receiver or result type depends on the front end's context (boxed
// dynamic values, class values, run-time class tests) are special-cased by the
// verifier instead: SpecialOps.
type RuntimeSpec struct {
	Receiver Kind
	Arity    int
	Mutates  bool
}

var RuntimeSpecs = map[string]RuntimeSpec{
	"xml.parseSubset":           {String, 0, false},
	"json.parseSubset":          {String, 0, false},
	"string.compareRegistryKey": {String, 1, false},
	"string.compareObjectName":  {String, 1, false},
	"dynamic.isNullish":         {Dynamic, 0, false},
	"dynamic.get":               {Dynamic, 1, false},
	"dynamic.put":               {Dynamic, 2, true},
	"dynamic.strictEquals":      {Dynamic, 1, false},
	"dynamic.asBoolean":         {Dynamic, 0, false},
	"dynamic.null":              {Number, 0, false},
	"clock.telemetry":           {Number, 0, false},
	"number.remainder2":         {Number, 0, false},
	"number.index":              {Number, 0, false}, "number.fromI32": {I32, 0, false}, "number.toString": {Number, 0, false},
	"string.length": {String, 0, false}, "string.concat": {String, 1, false},
	"string.slice": {String, 2, false}, "string.substring": {String, 2, false}, "string.charCodeAt": {String, 1, false},
	"string.charAt": {String, 1, false}, "string.substr": {String, 2, false},
	"string.repeatIndent": {String, 1, false}, "string.trim": {String, 0, false}, "string.toUpperCase": {String, 0, false},
	"string.replaceAll": {String, 2, false}, "i32.toString": {I32, 0, false},
	"string.startsWith": {String, 1, false}, "string.endsWith": {String, 1, false},
	"string.indexOf": {String, 1, false}, "string.split": {String, 1, false},
	"string.toLowerCase":  {String, 0, false},
	"string.replaceRegex": {String, 2, false},
	"array.reverse":       {Array, 0, true},
	"array.unshift":       {Array, 1, true}, "array.push": {Array, 1, true}, "array.length": {Array, 0, false}, "array.get": {Array, 1, false},
	"array.concat": {Array, 1, false}, "array.slice0": {Array, 0, false},
	"array.slice1": {Array, 1, false}, "array.slice2": {Array, 2, false},
	"array.splice1": {Array, 1, true}, "array.splice2": {Array, 2, true},
	"array.splice3": {Array, 3, true}, "array.pop": {Array, 0, true}, "array.shift": {Array, 0, true},
	"array.indexOf": {Array, 1, false}, "array.includes": {Array, 1, false},
	"array.join": {Array, 1, false},
	"set.delete": {OrderedSet, 1, true}, "map.values": {OrderedMap, 0, false},
	"string.at": {String, 1, false}, "string.parseInt10": {String, 0, false},
	"string.replaceFirst":       {String, 2, false},
	"string.localeCompareNames": {String, 1, false},
	"record.delete":             {OrderedMap, 1, true},
	"map.set":                   {OrderedMap, 2, true}, "map.get": {OrderedMap, 1, false}, "map.has": {OrderedMap, 1, false}, "map.size": {OrderedMap, 0, false},
	"set.add": {OrderedSet, 1, true}, "set.has": {OrderedSet, 1, false}, "set.size": {OrderedSet, 0, false},
	"set.copy":      {OrderedSet, 1, false},
	"set.fromArray": {OrderedSet, 1, true},
	// Snapshot iteration exposes an Array to the ordinary ForEach node.
	"map.keys": {OrderedMap, 0, false}, "set.values": {OrderedSet, 0, false},
	"classvalue.name":   {ClassValue, 0, false},
	"classvalue.has":    {ClassValue, 1, false},
	"regexp.match_test": {RegExp, 1, false}, "regexp.test": {RegExp, 1, false}, "regexp.source": {RegExp, 0, false},
	"dynamic.isNumber": {Dynamic, 0, false}, "dynamic.asNumber": {Dynamic, 0, false}, "dynamic.typeof": {Dynamic, 0, false}, "dynamic.toString": {Dynamic, 0, false},
	"dynamic.isString": {Dynamic, 0, false}, "dynamic.isFunction": {Dynamic, 0, false}, "dynamic.isArray": {Dynamic, 0, false},
	"dynamic.asString": {Dynamic, 0, false}, "dynamic.asClassValue": {Dynamic, 0, false},
}

// SpecialOps lists ops whose typing rules depend on more than the receiver
// kind; the verifier checks them individually.
var SpecialOps = map[string]bool{
	"object.classOf":      true, // any object ref -> ClassValue
	"classvalue.new":      true, // ClassValue -> a class or interface reference
	"dynamic.of":          true, // any value -> Dynamic (tagged box)
	"dynamic.asRef":       true,
	"dynamic.materialize": true, // Dynamic -> a class or interface reference
	"regexp.new":          true, // pattern, [flags] -> RegExp
}

// optionalOf wraps t as an Optional; an optional element stays as it is
// (absent either way, never nested).
func optionalOf(t Type) Type {
	if t.Kind == Optional {
		return t
	}
	return T(Optional, t)
}

func RuntimeSignature(op string, t Type) ([]Type, Type, bool) {
	s, ok := RuntimeSpecs[op]
	if !ok || s.Receiver != t.Kind || (t.Kind != String && len(t.Args) != map[Kind]int{Array: 1, OrderedMap: 2, OrderedSet: 1}[t.Kind]) {
		return nil, Type{}, false
	}
	i, b := T(I32), T(Bool)
	switch op {
	case "xml.parseSubset", "json.parseSubset", "dynamic.null":
		return nil, T(Dynamic), true
	case "dynamic.get":
		return []Type{T(String)}, T(Dynamic), true
	case "dynamic.put":
		return []Type{T(String), T(Dynamic)}, T(Void), true
	case "dynamic.strictEquals":
		return []Type{T(Dynamic)}, T(Bool), true
	case "dynamic.isNullish", "dynamic.asBoolean":
		return nil, T(Bool), true
	case "clock.telemetry":
		return nil, T(Number), true
	case "string.length", "array.length", "map.size", "set.size":
		return nil, i, true
	case "string.repeatIndent":
		return []Type{T(Number)}, T(String), true
	case "string.concat":
		return []Type{T(String)}, T(String), true
	case "string.compareRegistryKey", "string.compareObjectName":
		return []Type{T(String)}, i, true
	case "string.substring", "string.slice":
		return []Type{i, i}, T(String), true
	case "string.charCodeAt":
		return []Type{i}, i, true
	case "string.charAt":
		return []Type{i}, T(String), true
	case "string.trim", "string.toUpperCase", "string.toLowerCase":
		return nil, T(String), true
	case "string.substr":
		return []Type{i, i}, T(String), true
	case "string.replaceAll":
		return []Type{T(String), T(String)}, T(String), true
	case "string.startsWith", "string.endsWith":
		return []Type{T(String)}, b, true
	case "string.indexOf":
		return []Type{T(String)}, i, true
	case "string.split":
		return []Type{T(String)}, T(Array, T(String)), true
	case "string.replaceRegex":
		return []Type{T(RegExp), T(String)}, T(String), true
	case "number.index":
		return nil, i, true
	case "number.remainder2", "number.fromI32":
		return nil, T(Number), true
	case "dynamic.asNumber":
		return nil, T(Number), true
	case "dynamic.typeof", "dynamic.toString":
		return nil, T(String), true
	case "dynamic.isNumber":
		return nil, b, true
	case "number.toString", "i32.toString":
		return nil, T(String), true
	case "array.push", "array.unshift":
		return []Type{t.Args[0]}, i, true
	case "array.get":
		return []Type{i}, optionalOf(t.Args[0]), true
	case "array.reverse":
		return nil, t, true
	case "array.concat":
		return []Type{t}, t, true
	case "array.slice0":
		return nil, t, true
	case "array.slice1":
		return []Type{i}, t, true
	case "array.slice2":
		return []Type{i, i}, t, true
	case "array.splice1":
		return []Type{i}, t, true
	case "array.splice2":
		return []Type{i, i}, t, true
	case "array.splice3":
		return []Type{i, i, t.Args[0]}, t, true
	case "array.pop", "array.shift":
		return nil, optionalOf(t.Args[0]), true
	case "set.delete":
		return []Type{t.Args[0]}, b, true
	case "map.values":
		return nil, T(Array, t.Args[1]), true
	case "string.at":
		// Absent when the index is out of range (JavaScript s[i] === undefined).
		return []Type{i}, T(Optional, T(String)), true
	case "string.parseInt10":
		// Radix-10 parseInt; absent stands for NaN (no digits).
		return nil, T(Optional, T(Number)), true
	case "string.replaceFirst":
		return []Type{T(String), T(String)}, T(String), true
	case "string.localeCompareNames":
		// ICU root collation restricted to the object-name domain
		// [_/0-9A-Z]; any other code unit raises.
		return []Type{T(String)}, i, true
	case "array.indexOf":
		return []Type{t.Args[0]}, i, true
	case "array.includes":
		return []Type{t.Args[0]}, b, true
	case "array.join":
		// Reference/optional elements require JavaScript ToString semantics
		// which this runtime does not implement. Never emit invalid ABAP or
		// silently substitute reference formatting.
		if t.Args[0].IsRef() || t.Args[0].Kind == Optional {
			return nil, Type{}, false
		}
		return []Type{T(Optional, T(String))}, T(String), true
	case "map.set":
		return []Type{t.Args[0], t.Args[1]}, t, true
	case "map.get":
		return []Type{t.Args[0]}, T(Optional, t.Args[1]), true
	case "record.delete", "map.has", "set.has":
		return []Type{t.Args[0]}, b, true
	case "set.add":
		return []Type{t.Args[0]}, t, true
	case "set.copy":
		return []Type{t}, t, true
	case "set.fromArray":
		return []Type{T(Array, t.Args[0])}, t, true
	case "map.keys", "set.values":
		return nil, T(Array, t.Args[0]), true
	case "classvalue.name":
		return nil, T(String), true
	case "classvalue.has":
		return []Type{T(String)}, b, true
	case "regexp.test", "regexp.match_test":
		return []Type{T(String)}, b, true
	case "regexp.source":
		return nil, T(String), true
	case "dynamic.isString", "dynamic.isFunction", "dynamic.isArray":
		return nil, b, true
	case "dynamic.asString":
		return nil, T(String), true
	case "dynamic.asClassValue":
		return nil, T(ClassValue), true
	}
	return nil, Type{}, false
}
