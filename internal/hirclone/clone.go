// Package hirclone provides lossless HIR copies for independent test runs.
// Unlike gob, it preserves nil entries in statement lists and literal types.
package hirclone

import (
	"reflect"

	"github.com/oisee/abapiti/hir"
)

func Clone(p *hir.Program) *hir.Program {
	seen := map[any]reflect.Value{}
	var copy func(reflect.Value) reflect.Value
	copy = func(v reflect.Value) reflect.Value {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				return reflect.Zero(v.Type())
			}
			key := v.Interface()
			if prior, ok := seen[key]; ok {
				return prior
			}
			out := reflect.New(v.Type().Elem())
			seen[key] = out
			out.Elem().Set(copy(v.Elem()))
			return out
		case reflect.Interface:
			if v.IsNil() {
				return reflect.Zero(v.Type())
			}
			out := reflect.New(v.Type()).Elem()
			out.Set(copy(v.Elem()))
			return out
		case reflect.Slice:
			if v.IsNil() {
				return reflect.Zero(v.Type())
			}
			out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
			for i := 0; i < v.Len(); i++ {
				out.Index(i).Set(copy(v.Index(i)))
			}
			return out
		case reflect.Struct:
			out := reflect.New(v.Type()).Elem()
			for i := 0; i < v.NumField(); i++ {
				out.Field(i).Set(copy(v.Field(i)))
			}
			return out
		default:
			return v
		}
	}
	out, ok := copy(reflect.ValueOf(p)).Interface().(*hir.Program)
	if !ok {
		panic("hirclone: copy changed program type")
	}
	return out
}
