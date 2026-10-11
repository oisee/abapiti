package golang

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"strconv"
	"strings"
)

// Missing arguments are undefined; excess arguments have already been evaluated
// by the caller and are ignored. Unsupported storage domains refuse explicitly.
func (e *emitter) argumentFactory(c *hir.Class, ctor *hir.Method) string {
	refuse := `func(a *array[any]) any {panic(newTrap(` + strconv.Quote("dynamic constructor storage is unsupported: "+c.Name) + `))}`
	if c.Abstract {
		return refuse
	}
	var convert func(hir.Type, string) (string, bool)
	convert = func(t hir.Type, v string) (string, bool) {
		switch t.Kind {
		case hir.String:
			return "dynString(" + v + ")", true
		case hir.Bool:
			return "dynBoolean(" + v + ")", true
		case hir.Number:
			return "dynNumber(" + v + ")", true
		case hir.Dynamic:
			return v, true
		case hir.ClassValue:
			return "dynClass(" + v + ")", true
		case hir.ClassRef, hir.InterfaceRef:
			return "castRef[" + e.typ(t) + "](dynRef(" + v + "))", true
		case hir.Optional:
			value, ok := convert(t.Args[0], v)
			if !ok {
				return "", false
			}
			if !t.Args[0].IsRef() {
				value = "present(" + value + ")"
			}
			return "func() " + e.typ(t) + " {if " + v + "==nil {var z " + e.typ(t) + ";return z};return " + value + "}()", true
		}
		return "", false
	}
	var args []string
	if ctor != nil {
		for i, p := range ctor.Params {
			if p.Variadic {
				return refuse
			}
			v, ok := convert(p.Type, fmt.Sprintf("arg(%d)", i))
			if !ok {
				return refuse
			}
			args = append(args, v)
		}
	}
	return `func(a *array[any]) any {arg:=func(i int32)*dynamic {v:=a.get(i);if !v.Has {return nil};return box(v.Value)};_ = arg;return ` + e.name("new."+c.Name) + "(" + strings.Join(args, ",") + `)} `
}
