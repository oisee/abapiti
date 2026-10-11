package abap

import (
	"sort"

	"github.com/oisee/abapiti/hir"
)

var supportedOps = map[string]bool{
	"string.repeat": true, "string.match": true, "array.matchIndex": true,
	"string.replaceRegex": true,
	"string.toLowerCase":  true,
	"object.classOf":      true, "classvalue.new": true, "classvalue.name": true, "classvalue.has": true, "dynamic.asClassValue": true,
	"clock.telemetry": true, "xml.parseSubset": true, "dynamic.materialize": true, "json.parseSubset": true,
	"dynamic.null": true, "dynamic.strictEquals": true, "dynamic.isNullish": true, "dynamic.get": true, "dynamic.put": true, "dynamic.asBoolean": true, "dynamic.asNumber": true, "dynamic.isNumber": true, "dynamic.isArray": true, "dynamic.typeof": true, "dynamic.toString": true,

	"string.startsWith": true, "string.endsWith": true, "string.indexOf": true, "string.at": true, "string.replaceFirst": true, "string.repeatIndent": true, "string.parseInt10": true, "string.parseInt10i64": true, "string.compareRegistryKey": true, "string.compareObjectName": true, "string.localeCompareNames": true, "regexp.new": true, "regexp.test": true, "regexp.match_test": true, "regexp.source": true, "regexp.toString": true,

	"array.reverse": true, "array.unshift": true, "array.concat": true, "array.slice0": true, "array.slice1": true, "array.slice2": true, "array.splice1": true, "array.splice2": true, "array.splice3": true, "array.splice1_view": true, "array.pop": true, "array.shift": true, "array.indexOf": true, "array.includes": true, "array.join": true, "map.values": true, "record.delete": true, "set.delete": true, "set.copy": true,

	"string.substr": true, "string.trim": true, "string.slice": true, "string.charAt": true, "string.replaceAll": true, "string.split": true, "string.toUpperCase": true, "set.fromArray": true,
	"array.push": true, "array.get": true, "array.length": true,
	"map.delete": true, "string.trimStart": true, "string.trimEnd": true,
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
