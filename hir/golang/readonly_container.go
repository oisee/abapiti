package golang

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) readonlyContainerView(t hir.Type, d string) string {
	switch t.Kind {
	case hir.Array:
		return e.viewValue(t, d)
	case hir.OrderedSet:
		st := e.typ(t)
		storage := e.typ(t.Args[0])
		decode := e.viewValue(t.Args[0], "v")
		if e.referenceSets && t.Args[0].IsRef() {
			storage = "any"
			decode = "box(" + decode + ")"
		}
		return fmt.Sprintf(`func(d *dynamic) %s {if d==nil{return nil};src,ok:=d.Value.(readonlySetStorage);if !ok {panic(newRangeFault())};return &%s{readProxy:&setReadProxy[%s]{source:d,storage:src,decode:func(v *dynamic)%s{return %s}}}}(%s)`, st, strings.TrimPrefix(st, "*"), storage, storage, decode, d)
	case hir.OrderedMap:
		mt, kt, vt := e.typ(t), e.typ(t.Args[0]), e.typ(t.Args[1])
		return fmt.Sprintf(`func(d *dynamic) %s {
 if d==nil {return nil};src,ok:=d.Value.(readonlyMapStorage);if !ok {panic(newRangeFault())}
 return &%s{proxy:&mapProxy[%s,%s]{source:d,
 get:func(k %s) optional[%s] {v,has:=src.identityGet(box(k));if !has{return optional[%s]{}};return present(%s)},
 has:func(k %s)bool{return src.identityHas(box(k))},
 keys:func()*array[%s]{out:=&array[%s]{};for _,k:=range src.readonlyKeys().Items {out.push(%s)};return out},
 size:src.mapSize,insertions:src.mapInsertions,
 set:func(%s,%s){panic(newTrap("readonly Map view cannot write"))},remove:func(%s)bool{panic(newTrap("readonly Map view cannot delete"))}}}
 }(%s)`, mt, strings.TrimPrefix(mt, "*"), kt, vt, kt, vt, vt, e.viewValue(t.Args[1], "v"), kt, kt, kt, e.viewValue(t.Args[0], "k"), kt, vt, kt, d)
	}
	panic("not a readonly container")
}

// Identity queries accept the original boxed carrier. They never require a
// nominal cast of a foreign query object to the collection's key/value layout.
const readonlyCollectionSource = `
func identityQueryKey[K comparable](query *dynamic)(K,bool) {
 var z K
 if _,ok:=any(z).(*dynamic);ok {return any(query).(K),true}
 if query==nil {return z,nilRef(z)}
 key,ok:=query.Value.(K);return key,ok
}
type readonlySetStorage interface {identityHas(*dynamic)bool;readonlySetValues()*array[*dynamic];readonlySetSize()int32}
type setReadProxy[T comparable] struct {source *dynamic;storage readonlySetStorage;decode func(*dynamic)T}
func (s *orderedSet[T]) dynamicSource()*dynamic {if s==nil || s.readProxy==nil {return nil};return s.readProxy.source}
func (s *orderedSet[T]) readonlySetSize()int32 {if s.readProxy!=nil {return s.readProxy.storage.readonlySetSize()};return int32(len(s.Items))}
func (s *orderedSet[T]) readonlySetValues()*array[*dynamic] {if s.readProxy!=nil {return s.readProxy.storage.readonlySetValues()};out:=&array[*dynamic]{};for _,v:=range s.Items {out.push(box(v))};return out}
type identityCollection interface {identityHas(*dynamic)bool}
type readonlyMapStorage interface {identityCollection;identityGet(*dynamic)(*dynamic,bool);readonlyKeys()*array[*dynamic];mapSize()int32;mapInsertions()int32}
func (m *orderedMap[K,V]) identityGet(query *dynamic)(*dynamic,bool) {
 if k,ok:=identityQueryKey[K](query);ok {v:=m.get(k);return mapValueBox(v.Value),v.Has};for _,k:=range m.keys().Items {if setKey(box(k))==setKey(query) {v:=m.get(k);return mapValueBox(v.Value),v.Has}};return nil,false
}
func (m *orderedMap[K,V]) identityHas(query *dynamic)bool {if k,ok:=identityQueryKey[K](query);ok {return m.has(k)};for _,k:=range m.keys().Items {if setKey(box(k))==setKey(query) {return true}};return false}
func (m *orderedMap[K,V]) readonlyKeys()*array[*dynamic] {out:=&array[*dynamic]{};for _,k:=range m.keys().Items {out.push(box(k))};return out}
func (s *orderedSet[T]) identityHas(query *dynamic)bool {if s.readProxy!=nil {return s.readProxy.storage.identityHas(query)};if v,ok:=identityQueryKey[T](query);ok {return s.has(v)};for _,v:=range s.values().Items {if setKey(box(v))==setKey(query) {return true}};return false}
`

func readonlySetRuntime(source string) string {
	source = mustReplace(source, "type orderedSet[T comparable] struct {", "type orderedSet[T comparable] struct { readProxy *setReadProxy[T];", "readonly Set read proxy")
	for signature, guard := range map[string]string{
		"has(v T) bool":           "if s.readProxy!=nil {return s.readProxy.storage.identityHas(box(v))}",
		"add(v T) *orderedSet[T]": "if s.readProxy!=nil {panic(newTrap(\"readonly Set view cannot write\"))}",
		"delete(v T) bool":        "if s.readProxy!=nil {panic(newTrap(\"readonly Set view cannot delete\"))}",
		"values() *array[T]":      "if s.readProxy!=nil {out:=&array[T]{};for _,v:=range s.readProxy.storage.readonlySetValues().Items {out.push(s.readProxy.decode(v))};return out}",
	} {
		needle := "func (s *orderedSet[T]) " + signature + " {"
		if !strings.Contains(source, needle) {
			panic("readonly Set patch target moved: " + signature)
		}
		source = strings.Replace(source, needle, needle+guard+";", 1)
	}

	needle := "func (s *orderedSet[T]) iterationValues() *array[T] {"
	if strings.Contains(source, needle) {
		source = strings.Replace(source, needle, needle+`if s.readProxy!=nil {src:=s.readProxy.storage.(interface{readonlySetIteration()*array[*dynamic]});return liveReadonlyArrayView[T](box(src.readonlySetIteration()),s.readProxy.decode)};`, 1)
		source += `
func (s *orderedSet[T]) readonlySetIteration()*array[*dynamic] {return liveReadonlyArrayView[*dynamic](box(s.iterationValues()),func(d *dynamic)*dynamic{return d})}
`
	}
	return source
}
