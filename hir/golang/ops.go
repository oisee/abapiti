package golang

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

var supportedOps = map[string]bool{
	"union.declarations": true,
	"map.view":           true, "union.check": true, "union.admit": true,
	"union.widen":            true,
	"union.project":          true,
	"union.tag":              true,
	"union.read":             true,
	"dynamic.typeofValue":    true,
	"dynamic.recordString":   true,
	"dynamic.call0":          true,
	"lsp.isTextDocumentEdit": true, "lsp.isRenameFile": true, "lsp.isCreateFile": true, "lsp.isDeleteFile": true,
	"chai.members": true, "chai.deepEqual": true, "chai.deepInclude": true, "chai.box": true, "chai.arrayInclude": true, "chai.throwMatch": true,
	"string.sha1Hex":          true,
	"string.parseFloatNumber": true, "string.parseIntNumber": true, "string.toNumber": true, "dynamic.toNumber": true, "number.isNaN": true,
	"string.repeat": true, "string.padStart": true, "string.padEnd": true, "string.relativeAt": true, "regexp.clone": true, "regexp.flags": true, "regexp.matchAllNext": true, "array.matchGroups": true, "string.parseFloat": true, "string.match": true, "array.matchIndex": true,
	"json.stringifySubset": true,
	"json.stringifyConfig": true,
	"string.replaceRegex":  true,
	"string.toLowerCase":   true,
	"object.get":           true, "object.hasOwn": true, "object.has": true,
	"object.checkedClassView": true,
	"dynamic.keys":            true,
	"dynamic.spread":          true,
	"dynamic.isMap":           true,
	"dynamic.isSet":           true,
	"map.markJS":              true,
	"json.stringifyReplacer":  true,
	"dynamic.classView":       true,
	"dynamic.arrayView":       true,
	"object.classOf":          true, "classvalue.newArgs": true, "classvalue.new": true, "classvalue.name": true, "classvalue.has": true, "dynamic.asClassValue": true,
	"xml.validate":    true,
	"clock.telemetry": true, "xml.parseSubset": true, "xml.parseWithErrors": true, "dynamic.materialize": true, "json.parseSubset": true, "json5.parse": true, "json.indent": true,
	"dynamic.null": true, "dynamic.strictEquals": true, "dynamic.isNullish": true, "dynamic.get": true, "dynamic.put": true, "dynamic.delete": true, "dynamic.asBoolean": true, "dynamic.asNumber": true, "dynamic.isNumber": true, "dynamic.isArray": true, "dynamic.typeof": true, "dynamic.toString": true,

	"string.startsWith": true, "string.endsWith": true, "string.indexOf": true, "string.at": true, "string.replaceFirst": true, "string.repeatIndent": true, "string.parseInt10": true, "string.parseInt10i64": true, "string.compareRegistryKey": true, "string.compareObjectName": true, "string.localeCompareNames": true, "regexp.new": true, "regexp.lastIndex": true, "regexp.setLastIndex": true, "regexp.exec": true, "regexp.test": true, "regexp.match_test": true, "regexp.source": true, "regexp.toString": true,

	"array.spread0": true, "array.reverse": true, "array.unshift": true, "array.concat": true, "array.slice0": true, "array.slice1": true, "array.slice2": true, "array.splice1": true, "array.splice2": true, "array.spliceDrop2": true, "array.splice3": true, "array.splice1_view": true, "array.pop": true, "array.shift": true, "array.indexOf": true, "array.includes": true, "array.join": true, "map.values": true, "record.delete": true, "set.delete": true, "set.clear": true, "map.clear": true, "set.copy": true,

	"string.substr": true, "string.trim": true, "string.slice": true, "string.charAt": true, "string.replaceAll": true, "string.split": true, "string.splitRegex": true, "string.toUpperCase": true, "set.fromArray": true,
	"array.push": true, "array.get": true, "array.has": true, "array.delete": true, "array.length": true,
	"map.delete": true, "string.trimStart": true, "string.trimEnd": true,
	"map.set": true, "map.get": true, "map.has": true, "map.size": true, "map.keys": true,
	"set.iterationValues": true, "set.add": true, "set.has": true, "set.size": true, "set.values": true,
	"string.length": true, "string.concat": true, "string.substring": true, "string.charCodeAt": true,
	"i32.toString": true, "i64.toString": true, "i64.remainder2": true,
	"number.ceil": true, "number.remainder2": true, "number.index": true, "number.fromI32": true, "number.toString": true,
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
	if x.Op == "map.set" && x.X.Type.Args[0].Kind == hir.Dynamic {
		b.e.unsupported(x.Node, "boxed Map writes require value-based SameValueZero")
		return ""
	}
	if x.Op == "map.view" {
		return b.temp(x.Type, b.mapView(x))
	}
	if code, ok := b.builtinRuntime(x); ok {
		return code
	}
	if code, ok := b.bigintRuntime(x); ok {
		return code
	}
	if code, ok := b.gogenRuntime(x); ok {
		return code
	}
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
	switch x.Op {
	case "array.indexOf", "array.includes":
		if x.X.Type.Kind == hir.Array && x.X.Type.Args[0].Kind == hir.Dynamic {
			// any[] search compares boxed values: === for indexOf, SameValueZero for includes.
			b.e.dynamicSearch = true
			code = "dynArray" + strings.ToUpper(x.Op[6:7]) + x.Op[7:] + "(" + a + "," + strings.Join(args, ",") + ")"
			break
		}
		code = a + "." + strings.Split(x.Op, ".")[1] + "(" + strings.Join(args, ",") + ")"
	case "array.spread0":
		length := "int32(len(" + a + ".Items))"
		if b.e.liveStorage() {
			length = a + ".length()"
		}
		helper := "spreadArray"
		if x.X.Type.Args[0].Kind != hir.Optional && !x.X.Type.Args[0].IsRef() {
			helper = "spreadOptionalArray"
		}
		code = helper + "(" + a + "," + length + ")"

	case "array.has", "array.delete", "array.reverse", "array.unshift", "array.concat", "array.slice0", "array.slice1", "array.slice2", "array.splice1", "array.splice2", "array.spliceDrop2", "array.splice3", "array.splice1_view", "array.pop", "array.shift", "array.join", "map.values", "set.delete", "set.copy", "array.push", "array.get", "map.set", "map.get", "map.has", "map.delete", "map.keys", "set.add", "set.has", "set.values", "set.fromArray":
		code = a + "." + strings.Split(x.Op, ".")[1] + "(" + strings.Join(args, ",") + ")"
	case "string.parseFloatNumber":
		code = "parseFloatNumber(" + a + ")"
	case "string.toNumber":
		code = "stringToNumber(" + a + ")"
	case "dynamic.toNumber":
		code = "dynToNumber(" + a + ")"
	case "string.parseIntNumber":
		code = "parseIntNumber(" + a + ")"
	case "number.isNaN":
		code = "numberIsNaN(" + a + ")"
	case "string.parseFloat":
		code = a + ".parseFloat()"
	case "string.padStart", "string.padEnd", "string.relativeAt", "regexp.matchAllNext":
		code = a + "." + strings.Split(x.Op, ".")[1] + "(" + strings.Join(args, ",") + ")"
	case "regexp.clone":
		code = "cloneRegexp(" + a + ")"
	case "regexp.flags":
		code = a + ".Flags"
	case "array.matchGroups":
		code = a + ".MatchGroups"
	case "string.repeat", "string.match":
		code = a + "." + strings.Split(x.Op, ".")[1] + "(" + args[0] + ")"
	case "array.matchIndex":
		code = a + ".matchIndex()"
	case "dynamic.delete":
		code = a + ".deleteProperty(" + args[0] + ")"
	case "record.delete":
		code = "func() bool {" + a + ".delete(" + args[0] + ");return true}()"
	case "set.iterationValues":
		code = a + ".iterationValues()"
	case "set.clear":
		// Deleting from the end keeps every set representation's index and
		// side tables consistent without moving the remaining items.
		code = "func() {s:=" + a + ";for len(s.Items)>0 {s.delete(s.Items[len(s.Items)-1])}}()"
	case "map.clear":
		// Keys go through delete (proxy-aware for Map views), last first so
		// no remaining entry moves.
		code = "func() {m:=" + a + ";k:=m.keys().Items;for i:=len(k)-1;i>=0;i-- {m.delete(k[i])}}()"
	case "array.length", "set.size":
		code = "int32(len(" + a + ".Items))"
		if x.Op == "array.length" && b.e.liveStorage() {
			code = a + ".length()"
		}
	case "map.size":
		code = "int32(len(" + a + ".Entries))"
		if b.e.usesMapViews() {
			code = a + ".mapSize()"
		}
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
	case "string.splitRegex":
		b.e.regexSplit = true
		code = a + ".splitRegex(" + strings.Join(args, ",") + ")"
	case "string.replaceRegex":
		code = a + ".replaceRegex(" + strings.Join(args, ",") + ")"
	case "regexp.match_test":
		code = a + ".match_test(" + args[0] + ")"
	case "regexp.lastIndex":
		code = a + ".LastIndex"
	case "regexp.setLastIndex":
		code = a + ".setLastIndex(" + args[0] + ")"
	case "regexp.exec":
		code = a + ".exec(" + args[0] + ")"
	case "regexp.test":
		code = a + ".test(" + args[0] + ")"
	case "regexp.source":
		code = a + ".Source"
	case "regexp.toString":
		code = a + ".toString()"
	case "string.trim", "string.trimStart", "string.trimEnd":
		code = a + "." + strings.TrimPrefix(x.Op, "string.") + "()"
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
	case "number.ceil":
		code = "numberCeil(" + a + ")"
	case "number.remainder2":
		code = "numberRemainder2(" + a + ")"
	case "number.index":
		code = "numberIndex(" + a + ")"
	case "number.fromI32":
		code = "float64(" + a + ")"
	case "number.toString":
		code = "numberString(" + a + ")"
	case "object.get", "object.hasOwn", "object.has":
		lookup := b.e.propertyLookup(x.X.Type)
		code = lookup + "(" + a + "," + args[0] + "," + fmt.Sprint(x.Op == "object.get") + ")"
		if x.Op == "object.get" {
			code += ".value"
		} else if x.Op == "object.hasOwn" {
			code += ".own"
		} else {
			code += ".exists"
		}
	case "object.classOf":
		code = "nativeConstructor(" + a + ")"
	case "classvalue.newArgs":
		code = "castRef[" + b.e.typ(x.Type) + "](" + a + ".ArgsFactory(" + args[0] + "))"
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
	case "chai.members":
		code = "chaiMembers(" + a + "," + strings.Join(args, ",") + ")"
	case "xml.validate":
		b.e.xmlValidation = true
		code = "validateXML(" + a + ")"
	case "xml.parseSubset":
		code = "parseXML(" + a + ")"
	case "xml.parseWithErrors":
		c := b.e.classBy("builtin.Error")
		if c == nil {
			code = "func() *dynamic {panic(newTrap(\"xml parser Error representation is unsupported\"))}()"
			break
		}
		b.e.usesTypedCatch = true
		errType := hir.Ref(c.Name)
		code = "parseXMLWithErrors(" + a + ",func(message jsString){panic(payload[" + b.e.typ(errType) + "]{Type:" + strconv.Quote(errType.String()) + ",Value:" + b.e.name("new."+c.Name) + "(present(message))})})"
	case "json.stringifySubset":
		code = "stringifyConfig(" + a + ")"
	case "json.stringifyConfig":
		code = "stringifyConfigChecked(" + a + "," + strings.Join(args, ",") + ")"
	case "json.indent":
		code = "indentJSON(" + a + "," + args[0] + ")"
	case "json.parseSubset":
		code = "parseJSON(" + a + ")"
		if c := b.e.classBy("builtin.SyntaxError"); c != nil {
			b.e.usesTypedCatch = true
			errType := hir.Ref(c.Name)
			code = "parseJSONWithErrors(" + a + ",func(message jsString){panic(payload[" + b.e.typ(errType) + "]{Type:" + strconv.Quote(errType.String()) + ",Value:" + b.e.name("new."+c.Name) + "(present(message))})})"
		}
	case "json5.parse":
		code = "parseJSON5(" + a + ",func(message jsString){panic(newTrap(\"JSON5 SyntaxError payload ABI is unsupported: \"+message.String()))})"
	case "string.sha1Hex":
		code = "sha1Hex(" + a + ")"
	case "dynamic.null":
		code = "dynNull()"
	case "chai.throwMatch":
		b.e.usesTypedCatch = true
		call := a + "." + b.e.member("call") + "()"
		if method, _ := b.e.method(b.e.classBy(x.X.Type.Name), "call"); method != nil && method.Result.Kind == hir.Promise {
			call = "drainSettled(" + call + ")"
		} else {
			for _, iface := range b.e.p.Interfaces {
				if iface.Name == x.X.Type.Name {
					for _, m := range iface.Methods {
						if m.Name == "call" && m.Result.Kind == hir.Promise {
							call = "drainSettled(" + call + ")"
						}
					}
				}
			}
		}
		if x.X.Type.Kind == hir.Dynamic {
			code = "chaiThrowMatch(" + b.e.chaiAction() + "(" + a + ")," + args[0] + ")"
		} else {
			code = "chaiThrowMatch(func(){" + call + "}," + args[0] + ")"
		}
	case "chai.deepEqual", "chai.deepInclude", "chai.arrayInclude":
		code = strings.ReplaceAll(x.Op, ".", "") + "(" + a + "," + args[0] + ")"
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
	case "dynamic.typeofValue":
		code = b.e.dynamicTypeof() + "(" + a + ")"
	case "dynamic.recordString":
		b.e.recordString = true
		code = "recordString(" + a + ")"
	case "dynamic.call0":
		code = b.nativeCall0(x, a)
	case "dynamic.toString":
		code = "dynToString(" + a + ")"
	case "dynamic.of", "chai.box":
		boxer := "box"
		if x.Op == "chai.box" {
			boxer = "chaiBox"
		}
		code = boxer + "(" + a + ")"
		if x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			code = "func() *dynamic {if !" + a + ".Has {return nil};return " + boxer + "(" + a + ".Value)}()"
		}
	case "lsp.isTextDocumentEdit", "lsp.isRenameFile", "lsp.isCreateFile", "lsp.isDeleteFile":
		if !b.e.lspGuards {
			b.e.extra.WriteString(lspGuardsSource)
			b.e.lspGuards = true
		}
		helper := "lspTextDocumentEdit"
		tail := ""
		if x.Op != "lsp.isTextDocumentEdit" {
			helper = "lspRenameFile"
			tail = "," + args[0]
			if x.Op == "lsp.isCreateFile" {
				helper = "lspResourceFile"
				tail = ",str(\"create\"),str(\"overwrite\"),str(\"ignoreIfExists\")," + args[0]
			}
			if x.Op == "lsp.isDeleteFile" {
				helper = "lspResourceFile"
				tail = ",str(\"delete\"),str(\"recursive\"),str(\"ignoreIfNotExists\")," + args[0]
			}
		}
		code = helper + "(" + a + tail + ")"
	case "dynamic.isString":
		code = a + "!=nil && " + a + ".Tag==1"
	case "dynamic.isFunction":
		code = a + "!=nil && " + a + ".Tag==10"
	case "dynamic.asString":
		code = "dynString(" + a + ")"
	case "object.checkedClassView":
		code = "castRef[" + b.e.typ(x.Type) + "](" + a + ")"
	case "json.stringifyReplacer":
		fn := "nil"
		if x.Args[0].Kind != hir.Lit {
			var m *hir.Method
			for _, in := range b.e.p.Interfaces {
				if in.Name == x.Args[0].Type.Name {
					m = in.Methods[0]
				}
			}
			fn = "func(k jsString, v *dynamic) *dynamic {return (" + args[0] + ")." + b.e.member(m.Name) + "(k,v)}"
		}
		code = "stringifyReplacer(" + a + "," + fn + "," + args[1] + "," + args[2] + ")"
	case "map.markJS":
		code = "markJSMap(" + a + ")"
		if b.e.directCollectionKey(x.X.Type.Args[0]) {
			code = "markJSFastMap(" + a + ")"
		}
		b.e.collectionProbe = true
	case "dynamic.isMap", "dynamic.isSet":
		kind := "orderedMap["
		if x.Op == "dynamic.isSet" {
			kind = "orderedSet["
		}
		code = "dynIsCollection(" + a + "," + strconv.Quote(kind) + ")"
		b.e.collectionProbe = true
	case "dynamic.spread":
		code = "dynSpread(" + a + ")"
		b.e.spreadRuntime = true
	case "dynamic.keys":
		code = "dynamicKeys(" + a + ")"
		b.e.extraRuntimeKeys = true
	case "union.check", "union.admit", "union.widen", "union.project", "union.tag", "union.read":
		code = b.unionOperation(x, a)
	case "dynamic.classView":
		code = b.e.liveView(x.Type) + "(" + a + ")"
	case "dynamic.arrayView":
		code = b.e.viewValue(x.Type, a)
	case "dynamic.asRef":
		code = "castRef[" + b.e.typ(x.Type) + "](dynRef(" + a + "))"
		if x.Type.Kind == hir.OrderedMap && x.Type.Args[0].Kind == hir.String {
			code = "dynMap[" + b.e.typ(x.Type.Args[1]) + "](" + a + ")"
		}
	}
	if (x.Op == "array.get" || x.Op == "array.pop" || x.Op == "array.shift" || x.Op == "map.get") && x.Type.Args[0].IsRef() {
		code += ".Value"
		if x.X.Type.Kind == hir.Array && x.Type.Args[0].IsRef() {
			if x.Type.Args[0].Kind == hir.Dynamic {
				// Reference-array storage is erased. A view as any[] boxes
				// the actual element, rather than requiring a pre-existing box.
				code = "box(" + code + ")"
			} else {
				code = b.e.arrayReference(x.Type, code)
			}
		}
	}
	if (x.Op == "array.get" || x.Op == "array.shift" || x.Op == "array.pop") && x.X.Type.Args[0].Kind == hir.Optional && !x.Type.Args[0].IsRef() {
		code += ".Value"
	}
	if x.Op == "map.get" && x.X.Type.Args[1].Kind == hir.Optional && !x.Type.Args[0].IsRef() {
		// The outer lookup box carries membership; JS get exposes its payload.
		code += ".Value"
	}
	if (x.Op == "map.keys" || x.Op == "map.values" || x.Op == "set.values" || x.Op == "set.iterationValues") && x.Type.Args[0].IsRef() {
		code = "referenceArray(" + code + ")"
	}
	if x.Op == "set.fromArray" && x.X.Type.Args[0].IsRef() {
		helper := "referenceSetFromArray"
		if b.e.directCollectionKey(x.X.Type.Args[0]) {
			helper = "referenceFastSetFromArray"
		}
		code = helper + "(" + a + "," + args[0] + ")"
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

// SupportsRuntimeOp reports explicit Go emitter coverage.
func SupportsRuntimeOp(op string) bool { return supportedOps[op] }

// dynSpread is [...v] over a tagged iterable: a string by code point, an
// array or Set by element. Any other value raises (JavaScript: TypeError).
const dynSpreadSource = `
func dynSpread(d *dynamic) *array[any] {
 if d==nil {panic(newRangeFault())}
 out:=&array[any]{}
 if d.Tag==1 {
  s:=d.Value.(jsString);n:=s.length()
  for i:=int32(0);i<n; {
   w:=int32(1);c:=s.charCodeAt(i)
   if c>=0xD800 && c<=0xDBFF && i+1<n {if m:=s.charCodeAt(i+1);m>=0xDC00 && m<=0xDFFF {w=2}}
   out.Items=append(out.Items,box(s[2*i:2*(i+w)]));i+=w
  }
  return out
 }
 if d.Tag==2 || d.Tag==tagArray {
  if rv:=reflect.ValueOf(d.Value);rv.Kind()==reflect.Pointer && !rv.IsNil() && rv.Elem().Kind()==reflect.Struct {
   e:=rv.Elem();name:=collectionTypeName(e.Type())
   if strings.HasPrefix(name,"orderedSet[") || strings.HasPrefix(name,"array[") {
    items,missing:=e.FieldByName("Items"),e.FieldByName("Missing")
    for i:=0;i<items.Len();i++ {
     if missing.IsValid() && missing.Len()>0 && missing.MapIndex(reflect.ValueOf(int32(i))).IsValid() {out.Items=append(out.Items,nil);continue}
     out.Items=append(out.Items,box(items.Index(i).Interface()))
    }
    return out
   }
  }
 }
 panic(newRangeFault())
}
`

// collectionTypeNameSource maps the concrete-key fast collections onto the
// orderedMap/orderedSet names the reflective probes recognise.
const collectionTypeNameSource = `
func collectionTypeName(t reflect.Type) string {
 name:=t.Name();if rest,ok:=strings.CutPrefix(name,"fastO");ok {return "o"+rest};return name
}
`

// dynIsCollection is instanceof Map / Set on a tagged value: the exact Go
// collection type, and for a Map the jsMap marker that only the Map
// constructor sets (records share the ordered-map storage).
const dynIsCollectionSource = `
func markJSMap[K comparable, V any](m *orderedMap[K, V]) *orderedMap[K, V] {if m!=nil {m.jsMap=true};return m}
func markJSFastMap[K comparable, V any](m *fastOrderedMap[K, V]) *fastOrderedMap[K, V] {if m!=nil {m.jsMap=true};return m}
func dynIsCollection(d *dynamic, kind string) bool {
 if d==nil || d.Tag!=2 {return false}
 rv:=reflect.ValueOf(d.Value)
 if rv.Kind()!=reflect.Pointer || rv.IsNil() || rv.Elem().Kind()!=reflect.Struct || !strings.HasPrefix(collectionTypeName(rv.Elem().Type()),kind) {return false}
 if kind=="orderedMap[" {f:=rv.Elem().FieldByName("jsMap");return f.IsValid() && f.Bool()}
 return true
}
`

// configJSONReplacerPatches extend the usage-gated JSON writer with
// JSON.stringify's replacer function and key list (SerializeJSONProperty /
// SerializeJSONObject), only when a program uses them.
var configJSONReplacerPatches = [][2]string{
	{"type configJSONWriter struct { active map[any]bool }", "type configJSONWriter struct { active map[any]bool; replacer func(jsString,*dynamic)*dynamic; keys []jsString; hasKeys bool }"},
	{`out:=str("{");comma:=false;for _,e:=range append(indexed,rest...) {s,ok:=w.value(e.Value);`, `all:=append(indexed,rest...);if w.hasKeys {byKey:=map[jsString]any{};present:=map[jsString]bool{};for _,e:=range all {byKey[e.Key]=e.Value;present[e.Key]=true};all=nil;for _,k:=range w.keys {if present[k] {all=append(all,entry[jsString,any]{k,byKey[k]})}}};out:=str("{");comma:=false;for _,e:=range all {s,ok:=w.prop(e.Key,e.Value);`},
	{`s,ok:=w.value(a.get(int32(i)));if !ok {s=str("null")}`, `s,ok:=w.prop(str(strconv.Itoa(i)),a.get(int32(i)));if !ok {s=str("null")}`},
}

const configJSONReplacerSource = `
func jsonBoxed(v any) *dynamic {if o,ok:=v.(interface{configJSONOptional()(any,bool)});ok {value,has:=o.configJSONOptional();if !has {return nil};return jsonBoxed(value)};return box(v)}
func (w *configJSONWriter) prop(k jsString, v any) (jsString,bool) {if w.replacer==nil {return w.value(v)};return w.value(w.replacer(k,jsonBoxed(v)))}
func stringifyReplacer(v any, fn func(jsString,*dynamic)*dynamic, keys *array[jsString], gap jsString) optional[jsString] {
 w:=&configJSONWriter{active:map[any]bool{},replacer:fn}
 if keys!=nil && fn==nil {seen:=map[jsString]bool{};for _,k:=range keys.Items {if !seen[k] {seen[k]=true;w.keys=append(w.keys,k)}};w.hasKeys=true}
 s,ok:=w.prop(str(""),v)
 if !ok {return optional[jsString]{}}
 if gap!=str("") {s=indentJSON(s,gap)}
 return present(s)
}
`
