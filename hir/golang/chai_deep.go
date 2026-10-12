package golang

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// A required-field data shape has an exact enumerable key set. Optional fields
// cannot prove absence versus present undefined; nominal instances may have
// inherited properties/hooks. Never infer either from Go implementation fields.
func (e *emitter) chaiDeepShape(c *hir.Class, data bool) {
	e.line("func (self *%s) chaiEntries() []entry[jsString,*dynamic] {", e.obj(c.Name))
	proven := data
	for _, f := range c.Fields {
		proven = proven && !f.Static && f.Type.Kind != hir.Optional
	}
	if !proven {
		e.line("panic(newTrap(%q)) }", "chai enumerable storage is unproven for "+c.Name)
		return
	}
	e.line("if self==nil {panic(newTrap(\"chai null data reference\"))};return []entry[jsString,*dynamic]{")
	for _, f := range c.Fields {
		e.line("{str(%q),chaiBox(self.%s)},", f.Name, e.member(f.Name))
	}
	e.line("} }")
}

// Only builtin Error storage and its native descendants certify this scalar
// message read. No general catch, enumerable-key, or live-view ABI is changed.
func (e *emitter) chaiThrowShape(c *hir.Class) {
	for parent := c; parent != nil; parent = e.classBy(parent.Super) {
		if parent.Name == "builtin.Error" {
			e.line("func (self *%s) chaiErrorMessage() jsString {if self==nil {panic(newTrap(\"chai null Error throw\"))};return self.%s}", e.obj(c.Name), e.member("message"))
			return
		}
	}
}

// Chai calls a synchronous action and discards its result. Dispatch only the
// compiler's actual closure classes, not arbitrary objects with a call method.
// This harness boundary does not adapt general erased callable/config views.
func (e *emitter) chaiAction() string {
	name := e.name("chai.action")
	if e.chaiActionUsed {
		return name
	}
	e.chaiActionUsed = true
	var b strings.Builder
	b.WriteString("func " + name + "(d *dynamic) func() {\n")
	b.WriteString("if d==nil || d.Tag!=2 {panic(newTrap(\"chai action requires certified parameterless callable storage\"))};switch action:=d.Value.(type) {\n")
	for _, c := range e.p.Classes {
		if !strings.HasPrefix(c.Name, "closure.") {
			continue
		}
		callable := false
		thunk := false
		for _, i := range c.Implements {
			thunk = thunk || i == "closure.thunk"
			callable = callable || i == "closure.thunk" || strings.HasPrefix(i, "closure.function.")
		}
		m, _ := e.method(c, "call")
		if !callable || m == nil || len(m.Params) != 0 || m.Result.Kind == hir.Promise && !thunk {
			continue
		}
		call := "action." + e.member("call") + "()"
		if m.Result.Kind == hir.Promise {
			call = "drainSettled(" + call + ")"
		}
		b.WriteString("case *" + e.obj(c.Name) + ":return func(){" + call + "}\n")
	}
	b.WriteString("default:panic(newTrap(\"chai action requires certified parameterless callable storage\"))}}\n")
	e.extra.WriteString(b.String())
	return name
}

const chaiDeepRuntimeSource = `
// Chai 4 / deep-eql 4: SameValue primitives, positional arrays (holes read as
// undefined), enumerable object keys, and pair memoization for cyclic graphs.
// Unsupported JS types/storage refuse instead of falling back to reflect.DeepEqual
// or JSON text (which loses NaN, signed zero, undefined, identity and key presence).
func chaiBox(v any) *dynamic {
 if nilRef(v) {return nil}
 if s,ok:=v.(interface{dynamicSource() *dynamic});ok && s.dynamicSource()!=nil {panic(newTrap("chai parsed/native snapshot escape is unsupported"))}
 return box(v)
}
type chaiArray interface { chaiLength() int; chaiAt(int) *dynamic; chaiHas(int) bool }
func (a *array[T]) chaiLength() int {return len(a.Items)}
func (a *array[T]) chaiAt(i int) *dynamic {v:=a.get(int32(i));if !v.Has {return nil};return chaiBox(v.Value)}
func (a *array[T]) chaiHas(i int) bool {return a.has(int32(i))}
func chaiKind(d *dynamic) int {
 if d==nil {return 0}
 if d.Tag>=4 && d.Tag<=6 {return 4}
 if d.Tag==tagArray {return 8}
 if d.Tag==2 {if _,ok:=d.Value.(chaiArray);ok {return 8};return 7}
 return int(d.Tag)
}
func chaiEntries(d *dynamic) []entry[jsString,*dynamic] {
 if d!=nil {
  if d.Tag==tagObject {return d.Value.(*orderedMap[jsString,*dynamic]).Entries}
  if d.Tag==2 {if v,ok:=d.Value.(interface{chaiEntries() []entry[jsString,*dynamic]});ok {return v.chaiEntries()}}
 }
 panic(newTrap("chai object enumerable keys are unsupported"))
}
func chaiProperty(d *dynamic,k jsString) (*dynamic,bool) {
 // Property assertions require presence as well as value equality. A missing
 // key is not equivalent to an own property whose value is undefined.
 for _,e:=range chaiEntries(d) {if e.Key==k {return e.Value,true}}
 // Parsed ordinary objects inherit Object.prototype. These names need a
 // prototype ABI, so do not silently report them absent.
 switch k.String() {case "constructor","toString","valueOf","hasOwnProperty","isPrototypeOf","propertyIsEnumerable","toLocaleString","__proto__","__defineGetter__","__defineSetter__","__lookupGetter__","__lookupSetter__":panic(newTrap("chai inherited property is unsupported"))}
 return nil,false
}
type chaiPair struct {a,b any}
func chaideepEqual(a,b *dynamic) bool {return chaiEqual(a,b,map[chaiPair]bool{})}
func chaiEqual(a,b *dynamic,seen map[chaiPair]bool) bool {
 ak,bk:=chaiKind(a),chaiKind(b)
 if ak!=bk {return false}
 switch ak {
 case 0,9:return true
 case 4:
  x,y:=dynNumber(a),dynNumber(b)
  return math.IsNaN(x)&&math.IsNaN(y) || x==y && (x!=0 || math.Signbit(x)==math.Signbit(y))
 case 1,3,10:return dynEqual(a,b)
 case 7,8:
  if dynEqual(a,b) {return true}
  pair:=chaiPair{a.Value,b.Value}
  if seen[pair] {return true};seen[pair]=true
  if ak==8 {
   x,xok:=a.Value.(chaiArray);y,yok:=b.Value.(chaiArray)
   if !xok||!yok {panic(newTrap("chai array storage is unsupported"))}
   if x.chaiLength()!=y.chaiLength() {return false}
   for i:=0;i<x.chaiLength();i++ {if !chaiEqual(x.chaiAt(i),y.chaiAt(i),seen) {return false}}
   return true
  }
  x,y:=chaiEntries(a),chaiEntries(b)
  if len(x)!=len(y) {return false}
  for _,e:=range x {
   found:=false
   for _,f:=range y {if e.Key==f.Key {found=true;if !chaiEqual(e.Value,f.Value,seen) {return false};break}}
   if !found {return false}
  }
  return true
 default:panic(newTrap("chai deep equality type is unsupported"))
 }
}
// A harness-only exception observation boundary, not generic TS catch
// lowering. Only actual translated payloads are caught; refusals and host
// faults always escape. Chai/check-error accepts primitive string throws too.
func chaiThrowMatch(action func(), expected jsString) (matched bool) {
 defer func(){if x:=recover();x!=nil {
  thrown,ok:=x.(interface{payloadValue() any});if !ok {panic(x)}
  value:=thrown.payloadValue()
  if boxed,ok:=value.(*dynamic);ok {
   if boxed==nil {panic(newTrap("chai unsupported thrown undefined"))}
   value=boxed.Value
  }
  var message jsString
  switch value:=value.(type) {
  case jsString:message=value
  case interface{chaiErrorMessage() jsString}:message=value.chaiErrorMessage()
  default:panic(newTrap("chai throw supports only string and native Error payloads"))
  }
  matched=message.indexOf(expected)>=0
 }}()
 action()
 return false
}
// Chai's shallow array include uses indexOf: strict equality, not the
// SameValueZero of Array.includes. Holes are skipped; NaN never matches.
// This tests-only operation reads boxed storage without converting/copying it.
func chaiarrayInclude(a,b *dynamic) bool {
 if chaiKind(a)!=8 {panic(newTrap("chai shallow include requires an array"))}
 x,ok:=a.Value.(chaiArray);if !ok {panic(newTrap("chai array storage is unsupported"))}
 for i:=0;i<x.chaiLength();i++ {if x.chaiHas(i)&&dynEqual(x.chaiAt(i),b) {return true}}
 return false
}
// Dense-array unordered members: consume one matching occurrence per item.
// Strict members preserve reference identity; deep members use deep-eql.
func chaiMembers(a,b *dynamic,deep bool) bool {
 if chaiKind(a)!=8 || chaiKind(b)!=8 {panic(newTrap("chai members requires arrays"))}
 x,xok:=a.Value.(chaiArray);y,yok:=b.Value.(chaiArray)
 if !xok || !yok {panic(newTrap("chai members array storage is unsupported"))}
 for i:=0;i<x.chaiLength();i++ {if !x.chaiHas(i) {panic(newTrap("chai members requires dense arrays"))}}
 for i:=0;i<y.chaiLength();i++ {if !y.chaiHas(i) {panic(newTrap("chai members requires dense arrays"))}}
 if x.chaiLength()!=y.chaiLength() {return false}
 used:=make([]bool,x.chaiLength())
 for i:=0;i<y.chaiLength();i++ {
  found:=false
  for j:=0;j<x.chaiLength();j++ {if used[j] {continue};same:=dynEqual(x.chaiAt(j),y.chaiAt(i));if deep {same=chaideepEqual(x.chaiAt(j),y.chaiAt(i))};if same {used[j]=true;found=true;break}}
  if !found {return false}
 }
 return true
}
// 2 = no assertion (empty object subset); 1 = included; 0 = not included.
func chaideepInclude(a,b *dynamic) int32 {
 switch chaiKind(a) {
 case 1:
  if chaiKind(b)!=1 {panic(newTrap("chai string include coercion is unsupported"))}
  if a.Value.(jsString).indexOf(b.Value.(jsString))>=0 {return 1};return 0
 case 8:
  x,ok:=a.Value.(chaiArray);if !ok {panic(newTrap("chai array storage is unsupported"))}
  for i:=0;i<x.chaiLength();i++ {if x.chaiHas(i)&&chaideepEqual(x.chaiAt(i),b) {return 1}};return 0
 case 7:
  props:=chaiEntries(b)
  if len(props)==0 {return 2}
  for _,e:=range props {v,has:=chaiProperty(a,e.Key);if !has || !chaideepEqual(v,e.Value) {return 0}}
  return 1
 default:panic(newTrap("chai deep include type is unsupported"))
 }
}
`
