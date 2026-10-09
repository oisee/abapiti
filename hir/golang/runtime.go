package golang

// runtimeSource is self-contained: generated modules need only the standard library.
const runtimeSource = `package main

import (
	"encoding/binary"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// jsString stores UTF-16LE units, including isolated surrogate sections. Its
// comparable representation makes string and optional equality value based.
type jsString string

func str(s string) jsString {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 2*len(u))
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[2*i:], v)
	}
	return jsString(b)
}
func (s jsString) String() string {
	u := make([]uint16, len(s)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16([]byte(s[2*i : 2*i+2]))
	}
	return string(utf16.Decode(u))
}
func (s jsString) length() int32 { return int32(len(s) / 2) }
func (s jsString) substring(a, b int32) jsString {
	n := s.length()
	a = max(0, min(n, a))
	b = max(0, min(n, b))
	if a > b {
		a, b = b, a
	}
	return s[int(a)*2 : int(b)*2]
}
func (s jsString) charCodeAt(i int32) int32 {
	j := int(i) * 2
	if j < 0 || j >= len(s)-1 {
		panic(rangeFault{})
	}
	return int32(s[j]) | int32(s[j+1])<<8
}

func sliceIndex(i, n int32) int32 {
	if i < 0 {
		i = n + i
	}
	return max(0, min(n, i))
}
func (s jsString) slice(a, b int32) jsString {
	a = sliceIndex(a, s.length())
	b = sliceIndex(b, s.length())
	b = max(a, b)
	return s[2*a : 2*b]
}
func (s jsString) substr(a, n int32) jsString {
	a = sliceIndex(a, s.length())
	n = max(0, min(n, s.length()-a))
	return s[2*a : 2*(a+n)]
}
func (s jsString) charAt(i int32) jsString {
	if i < 0 || i >= s.length() {
		return ""
	}
	return s[2*i : 2*i+2]
}
func (s jsString) indexOf(needle jsString) int32 {
	// Search bytes with Go's optimized implementation, rejecting matches that
	// start inside a UTF-16 unit. Empty needles still match at unit zero.
	offset := 0
	for offset <= len(s)-len(needle) {
		i := strings.Index(string(s[offset:]), string(needle))
		if i < 0 { return -1 }
		i += offset
		if i & 1 == 0 { return int32(i/2) }
		offset = i+1
	}
	return -1
}
func (s jsString) replaceAll(needle, with jsString) jsString {
	if needle == "" {
		out := with
		for i := int32(0); i < s.length(); i++ {
			out += s.charAt(i) + with
		}
		return out
	}
	out := jsString("")
	for {
		i := s.indexOf(needle)
		if i < 0 {
			return out + s
		}
		out += s[:2*i] + with
		s = s[2*i+int32(len(needle)):]
	}
}
func (s jsString) split(sep jsString) *array[jsString] {
	a := &array[jsString]{}
	if sep == "" {
		for i := int32(0); i < s.length(); i++ {
			a.Items = append(a.Items, s.charAt(i))
		}
		return a
	}
	for {
		i := s.indexOf(sep)
		if i < 0 {
			a.Items = append(a.Items, s)
			return a
		}
		a.Items = append(a.Items, s[:2*i])
		s = s[2*i+int32(len(sep)):]
	}
}
func jsWhitespace(c int32) bool {
	return c == 9 || c == 10 || c == 11 || c == 12 || c == 13 || c == 32 || c == 160 || c == 0x1680 || c >= 0x2000 && c <= 0x200a || c == 0x2028 || c == 0x2029 || c == 0x202f || c == 0x205f || c == 0x3000 || c == 0xfeff
}
func (s jsString) trim() jsString {
	a, b := int32(0), s.length()
	for a < b && jsWhitespace(s.charCodeAt(a)) {
		a++
	}
	for b > a && jsWhitespace(s.charCodeAt(b-1)) {
		b--
	}
	return s[2*a : 2*b]
}
// Append a rune as UTF-16LE without temporary per-rune strings.
func writeUnit(b *strings.Builder, c uint16) { b.WriteByte(byte(c));b.WriteByte(byte(c>>8)) }
func writeRune(b *strings.Builder, r rune) {
 if r<=0xffff { writeUnit(b,uint16(r));return }
 hi,lo:=utf16.EncodeRune(r);writeUnit(b,uint16(hi));writeUnit(b,uint16(lo))
}
// Scan until the first changed unit, copy its prefix, and transform only the
// suffix. Any non-ASCII unit restarts the unchanged Unicode implementation.
func (s jsString) asciiCase(lower bool) (jsString, bool) {
 from,to,delta:=byte('a'),byte('z'),byte(32)
 if lower {from,to='A','Z'}
 first:=0
 low,high:=uint64(0x001f001f001f001f),uint64(0x0005000500050005)
 if lower {low,high=0x003f003f003f003f,0x0025002500250025}
 for first+8<=len(s) {
  word:=binary.LittleEndian.Uint64([]byte(s[first:first+8]))
  if word&0xff80ff80ff80ff80!=0 {return "",false}
  if (word+low)&^(word+high)&0x0080008000800080!=0 {break}
  first+=8
 }
 for ;first+1<len(s);first+=2 {
  c:=s[first]
  if s[first+1]!=0||c>=128 {return "",false}
  if c>=from&&c<=to {break}
 }
 if first==len(s) {return s,true}
 var out strings.Builder;out.Grow(len(s));out.WriteString(string(s[:first]))
 for i:=first;i+1<len(s);i+=2 {
  c:=s[i]
  if s[i+1]!=0||c>=128 {return "",false}
  if c>=from&&c<=to {if lower {c+=delta}else{c-=delta}}
  out.WriteByte(c);out.WriteByte(0)
 }
 return jsString(out.String()),true
}
func (s jsString) upper() jsString {
 if out,ok:=s.asciiCase(false);ok {return out}
 var out strings.Builder
 out.Grow(len(s))
 for i:=int32(0);i<s.length();i++ {
  c:=s.charCodeAt(i)
  r:=rune(c)
  if c>=0xd800&&c<=0xdbff&&i+1<s.length() {
   d:=s.charCodeAt(i+1)
   if d>=0xdc00&&d<=0xdfff {r=utf16.DecodeRune(rune(c),rune(d));i++}
  }
  if r>=0xd800&&r<=0xdfff {writeUnit(&out,uint16(r));continue}
  if v,ok:=upperExpansion[r];ok {out.WriteString(string(str(v)))} else {writeRune(&out,unicode.ToUpper(r))}
 }
 return jsString(out.String())
}

type optional[T any] struct {
	Value T
	Has   bool
}

func present[T any](v T) optional[T] { return optional[T]{v, true} }
func unwrap[T any](v optional[T]) T {
	if !v.Has {
		panic(rangeFault{})
	}
	return v.Value
}

type dynamic struct {
	Tag   uint8
	Value any
}

func box(v any) *dynamic {
	if nilRef(v) {
		return nil
	}
	if d, ok := v.(*dynamic); ok {
		return d
	}
	if s, ok := v.(interface{ dynamicSource() *dynamic }); ok && s.dynamicSource() != nil {
		return s.dynamicSource()
	}
	tag := uint8(2)
	switch v.(type) {
	case *classDescriptor:
		tag = 10
	case jsString:
		tag = 1
	case bool:
		tag = 3
	case int32:
		tag = 4
	case int64:
		tag = 5
	case float64:
		tag = 6
	}
	return &dynamic{tag, v}
}
func castRef[T any](v any) T {
	if nilRef(v) {
		var z T
		return z
	}
	r, ok := v.(T)
	if !ok {
		panic(rangeFault{})
	}
	return r
}
func dynRef(d *dynamic) any {
	if d == nil || d.Tag != 2 {
		panic(rangeFault{})
	}
	return d.Value
}
func integerString(x int64) jsString     { return str(strconv.FormatInt(x, 10)) }
func numberRemainder2(x float64) float64 { return math.Mod(x, 2) }
func dynString(d *dynamic) jsString {
	if d == nil || d.Tag != 1 {
		panic(rangeFault{})
	}
	return d.Value.(jsString)
}

type trap struct{ Source string }
type rangeFault struct{}
type payload[T any] struct {
	Type  string
	Value T
}
type returned[T any] struct{ Value T }
type returnedVoid struct{}

func nilRef(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Ptr, reflect.Interface:
		return r.IsNil()
	}
	return false
}

// Interfaces hold the original concrete object, never an embedded-base pointer.
func equal(a, b any) bool {
	if nilRef(a) || nilRef(b) {
		return nilRef(a) && nilRef(b)
	}
	return a == b
}
// The widened result is exact; narrowing still traps signed overflow.
func checkedI32(v int64) int32 {
 if v < -2147483648 || v > 2147483647 {panic(rangeFault{})}
 return int32(v)
}
func checkedAddI64(a,b int64) int64 {
 v:=a+b
 if b>0&&v<a || b<0&&v>a {panic(rangeFault{})}
 return v
}
func checkedSubI64(a,b int64) int64 {
 v:=a-b
 if b<0&&v<a || b>0&&v>a {panic(rangeFault{})}
 return v
}
func safeInteger(v int64) int64 {
 if v < -9007199254740991 || v > 9007199254740991 {panic(rangeFault{})}
 return v
}
func integerArithmetic(a, b int64, op string, checked bool, bits int) int64 {
	var v int64
	switch op {
	case "+":
		v = a + b
		if (b > 0 && v < a) || (b < 0 && v > a) { panic(rangeFault{}) }
	case "-":
		v = a - b
		if (b < 0 && v < a) || (b > 0 && v > a) { panic(rangeFault{}) }
	case "*":
		v = a * b
		if b != 0 && (v / b != a || a == -9223372036854775808 && b == -1) { panic(rangeFault{}) }
	case "/":
		if b == 0 || a == -9223372036854775808 && b == -1 { panic(rangeFault{}) }
		v = a / b
	case "%":
		if b == 0 { panic(rangeFault{}) }
		v = a % b
	default:
		// Preserve the helper's previous identity result for unknown operators.
		v = a
	}
	if bits == 32 && (v < -2147483648 || v > 2147483647) {
		panic(rangeFault{})
	}
	if checked && (v < -9007199254740991 || v > 9007199254740991) {
		panic(rangeFault{})
	}
	return v
}
func negativeZero() float64 { return math.Copysign(0, -1) }
func finite(x float64) float64 {
	if math.IsInf(x, 0) || math.IsNaN(x) {
		panic(rangeFault{})
	}
	return x
}
func numberIndex(x float64) int32 {
	if x > 2147483647 {
		return 2147483647
	}
	if x < -2147483648 {
		return -2147483648
	}
	return int32(math.Trunc(x))
}
func numberString(x float64) jsString {
	if math.IsNaN(x) || math.IsInf(x, 0) || x != math.Trunc(x) || math.Abs(x) > 9007199254740991 {
		panic(rangeFault{})
	}
	return str(strconv.FormatInt(int64(x), 10))
}

type array[T any] struct{ Items []T }

func (a *array[T]) reserve(n int) {
 if a==nil||n<=cap(a.Items) {return}
 items:=make([]T,len(a.Items),n);copy(items,a.Items);a.Items=items
}

// Capacity is not observable in HIR. Geometric growth reduces copying and
// allocation for large reference arrays while keeping every array independent.
func (a *array[T]) push(v T) int32 {
 if len(a.Items)==cap(a.Items) {
  items:=make([]T,len(a.Items),max(4,2*cap(a.Items)))
  copy(items,a.Items)
  a.Items=items
 }
 a.Items=append(a.Items,v)
 return int32(len(a.Items))
}
func (a *array[T]) get(i int32) optional[T] {
	if i < 0 || int(i) >= len(a.Items) {
		return optional[T]{}
	}
	return present(a.Items[i])
}
func (a *array[T]) put(i int32, v T) {
	if i < 0 {
		panic(rangeFault{})
	}
	for len(a.Items) <= int(i) {
		var z T
		a.Items = append(a.Items, z)
	}
	a.Items[i] = v
}

type entry[K, V any] struct {
	Key   K
	Value V
}
// Collection keys are primitive values or references. Normalize typed nil
// references to the zero key, retaining the equality used by equal().
func collectionKey[K comparable](k K) K {
 switch any(k).(type) {
 case jsString, bool, int32, int64, float64:
  return k
 default:
  if nilRef(k) { var zero K; return zero }
  return k
 }
}
type orderedMap[K comparable, V any] struct {
 Entries []entry[K,V]
 index map[K]int
}
func (m *orderedMap[K,V]) ensureIndex() {
 if m.index != nil { return }
 m.index=make(map[K]int,len(m.Entries))
 for i,e:=range m.Entries { m.index[collectionKey(e.Key)]=i }
}
func (m *orderedMap[K, V]) set(k K, v V) *orderedMap[K, V] {
 m.ensureIndex()
 key:=collectionKey(k)
 if i,ok:=m.index[key];ok { m.Entries[i].Value=v;return m }
 m.index[key]=len(m.Entries)
 m.Entries=append(m.Entries,entry[K,V]{k,v})
 return m
}
func (m *orderedMap[K,V]) get(k K) optional[V] {
 m.ensureIndex()
 if i,ok:=m.index[collectionKey(k)];ok { return present(m.Entries[i].Value) }
 return optional[V]{}
}
func (m *orderedMap[K,V]) has(k K) bool { m.ensureIndex();_,ok:=m.index[collectionKey(k)];return ok }
func (m *orderedMap[K, V]) keys() *array[K] {
	a := &array[K]{}
	for _, e := range m.Entries {
		a.Items = append(a.Items, e.Key)
	}
	return a
}

func (s *orderedSet[T]) fromArray(a *array[T]) *orderedSet[T] {
	for _, v := range a.Items {
		s.add(v)
	}
	return s
}

// A byte-valued Number can use a bitmap; other keys retain typed lookup.
func floatByte(v any) (uint8,bool) {
 n,ok:=v.(float64)
 if !ok || n<0 || n>=256 {return 0,false}
 b:=uint8(n)
 return b,n==float64(b)
}
type orderedSet[T comparable] struct {
 Items []T
 index map[T]int
 bytes [4]uint64
}
func (s *orderedSet[T]) ensureIndex() {
 if s.index != nil { return }
 s.index=make(map[T]int,len(s.Items))
 for i,v:=range s.Items { s.index[collectionKey(v)]=i;if b,ok:=floatByte(v);ok {s.bytes[b>>6]|=uint64(1)<<(b&63)} }
}
func (s *orderedSet[T]) has(v T) bool {
 if s.index!=nil {if b,ok:=floatByte(v);ok {return s.bytes[b>>6]&(uint64(1)<<(b&63))!=0}}
 // Small primitive sets avoid hashing and compare unboxed typed values.
 if len(s.Items)<=16 {
  switch any(v).(type) {
  case jsString, bool, int32, int64, float64:
   for _,x:=range s.Items { if x==v { return true } }
   return false
  }
 }
 s.ensureIndex()
 _,ok:=s.index[collectionKey(v)]
 return ok
}
func (s *orderedSet[T]) add(v T) *orderedSet[T] {
 s.ensureIndex()
 key:=collectionKey(v)
 if _,ok:=s.index[key];!ok { s.index[key]=len(s.Items);s.Items=append(s.Items,v);if b,ok:=floatByte(v);ok {s.bytes[b>>6]|=uint64(1)<<(b&63)} }
 return s
}
func (s *orderedSet[T]) values() *array[T] { return &array[T]{Items: append([]T(nil), s.Items...)} }
func (a *array[T]) reverse() *array[T] {
	for i, j := 0, len(a.Items)-1; i < j; i, j = i+1, j-1 {
		a.Items[i], a.Items[j] = a.Items[j], a.Items[i]
	}
	return a
}
func (a *array[T]) unshift(v T) int32 {
	a.Items = append([]T{v}, a.Items...)
	return int32(len(a.Items))
}
func (a *array[T]) concat(b *array[T]) *array[T] {
	out := a.slice0()
	out.Items = append(out.Items, b.Items...)
	return out
}
func (a *array[T]) slice0() *array[T]        { return a.slice2(0, int32(len(a.Items))) }
func (a *array[T]) slice1(i int32) *array[T] { return a.slice2(i, int32(len(a.Items))) }
func (a *array[T]) slice2(i, j int32) *array[T] {
	n := int32(len(a.Items))
	i = sliceIndex(i, n)
	j = max(i, sliceIndex(j, n))
	return &array[T]{Items: append([]T(nil), a.Items[i:j]...)}
}
func (a *array[T]) splice1(i int32) *array[T]      { return a.splice2(i, int32(len(a.Items))) }
func (a *array[T]) splice1_view(i int32) *array[T] { return a.splice1(i) }
func (a *array[T]) splice2(i, n int32) *array[T] {
	i = sliceIndex(i, int32(len(a.Items)))
	n = max(0, min(n, int32(len(a.Items))-i))
	out := a.slice2(i, i+n)
	a.Items = append(a.Items[:i], a.Items[i+n:]...)
	return out
}
func (a *array[T]) splice3(i, n int32, v T) *array[T] {
	i = sliceIndex(i, int32(len(a.Items)))
	out := a.splice2(i, n)
	a.Items = append(a.Items, v)
	copy(a.Items[i+1:], a.Items[i:len(a.Items)-1])
	a.Items[i] = v
	return out
}
func (a *array[T]) pop() optional[T] {
	if len(a.Items) == 0 {
		return optional[T]{}
	}
	v := a.Items[len(a.Items)-1]
	a.Items = a.Items[:len(a.Items)-1]
	return present(v)
}
func (a *array[T]) shift() optional[T] {
	if len(a.Items) == 0 {
		return optional[T]{}
	}
	v := a.Items[0]
	a.Items = a.Items[1:]
	return present(v)
}
func (a *array[T]) indexOf(v T) int32 {
	for i, x := range a.Items {
		if equal(x, v) {
			return int32(i)
		}
	}
	return -1
}
func (a *array[T]) includes(v T) bool { return a.indexOf(v) >= 0 }
func (a *array[T]) join(sep optional[jsString]) jsString {
	s := str(",")
	if sep.Has {
		s = sep.Value
	}
	out := jsString("")
	for i, v := range a.Items {
		if i > 0 {
			out += s
		}
		out += primitiveString(v)
	}
	return out
}
func primitiveString(v any) jsString {
	switch x := v.(type) {
	case jsString:
		return x
	case bool:
		return str(strconv.FormatBool(x))
	case int32:
		return integerString(int64(x))
	case int64:
		return integerString(x)
	case float64:
		return numberString(x)
	}
	panic(rangeFault{})
}
func (m *orderedMap[K, V]) values() *array[V] {
	a := &array[V]{}
	for _, e := range m.Entries {
		a.Items = append(a.Items, e.Value)
	}
	return a
}
func (m *orderedMap[K,V]) delete(k K) bool {
 m.ensureIndex()
 key:=collectionKey(k)
 i,ok:=m.index[key];if !ok { return false }
 delete(m.index,key)
 copy(m.Entries[i:],m.Entries[i+1:])
 var zero entry[K,V];m.Entries[len(m.Entries)-1]=zero
 m.Entries=m.Entries[:len(m.Entries)-1]
 for j:=i;j<len(m.Entries);j++ { m.index[collectionKey(m.Entries[j].Key)]=j }
 return true
}
func (s *orderedSet[T]) delete(v T) bool {
 s.ensureIndex()
 key:=collectionKey(v)
 i,ok:=s.index[key];if !ok { return false }
 delete(s.index,key)
 if b,ok:=floatByte(v);ok {s.bytes[b>>6] &^= uint64(1)<<(b&63)}
 copy(s.Items[i:],s.Items[i+1:])
 var zero T;s.Items[len(s.Items)-1]=zero
 s.Items=s.Items[:len(s.Items)-1]
 for j:=i;j<len(s.Items);j++ { s.index[collectionKey(s.Items[j])]=j }
 return true
}
func (s *orderedSet[T]) copy(other *orderedSet[T]) *orderedSet[T] {
	out := &orderedSet[T]{}
	for _, v := range other.Items {
		out.add(v)
	}
	return out
}
func (s jsString) startsWith(n jsString) bool { return s.length() >= n.length() && s[:len(n)] == n }
func (s jsString) endsWith(n jsString) bool {
	return s.length() >= n.length() && s[len(s)-len(n):] == n
}
func (s jsString) at(i int32) optional[jsString] {
	if i < 0 || i >= s.length() {
		return optional[jsString]{}
	}
	return present(s.charAt(i))
}
func (s jsString) replaceFirst(n, v jsString) jsString {
	i := s.indexOf(n)
	if i < 0 {
		return s
	}
	return s[:2*i] + v + s[2*i+int32(len(n)):]
}
func (s jsString) repeatIndent(n float64) jsString {
	n = math.Trunc(n)
	if n <= 0 {
		return ""
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n > 2147483647 {
		panic(rangeFault{})
	}
	return jsString(strings.Repeat(string(s), int(n)))
}
func compareDomain(a, b jsString, alphabet string) int32 {
	for _, s := range []jsString{a, b} {
		for i := int32(0); i < s.length(); i++ {
			if strings.IndexRune(alphabet, rune(s.charCodeAt(i))) < 0 {
				panic(rangeFault{})
			}
		}
	}
	for i := int32(0); i < min(a.length(), b.length()); i++ {
		x := strings.IndexRune(alphabet, rune(a.charCodeAt(i)))
		y := strings.IndexRune(alphabet, rune(b.charCodeAt(i)))
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	if a.length() < b.length() {
		return -1
	}
	if a.length() > b.length() {
		return 1
	}
	return 0
}
func (s jsString) parseInt10() optional[float64] {
	s = s.trim()
	i := int32(0)
	sign := float64(1)
	if s.charAt(i) == str("-") {
		sign = -1
		i++
	} else if s.charAt(i) == str("+") {
		i++
	}
	start := i
	v := float64(0)
	for i < s.length() {
		c := s.charCodeAt(i)
		if c < 48 || c > 57 {
			break
		}
		v = finite(v*10 + float64(c-48))
		i++
	}
	if i == start {
		return optional[float64]{}
	}
	return present(sign * v)
}
func (s jsString) parseInt10i64() optional[int64] {
	v := s.parseInt10()
	if !v.Has {
		return optional[int64]{}
	}
	if v.Value < -9223372036854775808 || v.Value >= 9223372036854775808 {
		panic(rangeFault{})
	}
	return present(int64(v.Value))
}

type jsRegExp struct {
	Source, Flags jsString
	compiled      *regexp.Regexp
	LastIndex     int32
}

func newRegExp(pattern, flags jsString) *jsRegExp {
	p, f := pattern.String(), flags.String()
	translated := ""
	switch {
	case (p == "^Y" || p == "^Z") && f == "":
		translated = p
	case p == "test$" && f == "i":
		translated = "[tT][eE][sS][tT]$"
	case p == "a.c" && f == "i":
		translated = "[aA][^\\n\\r\\x{2028}\\x{2029}][cC]"
	case p == "x/y" && f == "gi":
		translated = "[xX]/[yY]"
	default:
		panic(trap{Source: "not supported in the Go prototype: JavaScript regexp /" + p + "/" + f})
	}
	return &jsRegExp{pattern.replaceAll(str("/"), str("\\/")), flags, regexp.MustCompile(translated), 0}
}
func unitText(s jsString) string {
	var b strings.Builder
	for i := int32(0); i < s.length(); i++ {
		b.WriteRune(rune(s.charCodeAt(i)))
	}
	return b.String()
}
func (r *jsRegExp) test(s jsString) bool {
	start := int32(0)
	global := r.Flags.indexOf(str("g")) >= 0
	if global {
		start = r.LastIndex
	}
	if start < 0 || start > s.length() {
		r.LastIndex = 0
		return false
	}
	input := unitText(s[2*start:])
	m := r.compiled.FindStringIndex(input)
	if m == nil {
		if global {
			r.LastIndex = 0
		}
		return false
	}
	if global {
		r.LastIndex = start + int32(utf8.RuneCountInString(input[:m[1]]))
	}
	return true
}
func (r *jsRegExp) match_test(s jsString) bool {
	if r.Flags.indexOf(str("g")) < 0 {
		return r.test(s)
	}
	r.LastIndex = 0
	found := r.test(s)
	r.LastIndex = 0
	return found
}
func (s jsString) replaceRegex(r *jsRegExp, v jsString) jsString {
	if v.indexOf(str("$")) >= 0 {
		panic(trap{Source: "not supported in the Go prototype: regexp replacement substitutions"})
	}
	input := unitText(s)
	global := r.Flags.indexOf(str("g")) >= 0
	limit := 1
	if global {
		limit = -1
		r.LastIndex = 0
	}
	matches := r.compiled.FindAllStringIndex(input, limit)
	out := jsString("")
	last := int32(0)
	for _, m := range matches {
		a := int32(utf8.RuneCountInString(input[:m[0]]))
		b := int32(utf8.RuneCountInString(input[:m[1]]))
		out += s[2*last:2*a] + v
		last = b
	}
	return out + s[2*last:]
}
func (r *jsRegExp) toString() jsString { return str("/") + r.Source + str("/") + r.Flags }

const (
	tagObject uint8 = 7
	tagArray  uint8 = 8
	tagNull   uint8 = 9
)

func dynNull() *dynamic { return &dynamic{Tag: tagNull} }
func dynEqual(a, b *dynamic) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Tag >= 4 && a.Tag <= 6 && b.Tag >= 4 && b.Tag <= 6 {
		return dynNumber(a) == dynNumber(b)
	}
	if a.Tag != b.Tag {
		return false
	}
	switch a.Tag {
	case tagNull:
		return true
	case tagObject, tagArray:
		return a == b
	case 4, 5, 6:
		return dynNumber(a) == dynNumber(b)
	default:
		return equal(a.Value, b.Value)
	}
}
func dynNumber(d *dynamic) float64 {
	if d == nil {
		panic(rangeFault{})
	}
	switch d.Tag {
	case 4:
		return float64(d.Value.(int32))
	case 5:
		return float64(d.Value.(int64))
	case 6:
		return d.Value.(float64)
	}
	panic(rangeFault{})
}
func dynBoolean(d *dynamic) bool {
	if d == nil || d.Tag != 3 {
		panic(rangeFault{})
	}
	return d.Value.(bool)
}
func dynTypeof(d *dynamic) jsString {
	if d == nil {
		return str("undefined")
	}
	switch d.Tag {
	case 10:
		return str("function")
	case 1:
		return str("string")
	case 3:
		return str("boolean")
	case 4, 5, 6:
		return str("number")
	}
	return str("object")
}
func dynToString(d *dynamic) jsString {
	if d == nil {
		return str("undefined")
	}
	switch d.Tag {
	case tagNull:
		return str("null")
	case 4, 5, 6:
		return numberString(dynNumber(d))
	case 1, 3:
		return primitiveString(d.Value)
	default:
		panic(rangeFault{})
	}
}
func dynTruth(d *dynamic) bool {
	if d == nil || d.Tag == tagNull {
		return false
	}
	switch d.Tag {
	case 1:
		return d.Value.(jsString).length() > 0
	case 3:
		return d.Value.(bool)
	case 4, 5, 6:
		return dynNumber(d) != 0
	}
	return true
}
func (d *dynamic) get(k jsString) *dynamic {
	if d == nil {
		panic(rangeFault{})
	}
	switch d.Tag {
	case 2:
		if m, ok := d.Value.(interface{ dynamicGet(jsString) *dynamic }); ok {
			return m.dynamicGet(k)
		}
		panic(rangeFault{})
	case tagObject:
		return d.Value.(*orderedMap[jsString, *dynamic]).get(k).Value
	case tagArray:
		a := d.Value.(*array[*dynamic])
		if k == str("length") {
			return box(float64(len(a.Items)))
		}
		s := k.String()
		i, err := strconv.ParseUint(s, 10, 31)
		if err != nil || s == "" || strconv.FormatUint(i, 10) != s {
			panic(rangeFault{})
		}
		return a.get(int32(i)).Value
	case 1:
		if k == str("length") {
			return box(float64(d.Value.(jsString).length()))
		}
		return nil
	case 3, 4, 5, 6:
		return nil
	}
	panic(rangeFault{})
}
func (d *dynamic) put(k jsString, v *dynamic) {
	if d != nil && d.Tag == 2 {
		if m, ok := d.Value.(interface{ dynamicPut(jsString, *dynamic) }); ok {
			m.dynamicPut(k, v)
			return
		}
	}
	if d == nil || d.Tag != tagObject {
		panic(rangeFault{})
	}
	d.Value.(*orderedMap[jsString, *dynamic]).set(k, v)
}
func isArray(d *dynamic) bool {
	if d == nil {
		return false
	}
	if d.Tag == tagArray {
		return true
	}
	_, ok := d.Value.(interface{ arrayMarker() })
	return ok
}
func (a *array[T]) arrayMarker() {}

var telemetryStart = time.Now()

func telemetry() float64 { return float64(time.Since(telemetryStart).Nanoseconds()) / 1e6 }

func unboxValue[T any](d *dynamic) T {
	var z T
	if _, ok := any(z).(*dynamic); ok {
		return any(d).(T)
	}
	if d == nil {
		panic(rangeFault{})
	}
	if v, ok := d.Value.(T); ok {
		return v
	}
	if v, ok := any(dynNumberIfNeeded(d, z)).(T); ok {
		return v
	}
	panic(rangeFault{})
}
func dynNumberIfNeeded[T any](d *dynamic, z T) any {
	switch any(z).(type) {
	case float64:
		return dynNumber(d)
	default:
		return nil
	}
}
func dynMap[V any](d *dynamic) *orderedMap[jsString, V] {
	if d == nil {
		panic(rangeFault{})
	}
	if m, ok := d.Value.(*orderedMap[jsString, V]); ok {
		return m
	}
	var bag *orderedMap[jsString, *dynamic]
	if d.Tag == tagObject || d.Tag == 2 {
		bag, _ = d.Value.(*orderedMap[jsString, *dynamic])
	}
	if bag == nil {
		panic(rangeFault{})
	}
	out := &orderedMap[jsString, V]{}
	for _, e := range bag.Entries {
		out.set(e.Key, unboxValue[V](e.Value))
	}
	return out
}
func (m *orderedMap[K, V]) dynamicGet(k jsString) *dynamic {
	key, ok := any(k).(K)
	if !ok {
		panic(rangeFault{})
	}
	v := m.get(key)
	if !v.Has {
		return nil
	}
	return box(v.Value)
}
func (m *orderedMap[K, V]) dynamicPut(k jsString, v *dynamic) {
	key, ok := any(k).(K)
	if !ok {
		panic(rangeFault{})
	}
	m.set(key, unboxValue[V](v))
}

type classDescriptor struct {
	Name    jsString
	Parent  *classDescriptor
	Statics []jsString
	Factory func() any
}

func (d *classDescriptor) has(k jsString) bool {
	if d != nil {
		for _, s := range d.Statics {
			if s == k {
				return true
			}
		}
	}
	return false
}
func classOf(v any) *classDescriptor {
	if nilRef(v) {
		return nil
	}
	if d, ok := v.(interface{ descriptor() *classDescriptor }); ok {
		return d.descriptor()
	}
	panic(rangeFault{})
}
func descriptorInstance(v any, d *classDescriptor) bool {
	if nilRef(v) {
		return false
	}
	for c := classOf(v); c != nil; c = c.Parent {
		if c == d {
			return true
		}
	}
	return false
}
func dynClass(d *dynamic) *classDescriptor {
	if d == nil || d.Tag != 10 {
		panic(rangeFault{})
	}
	return d.Value.(*classDescriptor)
}

func cased(r rune) bool {
	return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r) || unicode.Is(unicode.Other_Uppercase, r) || unicode.Is(unicode.Other_Lowercase, r)
}
func (s jsString) lower() jsString {
	if out,ok:=s.asciiCase(true);ok {return out}
	var rs []rune
	for i := int32(0); i < s.length(); i++ {
		r := rune(s.charCodeAt(i))
		if utf16.IsSurrogate(r) && r <= 0xdbff && i+1 < s.length() {
			d := rune(s.charCodeAt(i + 1))
			if d >= 0xdc00 && d <= 0xdfff {
				r = utf16.DecodeRune(r, d)
				i++
			}
		}
		rs = append(rs, r)
	}
	out := jsString("")
	for i, r := range rs {
		if r >= 0xd800 && r <= 0xdfff {
			out += unit(int32(r))
			continue
		}
		if r == 'Σ' {
			before, after := false, false
			for j := i - 1; j >= 0; j-- {
				if caseIgnorable[rs[j]] {
					continue
				}
				before = cased(rs[j])
				break
			}
			for j := i + 1; j < len(rs); j++ {
				if caseIgnorable[rs[j]] {
					continue
				}
				after = cased(rs[j])
				break
			}
			if before && !after {
				out += str("ς")
				continue
			}
		}
		if v, ok := lowerExpansion[r]; ok {
			out += str(v)
		} else {
			out += str(string(unicode.ToLower(r)))
		}
	}
	return out
}

func referenceArray[T any](a *array[T]) *array[any] {
	out := &array[any]{}
	for _, v := range a.Items {
		out.push(v)
	}
	return out
}
func referenceSetFromArray[T comparable](s *orderedSet[T], a *array[any]) *orderedSet[T] {
	for _, v := range a.Items {
		s.add(castRef[T](v))
	}
	return s
}
`
