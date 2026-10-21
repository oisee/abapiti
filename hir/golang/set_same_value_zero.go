package golang

import "strings"

// Go maps cannot retrieve a NaN key. Use a comparable canonical key for JS
// SameValueZero, while keeping the original values in insertion order.
func sameValueZeroRuntime(source string) string {
	start := mustIndex(source, "type orderedSet[", "SameValueZero set block")
	end := mustIndex(source[start:], "func (a *array[T]) reverse", "SameValueZero set block end") + start
	block := source[start:end]
	block = strings.ReplaceAll(block, "map[collectionIndexKey[T]]int", "map[any]int")
	block = strings.ReplaceAll(block, "collectionKey(", "setKey(")
	block = strings.ReplaceAll(block, "if x==v", "if setKey(x)==setKey(v)")
	block = mustReplace(block, "func (s *orderedSet[T]) add(v T) *orderedSet[T] {", "func (s *orderedSet[T]) add(v T) *orderedSet[T] {v=normalizeSetValue(v);", "SameValueZero set add")
	source = source[:start] + block + source[end:]
	start = mustIndex(source, "func (s *orderedSet[T]) delete", "SameValueZero set delete")
	end = mustIndex(source[start:], "func (s *orderedSet[T]) copy", "SameValueZero set delete end") + start
	source = source[:start] + strings.ReplaceAll(source[start:end], "collectionKey(", "setKey(") + source[end:]
	return source + setSameValueZeroSource
}

const setSameValueZeroSource = `
type setNaNKey struct{}
type setNullKey struct{}
func setKey(v any) any {
 if nilRef(v) {return nil}
 switch x:=v.(type) {
 case float64:
  if math.IsNaN(x) {return setNaNKey{}};return x
 case int32: return float64(x)
 case int64: return float64(x)
 case *dynamic:
  if x.Tag==tagNull {return setNullKey{}}
  return setKey(x.Value)
 }
 // Optional storage represents JS undefined or its unboxed payload.
 r:=reflect.ValueOf(v)
 if r.Kind()==reflect.Struct {
  has,value:=r.FieldByName("Has"),r.FieldByName("Value")
  if has.IsValid() && has.Kind()==reflect.Bool && value.IsValid() {
   if !has.Bool() {return nil};return setKey(value.Interface())
  }
 }
 return v
}
func normalizeSetValue[T comparable](v T) T {
 switch x:=any(v).(type) {
 case float64: if x==0 {return any(float64(0)).(T)}
 case optional[float64]: if x.Has && x.Value==0 {x.Value=0;return any(x).(T)}
 case *dynamic:
  if x!=nil {if n,ok:=x.Value.(float64);ok && n==0 {return any(box(float64(0))).(T)}}
 }
 return v
}
`
