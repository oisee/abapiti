package golang

// runtimeSource is self-contained: generated modules need only the standard library.
const runtimeSource = `package main

import (
	"encoding/binary"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"unicode/utf16"
 "unicode"
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
	if i < 0 || i >= s.length() {
		panic(rangeFault{})
	}
	return int32(binary.LittleEndian.Uint16([]byte(s[int(i)*2 : int(i)*2+2])))
}

func sliceIndex(i,n int32) int32 { if i<0 {i=n+i}; return max(0,min(n,i)) }
func (s jsString) slice(a,b int32) jsString { a=sliceIndex(a,s.length());b=sliceIndex(b,s.length());b=max(a,b);return s[2*a:2*b] }
func (s jsString) substr(a,n int32) jsString {a=sliceIndex(a,s.length());n=max(0,min(n,s.length()-a));return s[2*a:2*(a+n)]}
func (s jsString) charAt(i int32) jsString {if i<0 || i>=s.length(){return ""};return s[2*i:2*i+2]}
func (s jsString) indexOf(needle jsString) int32 {for i:=int32(0);i<=s.length()-needle.length();i++ {if s[2*i:2*i+int32(len(needle))]==needle{return i}};return -1}
func (s jsString) replaceAll(needle,with jsString) jsString { if needle=="" {out:=with;for i:=int32(0);i<s.length();i++ {out+=s.charAt(i)+with};return out};out:=jsString("");for {i:=s.indexOf(needle);if i<0{return out+s};out+=s[:2*i]+with;s=s[2*i+int32(len(needle)):]}}
func (s jsString) split(sep jsString) *array[jsString] {a:=&array[jsString]{};if sep=="" {for i:=int32(0);i<s.length();i++ {a.Items=append(a.Items,s.charAt(i))};return a};for {i:=s.indexOf(sep);if i<0 {a.Items=append(a.Items,s);return a};a.Items=append(a.Items,s[:2*i]);s=s[2*i+int32(len(sep)):]}}
func jsWhitespace(c int32) bool {return c==9 || c==10 || c==11 || c==12 || c==13 || c==32 || c==160 || c==0x1680 || c>=0x2000 && c<=0x200a || c==0x2028 || c==0x2029 || c==0x202f || c==0x205f || c==0x3000 || c==0xfeff}
func (s jsString) trim() jsString {a,b:=int32(0),s.length();for a<b && jsWhitespace(s.charCodeAt(a)){a++};for b>a && jsWhitespace(s.charCodeAt(b-1)){b--};return s[2*a:2*b]}
func (s jsString) upper() jsString {out:=jsString("");for i:=int32(0);i<s.length();i++ {c:=s.charCodeAt(i);r:=rune(c);if c>=0xd800 && c<=0xdbff && i+1<s.length() {d:=s.charCodeAt(i+1);if d>=0xdc00 && d<=0xdfff {r=utf16.DecodeRune(rune(c),rune(d));i++}};if r>=0xd800 && r<=0xdfff {out+=s.charAt(i);continue};if v,ok:=upperExpansion[r];ok {out+=str(v)}else{out+=str(string(unicode.ToUpper(r)))}};return out}

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
	tag := uint8(2)
	switch v.(type) {
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
func integerArithmetic(a, b int64, op string, checked bool, bits int) int64 {
	x, y := big.NewInt(a), big.NewInt(b)
	switch op {
	case "+":
		x.Add(x, y)
	case "-":
		x.Sub(x, y)
	case "*":
		x.Mul(x, y)
	case "/":
		if b == 0 {
			panic(rangeFault{})
		}
		x.Quo(x, y)
	case "%":
		if b == 0 {
			panic(rangeFault{})
		}
		x.Rem(x, y)
	}
	if !x.IsInt64() {
		panic(rangeFault{})
	}
	v := x.Int64()
	if bits == 32 && (v < -2147483648 || v > 2147483647) {
		panic(rangeFault{})
	}
	if checked && (v < -9007199254740991 || v > 9007199254740991) {
		panic(rangeFault{})
	}
	return v
}
func negativeZero() float64 {return math.Copysign(0,-1)}
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

func (a *array[T]) push(v T) int32 { a.Items = append(a.Items, v); return int32(len(a.Items)) }
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
type orderedMap[K, V any] struct{ Entries []entry[K, V] }

func (m *orderedMap[K, V]) set(k K, v V) *orderedMap[K, V] {
	for i, e := range m.Entries {
		if equal(e.Key, k) {
			m.Entries[i].Value = v
			return m
		}
	}
	m.Entries = append(m.Entries, entry[K, V]{k, v})
	return m
}
func (m *orderedMap[K, V]) get(k K) optional[V] {
	for _, e := range m.Entries {
		if equal(e.Key, k) {
			return present(e.Value)
		}
	}
	return optional[V]{}
}
func (m *orderedMap[K, V]) has(k K) bool { return m.get(k).Has }
func (m *orderedMap[K, V]) keys() *array[K] {
	a := &array[K]{}
	for _, e := range m.Entries {
		a.Items = append(a.Items, e.Key)
	}
	return a
}

func (s *orderedSet[T]) fromArray(a *array[T]) *orderedSet[T] {for _,v:=range a.Items {s.add(v)};return s}

type orderedSet[T any] struct{ Items []T }

func (s *orderedSet[T]) has(v T) bool {
	for _, x := range s.Items {
		if equal(x, v) {
			return true
		}
	}
	return false
}
func (s *orderedSet[T]) add(v T) *orderedSet[T] {
	if !s.has(v) {
		s.Items = append(s.Items, v)
	}
	return s
}
func (s *orderedSet[T]) values() *array[T] { return &array[T]{Items: append([]T(nil), s.Items...)} }
`
