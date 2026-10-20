package golang

import "strings"

// Only live-view programs use the proxy ABI. Native arrays retain their storage;
// a typed proxy delegates every element read and mutation to the original array.
func liveArrayRuntime(source string) string {
	source = strings.Replace(source, "type array[T any] struct{ Items []T;", "type array[T any] struct{ Items []T; live liveArrayStorage; source *dynamic; decode func(*dynamic) T;", 1)
	guards := map[string]string{
		"reserve(n int)":                        "if a.live!=nil {return}",
		"chaiLength() int":                      "if a.live!=nil {return int(a.length())}",
		"push(v T) int32":                       "if a.live!=nil {n:=a.length();a.live.livePut(n,box(v));return n+1}",
		"get(i int32) optional[T]":              "if a.live!=nil {v,ok:=a.live.liveGet(i);if !ok {return optional[T]{}};return present(a.decode(v))}",
		"put(i int32, v T)":                     "if a.live!=nil {a.live.livePut(i,box(v));return}",
		"has(i int32) bool":                     "if a.live!=nil {_,ok:=a.live.liveGet(i);return ok}",
		"delete(i int32) bool":                  "if a.live!=nil {return a.live.liveDelete(i)}",
		"reverse() *array[T]":                   "if a.live!=nil {a.live.liveReverse();return a}",
		"unshift(v T) int32":                    "if a.live!=nil {a.live.liveSplice(0,0,box(v),true);return a.length()}",
		"concat(b *array[T]) *array[T]":         "if a.live!=nil || b.live!=nil {out:=a.slice0();for i:=int32(0);i<b.length();i++ {v:=b.get(i);if !v.Has {panic(newTrap(\"live array concat refuses holes\"))};out.push(v.Value)};return out}",
		"slice0() *array[T]":                    "if a.live!=nil {return a.slice2(0,a.length())}",
		"slice1(i int32) *array[T]":             "if a.live!=nil {return a.slice2(i,a.length())}",
		"slice2(i, j int32) *array[T]":          "if a.live!=nil {return a.liveSlice(i,j)}",
		"slice2(i,j int32) *array[T]":           "if a.live!=nil {return a.liveSlice(i,j)}",
		"splice1(i int32) *array[T]":            "if a.live!=nil {return a.splice2(i,a.length())}",
		"splice1_view(i int32) *array[T]":       "if a.live!=nil {return a.splice1(i)}",
		"splice2(i, n int32) *array[T]":         "if a.live!=nil {return a.proxySplice(i,n,nil,false)}",
		"spliceDrop2(i, n int32) *array[T]":     "if a.live!=nil {a.proxySplice(i,n,nil,false);return nil}",
		"spliceDrop2(i,n int32) *array[T]":      "if a.live!=nil {a.proxySplice(i,n,nil,false);return nil}",
		"splice2(i,n int32) *array[T]":          "if a.live!=nil {return a.proxySplice(i,n,nil,false)}",
		"splice3(i, n int32, v T) *array[T]":    "if a.live!=nil {return a.proxySplice(i,n,box(v),true)}",
		"splice3(i,n int32,v T) *array[T]":      "if a.live!=nil {return a.proxySplice(i,n,box(v),true)}",
		"pop() optional[T]":                     "if a.live!=nil {n:=a.length();if n==0 {return optional[T]{}};v:=a.get(n-1);a.live.liveSplice(n-1,1,nil,false);return v}",
		"shift() optional[T]":                   "if a.live!=nil {if a.length()==0 {return optional[T]{}};v:=a.get(0);a.live.liveSplice(0,1,nil,false);return v}",
		"indexOf(v T) int32":                    "if a.live!=nil {for i:=int32(0);i<a.length();i++ {x:=a.get(i);if x.Has && equal(x.Value,v) {return i}};return -1}",
		"join(sep optional[jsString]) jsString": "if a.live!=nil {return (&array[T]{Items:a.values()}).join(sep)}",
	}
	source = strings.Replace(source, "slice0() *array[T]        {", "slice0() *array[T] {", 1)
	source = strings.Replace(source, "splice1(i int32) *array[T]      {", "splice1(i int32) *array[T] {", 1)
	for signature, guard := range guards {
		source = strings.Replace(source, "func (a *array[T]) "+signature+" {", "func (a *array[T]) "+signature+" {"+guard+";", 1)
	}
	// The sparse implementation has an independent includes loop.
	source = strings.Replace(source, "func (a *array[T]) includes(v T) bool {\n", "func (a *array[T]) includes(v T) bool {\nif a.live!=nil {return a.indexOf(v)>=0};\n", 1)
	source = strings.Replace(source, "func presentArray[T any](a *array[optional[T]]) *array[T] {", "func presentArray[T any](a *array[optional[T]]) *array[T] {if a!=nil && a.live!=nil {panic(newTrap(\"optional narrowing of a live array is unsupported\"))};", 1)
	source = strings.Replace(source, "func referenceArray[T any](a *array[T]) *array[any] {", "func referenceArray[T any](a *array[T]) *array[any] {if a!=nil && a.live!=nil {panic(newTrap(\"reference repacking of a live array is unsupported\"))};", 1)
	source = strings.Replace(source, "func (s *orderedSet[T]) fromArray(a *array[T]) *orderedSet[T] {", "func (s *orderedSet[T]) fromArray(a *array[T]) *orderedSet[T] {if a!=nil && a.live!=nil {for _,v:=range a.values() {s.add(v)};return s};", 1)
	source = strings.Replace(source, "func (d *dynamic) put(k jsString, v *dynamic) {", "func (d *dynamic) put(k jsString, v *dynamic) {if d!=nil && d.Tag==tagArray {d.Value.(*array[any]).dynamicPut(k,v);return};", 1)
	return source + liveArraySource
}

const liveArraySource = `
func liveStringElement(d *dynamic) jsString {
 if d==nil || d.Tag!=1 {panic(newTrap("live string array requires a string element"))}
 return dynString(d)
}
func liveArrayRead[T any](a *array[T],i int32) T {
 v:=a.get(i);if v.Has {return v.Value};var z T
 if nilRef(z) {return z};r:=reflect.ValueOf(z)
 if r.Kind()==reflect.Struct && r.FieldByName("Has").IsValid() {return z}
 panic(newRangeFault())
}
func liveArrayElement[T any](d *dynamic) T {
 var z T
 // Erased reference arrays also hold parsed records/arrays. Keep their tags:
 // the raw map/table is not a nominal object, and nil is a present undefined.
 if reflect.TypeOf(z)==nil && (d==nil || d.Tag==tagObject || d.Tag==tagArray) {return any(d).(T)}
 switch any(z).(type) {
 case int32:n:=dynNumber(d);if n!=math.Trunc(n)||n< -2147483648||n>2147483647 {panic(newRangeFault())};return any(int32(n)).(T)
 case int64:n:=dynNumber(d);if n!=math.Trunc(n)||n< -9007199254740991||n>9007199254740991 {panic(newRangeFault())};return any(int64(n)).(T)
 }
 return unboxValue[T](d)
}
func liveArrayIndex(k jsString) int32 {s:=k.String();i,err:=strconv.ParseUint(s,10,31);if err!=nil || s=="" || strconv.FormatUint(i,10)!=s {panic(newTrap("live array property is not a supported index"))};return int32(i)}
func (a *array[T]) dynamicGet(k jsString) *dynamic {if k==str("length") {return box(float64(a.length()))};s:=k.String();i,err:=strconv.ParseUint(s,10,31);if err!=nil || s=="" || strconv.FormatUint(i,10)!=s {return nil};v:=a.get(int32(i));if !v.Has {return nil};return box(v.Value)}
func (a *array[T]) dynamicPut(k jsString,v *dynamic) {a.livePut(liveArrayIndex(k),v)}

type liveArrayStorage interface {
 liveLength() int32
 liveGet(int32)(*dynamic,bool)
 livePut(int32,*dynamic)
 liveSplice(int32,int32,*dynamic,bool) *array[any]
 liveReverse()
 liveDelete(int32) bool
}
// Explicit erased-array conversions preserve the existing invalid-iterable
// error domain. Field getters use liveArrayView directly so missing properties
// remain observable as undefined until an operation actually consumes them.
func checkedArraySource(d *dynamic,erased bool) *dynamic {
 if erased {if d==nil {panic(newRangeFault())};if _,ok:=d.Value.(liveArrayStorage);!ok {panic(newRangeFault())}}
 if d==nil {panic(newTrap("live typed array requires array storage"))}
 return d
}
func liveArrayView[T any](d *dynamic,decode func(*dynamic)T) *array[T] {
 if d==nil {return nil}
 a,ok:=d.Value.(liveArrayStorage);if !ok {panic(newTrap("live typed array requires array storage"))}
 // Check existing present elements before publishing a view. Future reads check
 // again, so caller writes can never bypass the element representation boundary.
 for i:=int32(0);i<a.liveLength();i++ {v,has:=a.liveGet(i);if has {decode(v)}}
 var zero T
 // Erased reference storage still needs the element converter on every read.
 if reflect.TypeOf(zero)!=nil {if v,ok:=d.Value.(*array[T]);ok {return v}}
 return &array[T]{live:a,source:d,decode:decode}
}
func (a *array[T]) dynamicSource() *dynamic {if a==nil {return nil};return a.source}
func (a *array[T]) length() int32 {if a.live!=nil {return a.live.liveLength()};return int32(len(a.Items))}
func (a *array[T]) values() []T {if a.live==nil {return a.Items};out:=make([]T,a.length());for i:=range out {v:=a.get(int32(i));if !v.Has {panic(newTrap("live typed array dense read refuses holes"))};out[i]=v.Value};return out}
func (a *array[T]) liveLength() int32 {return a.length()}
func (a *array[T]) liveGet(i int32)(*dynamic,bool) {v:=a.get(i);return box(v.Value),v.Has}
func (a *array[T]) livePut(i int32,v *dynamic) {a.put(i,liveArrayElement[T](v))}
func (a *array[T]) liveReverse() {a.reverse()}
func (a *array[T]) liveDelete(i int32) bool {return a.delete(i)}
func (a *array[T]) liveSlice(i,j int32) *array[T] {
 n:=a.length();i=sliceIndex(i,n);j=max(i,sliceIndex(j,n));out:=&array[T]{}
 for ;i<j;i++ {v:=a.get(i);if !v.Has {panic(newTrap("live typed array slice refuses holes"))};out.push(v.Value)};return out
}
func (a *array[T]) proxySplice(i,n int32,v *dynamic,insert bool) *array[T] {
 i=sliceIndex(i,a.length());n=max(0,min(n,a.length()-i))
 out:=a.liveSlice(i,i+n);a.live.liveSplice(i,n,v,insert);return out
}
func (a *array[T]) liveSplice(i,n int32,v *dynamic,insert bool) *array[any] {
 var value T;if insert {value=liveArrayElement[T](v)}
 var removed *array[T];if insert {removed=a.splice3(i,n,value)}else{removed=a.splice2(i,n)}
 out:=&array[any]{};for i:=int32(0);i<removed.length();i++ {x:=removed.get(i);if !x.Has {panic(newTrap("live typed array splice refuses holes"))};out.push(box(x.Value))};return out
}
`
