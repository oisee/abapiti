package golang

import (
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

var supportedOps = map[string]bool{
	"array.push": true, "array.get": true, "array.length": true,
	"map.set": true, "map.get": true, "map.has": true, "map.size": true, "map.keys": true,
	"set.add": true, "set.has": true, "set.size": true, "set.values": true,
	"string.length": true, "string.concat": true, "string.substring": true, "string.charCodeAt": true,
	"i32.toString": true, "i64.toString": true, "i64.remainder2": true,
	"number.remainder2": true, "number.index": true, "number.fromI32": true, "number.toString": true,
	"dynamic.of": true, "dynamic.isString": true, "dynamic.isFunction": true, "dynamic.asString": true, "dynamic.asRef": true,
}

// RuntimeOps reports the complete catalogue partition in sorted order.
func RuntimeOps() (supported, unsupported []string) {
	all := map[string]bool{}
	for op := range hir.RuntimeSpecs {
		all[op] = true
	}
	for op := range hir.SpecialOps {
		all[op] = true
	}
	for op := range all {
		if supportedOps[op] {
			supported = append(supported, op)
		} else {
			unsupported = append(unsupported, op)
		}
	}
	sort.Strings(supported)
	sort.Strings(unsupported)
	return
}

func (b *body) runtime(x *hir.Expr) string {
	if !supportedOps[x.Op] {
		b.e.unsupported(x.Node, x.Op)
		return ""
	}
	a := b.expr(x.X)
	args := []string{}
	ps, _, _ := hir.RuntimeSignature(x.Op, x.X.Type)
	for i, v := range x.Args {
		args = append(args, b.value(v, ps[i]))
	}
	code := ""
	switch x.Op {
	case "array.push", "array.get", "map.set", "map.get", "map.has", "map.keys", "set.add", "set.has", "set.values":
		code = a + "." + strings.Split(x.Op, ".")[1] + "(" + strings.Join(args, ",") + ")"
	case "array.length", "set.size":
		code = "int32(len(" + a + ".Items))"
	case "map.size":
		code = "int32(len(" + a + ".Entries))"
	case "string.length":
		code = a + ".length()"
	case "string.concat":
		code = a + "+" + args[0]
	case "string.substring":
		code = a + ".substring(" + strings.Join(args, ",") + ")"
	case "string.charCodeAt":
		code = a + ".charCodeAt(" + args[0] + ")"
	case "i32.toString", "i64.toString":
		code = "integerString(int64(" + a + "))"
	case "i64.remainder2":
		code = a + "%2"
	case "number.remainder2":
		code = "numberRemainder2(" + a + ")"
	case "number.index":
		code = "numberIndex(" + a + ")"
	case "number.fromI32":
		code = "float64(" + a + ")"
	case "number.toString":
		code = "numberString(" + a + ")"
	case "dynamic.of":
		code = "box(" + a + ")"
	case "dynamic.isString":
		code = a + "!=nil && " + a + ".Tag==1"
	case "dynamic.isFunction":
		code = "false" // ClassValue construction is explicitly unsupported.
	case "dynamic.asString":
		code = "dynString(" + a + ")"
	case "dynamic.asRef":
		code = "castRef[" + b.e.typ(x.Type) + "](dynRef(" + a + "))"
	}
	if (x.Op == "array.get" || x.Op == "map.get") && x.Type.Args[0].IsRef() {
		code += ".Value"
	}
	return b.temp(x.Type, code)
}
