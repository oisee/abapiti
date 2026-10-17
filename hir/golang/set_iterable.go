package golang

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

// Keep the deployed runtime bytes unchanged unless a Set iterable is used.
func (e *emitter) setIterationRuntime(runtime string) string {
	if _, ok := hir.UsedRuntimeOps(e.p)["set.iterationValues"]; !ok {
		return runtime
	}
	patches := [][2]string{
		{"type orderedSet[T comparable] struct {", "type orderedSet[T comparable] struct { iteration *array[T];"},
		{"s.index[key]=len(s.Items);s.Items=append(s.Items,collectionValue(v));", "s.index[key]=len(s.Items);s.Items=append(s.Items,collectionValue(v));if s.iteration!=nil {s.iteration.push(collectionValue(v))};"},
		{"delete(s.index,key)\n", "delete(s.index,key)\n if s.iteration!=nil {for j:=len(s.iteration.Items)-1;j>=0;j-- {if s.iteration.has(int32(j)) && setKey(s.iteration.Items[j])==key {s.iteration.delete(int32(j));break}}}\n"},
	}
	for _, patch := range patches {
		if !strings.Contains(runtime, patch[0]) {
			panic("Set iterator runtime patch target moved")
		}
		runtime = strings.Replace(runtime, patch[0], patch[1], 1)
	}
	return runtime + `
func (s *orderedSet[T]) iterationValues() *array[T] {
 if s.iteration==nil {s.iteration=&array[T]{Items:append([]T(nil),s.Items...)}}
 return s.iteration
}
`
}

func setIterationFastRuntime(runtime string) string {
	patches := [][2]string{
		{"type fastOrderedSet[T comparable] struct {", "type fastOrderedSet[T comparable] struct {iteration *array[T];"},
		{"s.Items=append(s.Items,v)", "s.Items=append(s.Items,v);if s.iteration!=nil {s.iteration.push(v)}"},
		{"delete(s.index,v)", "delete(s.index,v);if s.iteration!=nil {for j:=len(s.iteration.Items)-1;j>=0;j-- {if s.iteration.has(int32(j)) && s.iteration.Items[j]==v {s.iteration.delete(int32(j));break}}}"},
	}
	for _, patch := range patches {
		if !strings.Contains(runtime, patch[0]) {
			panic("fast Set iterator runtime patch target moved")
		}
		runtime = strings.Replace(runtime, patch[0], patch[1], 1)
	}
	return runtime
}
