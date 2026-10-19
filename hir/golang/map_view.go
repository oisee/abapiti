package golang

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) usesMapViews() bool {
	if !e.mapViewsKnown {
		_, e.mapViews = hir.UsedRuntimeOps(e.p)["map.view"]
		e.mapViewsKnown = true
	}
	return e.mapViews
}

func (b *body) mapView(x *hir.Expr) string {
	e := b.e
	source := b.expr(x.X)
	key, to := e.typ(x.Type.Args[0]), e.typ(x.Type.Args[1])
	if x.Type.Args[0].Kind == hir.Dynamic && x.X.Type.Args[0].Kind == hir.String {
		// A nullable or erased query never coerces to a string Map key.
		// Delegate string queries to the original storage, keeping aliases live.
		return fmt.Sprintf(`func(src %s) %s {
 if src==nil {panic(newRangeFault())}
 return &%s{proxy:&mapProxy[*dynamic,%s]{source:box(src),
 get:func(k *dynamic) optional[%s] {if k==nil || k.Tag!=1 {return optional[%s]{}};return src.get(dynString(k))},
 set:func(k *dynamic,v %s){panic(newTrap("writes through a boxed string-key Map view are unsupported"))},
 has:func(k *dynamic)bool{return k!=nil && k.Tag==1 && src.has(dynString(k))},
 remove:func(k *dynamic)bool{return k!=nil && k.Tag==1 && src.delete(dynString(k))},
 keys:func()*array[*dynamic]{out:=&array[*dynamic]{};for _,k:=range src.keys().Items {out.push(box(k))};return out},
 size:src.mapSize,insertions:src.mapInsertions}}
 }(%s)`, e.typ(x.X.Type), e.typ(x.Type), strings.TrimPrefix(e.typ(x.Type), "*"), to, to, to, to, source)
	}
	decoded := e.viewValue(x.Type.Args[1], "box(v.Value)")
	encoded := e.viewValue(x.X.Type.Args[1], "box(v)")
	return fmt.Sprintf(`func(src %s) %s {
 if src==nil {panic(newRangeFault())}
 return &%s{proxy:&mapProxy[%s,%s]{source:box(src),
 get:func(k %s) optional[%s] {v:=src.get(k);if !v.Has {return optional[%s]{}};return present(%s)},
 set:func(k %s,v %s){src.set(k,%s)},has:src.has,remove:src.delete,keys:src.keys,size:src.mapSize,insertions:src.mapInsertions}}
 }(%s)`, e.typ(x.X.Type), e.typ(x.Type), strings.TrimPrefix(e.typ(x.Type), "*"), key, to, key, to, to, decoded, key, to, encoded, source)
}

// Proxy methods are installed only in programs with a structural Map join.
// Every access delegates to original storage; boxing unwraps the proxy so
// identity, keys, and subsequent writes through the original alias stay live.
func mapViewRuntime(source string) string {
	source = strings.Replace(source, "type orderedMap[K comparable, V any] struct {", "type orderedMap[K comparable, V any] struct { proxy *mapProxy[K,V];", 1)
	guards := map[string]string{
		"set(k K, v V) *orderedMap[K, V]": "if m.proxy!=nil {m.proxy.set(k,v);return m}",
		"get(k K) optional[V]":            "if m.proxy!=nil {return m.proxy.get(k)}",
		"has(k K) bool":                   "if m.proxy!=nil {return m.proxy.has(k)}",
		"delete(k K) bool":                "if m.proxy!=nil {return m.proxy.remove(k)}",
		"keys() *array[K]":                "if m.proxy!=nil {return m.proxy.keys()}",
		"values() *array[V]":              "if m.proxy!=nil {out:=&array[V]{};for _,k:=range m.keys().Items {out.push(m.get(k).Value)};return out}",
	}
	for method, guard := range guards {
		// Methods predate consistent generic receiver spacing.
		for _, recv := range []string{"func (m *orderedMap[K, V]) ", "func (m *orderedMap[K,V]) "} {
			old := recv + method + " {"
			if strings.Contains(source, old) {
				source = strings.Replace(source, old, old+guard+";", 1)
				break
			}
		}
	}
	insertions := "int32(len(m.Entries))"
	if strings.Contains(source, " inserts int32;") {
		insertions = "m.inserts"
	}
	return source + `
type mapProxy[K comparable,V any] struct {source *dynamic;get func(K)optional[V];set func(K,V);has func(K)bool;remove func(K)bool;keys func()*array[K];size func()int32;insertions func()int32}
func (m *orderedMap[K,V]) dynamicSource() *dynamic {if m==nil || m.proxy==nil {return nil};return m.proxy.source}
func (m *orderedMap[K,V]) mapSize() int32 {if m.proxy!=nil {return m.proxy.size()};return int32(len(m.Entries))}
func (m *orderedMap[K,V]) mapInsertions() int32 {if m.proxy!=nil {return m.proxy.insertions()};return ` + insertions + `}
`
}

func fastMapViewRuntime(source string) string {
	guards := map[string]string{
		"set(k K,v V) *fastOrderedMap[K,V]": "if m.proxy!=nil {m.proxy.set(k,v);return m}",
		"get(k K) optional[V]":              "if m.proxy!=nil {return m.proxy.get(k)}",
		"has(k K) bool":                     "if m.proxy!=nil {return m.proxy.has(k)}",
		"delete(k K) bool":                  "if m.proxy!=nil {return m.proxy.remove(k)}",
	}
	for method, guard := range guards {
		old := "func (m *fastOrderedMap[K,V]) " + method + " {"
		if !strings.Contains(source, old) {
			panic("map proxy patch target moved: " + method)
		}
		source = strings.Replace(source, old, old+guard+";", 1)
	}
	return source
}

func (e *emitter) usesStorageIdentity() bool { return e.usesMapViews() || len(e.liveClasses) > 0 }

// A live data/array view and its original object are the same JS key. Keep
// native fast indexes for programs without these views.
func storageIdentityRuntime(source string) string {
	source = strings.Replace(source, "struct {Value K; NaN bool}", "struct {Value K; NaN bool; Identity any}", 1)
	source = strings.Replace(source, "func collectionKey[K comparable](k K) collectionIndexKey[K] {", "func collectionKey[K comparable](k K) collectionIndexKey[K] {if !nilRef(k) {switch any(k).(type) {case jsString,bool,int32,int64,float64:default:if s,ok:=any(k).(interface{dynamicSource()*dynamic});ok && s.dynamicSource()!=nil {return collectionIndexKey[K]{Identity:s.dynamicSource().Value}};return collectionIndexKey[K]{Identity:any(k)}}};", 1)
	source = strings.Replace(source, "func setKey(v any) any {", "func setKey(v any) any {if s,ok:=v.(interface{dynamicSource()*dynamic});ok && s.dynamicSource()!=nil {return setKey(s.dynamicSource().Value)};", 1)
	return source
}
