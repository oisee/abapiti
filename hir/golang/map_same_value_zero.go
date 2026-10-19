package golang

import "strings"

// Only programs with erased Map keys need this adapter. Concrete maps keep
// their typed indexes; the existing Map entries, counters and proxies stay live.
func boxedMapRuntime(source string) string {
	for _, patch := range [][2]string{
		{"struct {Value K; NaN bool", "struct {Value K; NaN bool; Boxed boxedMapIndexKey"},
		{"func collectionKey[K comparable](k K) collectionIndexKey[K] {", "func collectionKey[K comparable](k K) collectionIndexKey[K] {if d,ok:=any(k).(*dynamic);ok {return collectionIndexKey[K]{Boxed:boxedMapKey(d)}};"},
		{"func collectionValue[K comparable](k K) K {", "func collectionValue[K comparable](k K) K {if d,ok:=any(k).(*dynamic);ok && d!=nil && d.Tag>=4 && d.Tag<=6 && dynNumber(d)==0 {return any(box(float64(0))).(K)};"},
	} {
		if !strings.Contains(source, patch[0]) {
			panic("boxed Map runtime patch target moved: " + patch[0])
		}
		source = strings.Replace(source, patch[0], patch[1], 1)
	}
	return source + boxedMapKeySource
}

const boxedMapKeySource = `
// Payloads never enter a Go map as unchecked interface keys. JS objects in
// this runtime have pointer storage, even when their fields are noncomparable.
// Keep those pointers alive in the index and preserve their storage identity.
type boxedMapIndexKey struct {kind uint8; text jsString; number float64; flag bool; identity any}
func boxedMapKey(d *dynamic) boxedMapIndexKey {
 if d==nil {return boxedMapIndexKey{}}
 switch d.Tag {
 case tagNull: return boxedMapIndexKey{kind:9}
 case 1: return boxedMapIndexKey{kind:1,text:d.Value.(jsString)}
 case 3: return boxedMapIndexKey{kind:3,flag:d.Value.(bool)}
 case 4,5,6:
  n:=dynNumber(d);if math.IsNaN(n) {return boxedMapIndexKey{kind:12}}
  return boxedMapIndexKey{kind:6,number:n}
 case 11: return boxedMapIndexKey{kind:11,text:jsString(d.Value.(jsBigInt))}
 }
 if s,ok:=d.Value.(interface{dynamicSource()*dynamic});ok && s.dynamicSource()!=nil {return boxedMapKey(s.dynamicSource())}
 r:=reflect.ValueOf(d.Value)
 if r.IsValid() && r.Kind()==reflect.Ptr && !r.IsNil() {
  return boxedMapIndexKey{kind:2,identity:d.Value}
 }
 panic(newTrap("boxed Map key has no supported JavaScript identity storage"))
}
`
