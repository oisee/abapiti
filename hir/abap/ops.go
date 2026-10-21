package abap

import (
	"sort"

	"github.com/oisee/abapiti/hir"
)

var supportedOps = map[string]bool{
	"map.insertions":   true,
	"host.getenv":      true,
	"host.stderrWrite": true,

	"dynamic.isMap":  true,
	"dynamic.isSet":  true,
	"dynamic.values": true,

	"array.spread0":       true,
	"collection.has":      true,
	"collection.get":      true,
	"set.iterationValues": true,

	"regexp.clone": true, "regexp.flags": true,
	"string.padEnd":     true,
	"string.relativeAt": true,
	"string.codePoints": true,
	"string.sha256Hex":  true,
	"path.basename":     true,

	"number.isInteger":     true,
	"number.isSafeInteger": true,
	"number.isFinite":      true,
	"number.abs":           true,
	"number.hex16":         true,

	"map.checkedGet": true, "map.checkedSet": true, "map.checkedValues": true,
	"string.splitRegex": true, "string.padStart": true, "string.parseFloatNumber": true, "string.toNumber": true,
	"string.sha1Hex": true, "xml.parseWithErrors": true,
	"json5.parse": true,
	"chai.box":    true, "chai.deepEqual": true, "chai.deepInclude": true,
	"chai.arrayInclude": true, "chai.members": true, "chai.throwMatch": true,
	"dynamic.typeofValue":    true,
	"dynamic.call0":          true,
	"dynamic.delete":         true,
	"lsp.isTextDocumentEdit": true, "lsp.isRenameFile": true, "lsp.isCreateFile": true, "lsp.isDeleteFile": true,
	"dynamic.recordString":  true,
	"string.parseIntNumber": true, "number.isNaN": true,
	"json.stringifyConfig": true,
	"json.stringifySubset": true,
	"json.indent":          true,
	"dynamic.keys":         true,
	"dynamic.classView":    true,
	"dynamic.mapView":      true, "map.markJS": true,
	"dynamic.asObject":  true,
	"dynamic.arrayView": true,
	"object.get":        true, "object.hasOwn": true, "object.has": true, "xml.validate": true,
	"regexp.exec": true, "regexp.boxMatch": true, "regexp.lastIndex": true, "regexp.setLastIndex": true,
	"string.repeat": true, "string.match": true, "array.matchIndex": true, "array.matchGroups": true,
	"string.replaceRegex":     true,
	"string.toLowerCase":      true,
	"object.checkedClassView": true,
	"object.classOf":          true, "classvalue.newArgs": true, "classvalue.new": true, "classvalue.name": true, "classvalue.has": true, "dynamic.asClassValue": true,
	"clock.telemetry": true, "xml.parseSubset": true, "dynamic.materialize": true, "json.parseSubset": true,
	"dynamic.null": true, "dynamic.strictEquals": true, "dynamic.isNullish": true, "dynamic.get": true, "dynamic.put": true, "dynamic.asBoolean": true, "dynamic.asNumber": true, "dynamic.isNumber": true, "dynamic.isArray": true, "dynamic.typeof": true, "dynamic.toString": true,

	"string.startsWith": true, "string.endsWith": true, "string.indexOf": true, "string.at": true, "string.replaceFirst": true, "string.repeatIndent": true, "string.parseInt10": true, "string.parseInt10i64": true, "string.compareRegistryKey": true, "string.compareObjectName": true, "string.localeCompareNames": true, "regexp.new": true, "regexp.test": true, "regexp.match_test": true, "regexp.source": true, "regexp.toString": true,

	"array.reverse": true, "array.unshift": true, "array.concat": true, "array.slice0": true, "array.slice1": true, "array.slice2": true, "array.splice1": true, "array.splice2": true, "array.splice3": true, "array.splice1_view": true, "array.pop": true, "array.shift": true, "array.indexOf": true, "array.includes": true, "array.join": true, "map.values": true, "record.delete": true, "set.delete": true, "set.clear": true, "map.clear": true, "set.copy": true,

	"string.substr": true, "string.trim": true, "string.slice": true, "string.charAt": true, "string.replaceAll": true, "string.split": true, "string.toUpperCase": true, "set.fromArray": true,
	"array.push": true, "array.get": true, "array.has": true, "array.delete": true, "array.length": true,
	"map.delete": true, "string.trimStart": true, "string.trimEnd": true,
	"map.set": true, "map.get": true, "map.has": true, "map.size": true, "map.keys": true,
	"set.add": true, "set.has": true, "set.size": true, "set.values": true,
	"string.length": true, "string.concat": true, "string.substring": true, "string.charCodeAt": true,
	"i32.toString": true, "i64.toString": true, "i64.remainder2": true,
	"number.ceil": true, "number.remainder2": true, "number.index": true, "number.fromI32": true, "number.toString": true,
	"dynamic.of": true, "dynamic.isString": true, "dynamic.isFunction": true, "dynamic.asString": true, "dynamic.asRef": true,

	// Structural families: boxed live records (union.go). Payload projection
	// is refused per site, see SupportsRuntimeExpr.
	"union.declarations": true, "union.check": true, "union.admit": true, "union.tag": true, "union.widen": true, "union.project": true,
}

// Operations without a reviewed ABAP implementation retain explicit refusals.
var unsupportedOps = map[string]bool{
	"container.readonlyClassView": true, "container.readonlyView": true,
	"map.view": true, "set.view": true, "arrayLike.length": true,
	"number.strictIsNaN": true, "regexp.matchAllNext": true,
	"string.parseFloat": true,

	// Common family reads need guarded payload views.
	"union.read": true,
	// Spreading a tagged iterable (Set | array) needs a dynamic iteration ABI.
	"dynamic.spread": true,
	// Number(boxed) needs the dynamic ToPrimitive ABI; ABAP refuses at the call.
	"dynamic.toNumber": true,
	// JSON.stringify replacer callbacks need a per-property callback ABI.
	"json.stringifyReplacer": true,
}

// SupportsRuntimeOp is false for unreviewed or newly introduced operations.
func SupportsRuntimeOp(op string) bool { return supportedOps[op] }

// SupportsRuntimeExpr refines SupportsRuntimeOp for one site: a union
// projection to a payload class still has no ABAP emitter.
func SupportsRuntimeExpr(x *hir.Expr) bool {
	if x.Op == "union.project" && x.Type.Kind != hir.UnionRef {
		return false
	}
	return SupportsRuntimeOp(x.Op)
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
