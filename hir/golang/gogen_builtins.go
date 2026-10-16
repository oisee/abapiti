package golang

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
)

func init() {
	supportedOps["number.hex16"] = true
	for _, op := range []string{"number.isInteger", "number.isSafeInteger", "number.isFinite", "number.strictIsNaN", "number.abs", "host.getenv", "string.codePoints", "dynamic.values", "arrayLike.length"} {
		supportedOps[op] = true
	}
}

func (b *body) builtinRuntime(x *hir.Expr) (string, bool) {
	switch x.Op {
	case "number.hex16":
		b.e.hirStrconv = true
		return "func(n float64) jsString {if n==0 {return str(\"0\")};if n!=float64(int64(n)) || n < -9007199254740991 || n > 9007199254740991 {panic(newTrap(" + fmt.Sprintf("%q", x.Node.Source+": base-16 formatting requires a safe integer") + "))};return str(strconv.FormatInt(int64(n),16))}(" + b.expr(x.X) + ")", true
	case "arrayLike.length":
		b.e.hirMath = true
		return "func(d *dynamic) float64 {if d==nil || d.Tag==tagNull {panic(newRangeFault())};n:=dynToNumber(d.get(str(\"length\")));if math.IsNaN(n) || n<=0 {return 0};return math.Min(math.Floor(n),9007199254740991)}(" + b.expr(x.X) + ")", true
	case "host.getenv":
		if b.e.externImports == nil {
			b.e.externImports = map[string]string{}
		}
		alias := b.e.externImports["os"]
		if alias == "" {
			alias = fmt.Sprintf("externPkg%d", len(b.e.externImports))
			b.e.externImports["os"] = alias
		}
		name := fmt.Sprintf("hostGetenv%d", len(b.e.externAdapters))
		b.e.externAdapters = append(b.e.externAdapters, "func "+name+"(key jsString) optional[jsString] {v,ok:="+alias+".LookupEnv(key.String());if !ok{return optional[jsString]{}};return present(str(v))}")
		return name + "(" + b.expr(x.X) + ")", true
	case "number.abs":
		b.e.hirMath = true
		return "math.Abs(" + b.expr(x.X) + ")", true
	case "number.isInteger", "number.isSafeInteger", "number.isFinite", "number.strictIsNaN":
		b.e.hirMath = true
		test := "!math.IsNaN(n) && !math.IsInf(n,0)"
		switch x.Op {
		case "number.isInteger":
			test += " && math.Trunc(n)==n"
		case "number.isSafeInteger":
			test += " && math.Trunc(n)==n && math.Abs(n)<=9007199254740991"
		case "number.strictIsNaN":
			test = "math.IsNaN(n)"
		}
		return "func(v *dynamic) bool {if v==nil || v.Tag<4 || v.Tag>6{return false};n:=dynNumber(v);return " + test + "}(" + b.expr(x.X) + ")", true
	case "string.codePoints":
		return "func(s jsString) *array[jsString] {a:=&array[jsString]{};for i:=int32(0);i<s.length(); {j:=i+1;c:=s.charCodeAt(i);if c>=0xd800 && c<=0xdbff && j<s.length() {d:=s.charCodeAt(j);if d>=0xdc00 && d<=0xdfff {j++}};a.push(s.substring(i,j));i=j};return a}(" + b.expr(x.X) + ")", true
	case "dynamic.values":
		// Key capture precedes value reads. The runtime's own-key metadata
		// retains insertion order and integer-index ordering for records.
		b.e.extraRuntimeKeys = true
		return "func(d *dynamic) *array[*dynamic] {if d==nil || d.Tag==tagNull {panic(newRangeFault())};a:=&array[*dynamic]{};if d.Tag==1 {s:=d.Value.(jsString);for i:=int32(0);i<s.length();i++ {a.push(box(s.charAt(i)))};return a};if d.Tag>=3 && d.Tag<=6 {return a};if d.Tag==tagArray {v:=d.Value.(*array[any]);for i:=int32(0);i<int32(len(v.Items));i++ {x:=v.get(i);if x.Has {a.push(box(x.Value))}};return a};if d.Tag==2 {if v,ok:=d.Value.(interface{liveLength() int32;liveGet(int32)(*dynamic,bool)});ok {for i:=int32(0);i<v.liveLength();i++ {x,has:=v.liveGet(i);if has {a.push(x)}};return a}};for _,k:=range dynamicKeys(d).Items {a.push(d.get(k))};return a}(" + b.expr(x.X) + ")", true
	}
	return "", false
}
