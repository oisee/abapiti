package golang

import (
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

var supportedOps = map[string]bool{
	"string.replaceRegex": true,
	"string.toLowerCase":  true,
	"object.classOf":      true, "classvalue.new": true, "classvalue.name": true, "classvalue.has": true, "dynamic.asClassValue": true,
	"clock.telemetry": true, "xml.parseSubset": true, "dynamic.materialize": true, "json.parseSubset": true,
	"dynamic.null": true, "dynamic.strictEquals": true, "dynamic.isNullish": true, "dynamic.get": true, "dynamic.put": true, "dynamic.asBoolean": true, "dynamic.asNumber": true, "dynamic.isNumber": true, "dynamic.isArray": true, "dynamic.typeof": true, "dynamic.toString": true,

	"string.startsWith": true, "string.endsWith": true, "string.indexOf": true, "string.at": true, "string.replaceFirst": true, "string.repeatIndent": true, "string.parseInt10": true, "string.parseInt10i64": true, "string.compareRegistryKey": true, "string.compareObjectName": true, "string.localeCompareNames": true, "regexp.new": true, "regexp.test": true, "regexp.match_test": true, "regexp.source": true, "regexp.toString": true,

	"array.reverse": true, "array.unshift": true, "array.concat": true, "array.slice0": true, "array.slice1": true, "array.slice2": true, "array.splice1": true, "array.splice2": true, "array.splice3": true, "array.splice1_view": true, "array.pop": true, "array.shift": true, "array.indexOf": true, "array.includes": true, "array.join": true, "map.values": true, "record.delete": true, "set.delete": true, "set.copy": true,

	"string.substr": true, "string.trim": true, "string.slice": true, "string.charAt": true, "string.replaceAll": true, "string.split": true, "string.toUpperCase": true, "set.fromArray": true,
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
		typ := v.Type
		if i < len(ps) {
			typ = ps[i]
		}
		args = append(args, b.value(v, typ))
	}
	code := ""
	if b.e.resultDebug != nil && (strings.HasPrefix(x.Op, "array.") || strings.HasPrefix(x.Op, "map.") || strings.HasPrefix(x.Op, "set.")) {
		b.line("resultDebugSite(%q)", relativeSource(x.Source))
	}
	switch x.Op {
	case "array.reverse", "array.unshift", "array.concat", "array.slice0", "array.slice1", "array.slice2", "array.splice1", "array.splice2", "array.splice3", "array.splice1_view", "array.pop", "array.shift", "array.indexOf", "array.includes", "array.join", "map.values", "set.delete", "set.copy", "array.push", "array.get", "map.set", "map.get", "map.has", "map.keys", "set.add", "set.has", "set.values", "set.fromArray":
		code = a + "." + strings.Split(x.Op, ".")[1] + "(" + strings.Join(args, ",") + ")"
	case "record.delete":
		code = "func() bool {" + a + ".delete(" + args[0] + ");return true}()"
	case "array.length", "set.size":
		code = "int32(len(" + a + ".Items))"
	case "map.size":
		code = "int32(len(" + a + ".Entries))"
	case "string.startsWith", "string.endsWith", "string.indexOf", "string.at", "string.replaceFirst", "string.repeatIndent", "string.parseInt10", "string.parseInt10i64", "string.substr", "string.slice", "string.charAt", "string.replaceAll", "string.split":
		code = a + "." + strings.Split(x.Op, ".")[1] + "(" + strings.Join(args, ",") + ")"
	case "string.compareRegistryKey":
		code = "compareDomain(" + a + "," + args[0] + `,"_0123456789abcdefghijklmnopqrstuvwxyz")`
	case "string.compareObjectName", "string.localeCompareNames":
		code = "compareDomain(" + a + "," + args[0] + `,"_/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")`
	case "regexp.new":
		flags := "str(\"\")"
		if len(args) > 1 {
			flags = args[1]
		}
		code = "newRegExp(" + args[0] + "," + flags + ")"
	case "string.replaceRegex":
		code = a + ".replaceRegex(" + strings.Join(args, ",") + ")"
	case "regexp.match_test":
		code = a + ".match_test(" + args[0] + ")"
	case "regexp.test":
		code = a + ".test(" + args[0] + ")"
	case "regexp.source":
		code = a + ".Source"
	case "regexp.toString":
		code = a + ".toString()"
	case "string.trim":
		code = a + ".trim()"
	case "string.toLowerCase":
		code = a + ".lower()"
	case "string.toUpperCase":
		code = a + ".upper()"
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
	case "object.classOf":
		code = "classOf(" + a + ")"
	case "classvalue.new":
		code = "castRef[" + b.e.typ(x.Type) + "](" + a + ".Factory())"
	case "classvalue.name":
		code = a + ".Name"
	case "classvalue.has":
		code = a + ".has(" + args[0] + ")"
	case "dynamic.asClassValue":
		code = "dynClass(" + a + ")"
	case "dynamic.materialize":
		code = b.e.materializer(x.Type) + "(" + a + ")"
	case "clock.telemetry":
		code = "telemetry()"
	case "xml.parseSubset":
		code = "parseXML(" + a + ")"
	case "json.parseSubset":
		code = "parseJSON(" + a + ")"
	case "dynamic.null":
		code = "dynNull()"
	case "dynamic.strictEquals":
		code = "dynEqual(" + a + "," + args[0] + ")"
	case "dynamic.isNullish":
		code = a + "==nil || " + a + ".Tag==tagNull"
	case "dynamic.get", "dynamic.put":
		code = a + "." + strings.Split(x.Op, ".")[1] + "(" + strings.Join(args, ",") + ")"
	case "dynamic.asBoolean":
		code = "dynBoolean(" + a + ")"
	case "dynamic.asNumber":
		code = "dynNumber(" + a + ")"
	case "dynamic.isNumber":
		code = a + "!=nil && " + a + ".Tag>=4 && " + a + ".Tag<=6"
	case "dynamic.isArray":
		code = "isArray(" + a + ")"
	case "dynamic.typeof":
		code = "dynTypeof(" + a + ")"
	case "dynamic.toString":
		code = "dynToString(" + a + ")"
	case "dynamic.of":
		code = "box(" + a + ")"
		if x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			code = "func() *dynamic {if !" + a + ".Has {return nil};return box(" + a + ".Value)}()"
		}
	case "dynamic.isString":
		code = a + "!=nil && " + a + ".Tag==1"
	case "dynamic.isFunction":
		code = a + "!=nil && " + a + ".Tag==10"
	case "dynamic.asString":
		code = "dynString(" + a + ")"
	case "dynamic.asRef":
		code = "castRef[" + b.e.typ(x.Type) + "](dynRef(" + a + "))"
		if x.Type.Kind == hir.OrderedMap && x.Type.Args[0].Kind == hir.String {
			code = "dynMap[" + b.e.typ(x.Type.Args[1]) + "](" + a + ")"
		}
	}
	if (x.Op == "array.get" || x.Op == "array.pop" || x.Op == "array.shift" || x.Op == "map.get") && x.Type.Args[0].IsRef() {
		code += ".Value"
		if x.X.Type.Kind == hir.Array && x.Type.Args[0].IsRef() {
			code = "castRef[" + b.e.typ(x.Type) + "](" + code + ")"
		}
	}
	if (x.Op == "array.get" || x.Op == "array.shift" || x.Op == "array.pop") && x.X.Type.Args[0].Kind == hir.Optional && !x.Type.Args[0].IsRef() {
		code += ".Value"
	}
	if (x.Op == "map.keys" || x.Op == "map.values" || x.Op == "set.values") && x.Type.Args[0].IsRef() {
		code = "referenceArray(" + code + ")"
	}
	if x.Op == "set.fromArray" && x.X.Type.Args[0].IsRef() {
		code = "referenceSetFromArray(" + a + "," + args[0] + ")"
	}
	if x.Op == "set.has" && x.X.Kind == hir.StaticGet {
		if classifier := b.e.classifiers[staticKey(x.X)]; classifier != "" {
			code = classifier + "(" + a + "," + args[0] + ")"
			if b.e.characterInteger(x.Args[0], b.m, map[*hir.Expr]bool{}, map[*hir.Method]bool{}) {
				code = classifier + "_integer(" + a + ",int32(" + args[0] + "))"
			}
		}
	}
	return b.temp(x.Type, code)
}
