package golang

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) materializer(t hir.Type) string {
	name := e.name("materialize." + t.String())
	if e.materializers == nil {
		e.materializers = map[string]bool{}
	}
	if e.materializers[name] {
		return name
	}
	e.materializers[name] = true
	var b strings.Builder
	line := func(f string, args ...any) { fmt.Fprintf(&b, f+"\n", args...) }
	call := func(t hir.Type, v string) string { return e.materializer(t) + "(" + v + ")" }
	line("func %s(d *dynamic) %s {", name, e.typ(t))
	switch t.Kind {
	case hir.Dynamic:
		line("return d")
	case hir.Optional:
		line("if d==nil {var z %s;return z}", e.typ(t))
		v := call(t.Args[0], "d")
		if !t.Args[0].IsRef() {
			v = "present(" + v + ")"
		}
		line("return %s", v)
	case hir.String:
		line("return dynString(d)")
	case hir.Bool:
		line("return dynBoolean(d)")
	case hir.Number:
		line("return dynNumber(d)")
	case hir.Array:
		line("if d==nil {return nil};if d.Tag!=tagArray {jsonFail()}")
		line("out:=&%s{};for _,item:=range d.Value.(*array[*dynamic]).Items {out.push(%s)};return out", strings.TrimPrefix(e.typ(t), "*"), call(t.Args[0], "item"))
	case hir.ClassRef:
		c := e.classBy(t.Name)
		if c == nil || c.Super != "" || c.Abstract || len(c.Methods) > 0 || c.Ctor == nil || len(c.Ctor.Params) != len(c.Fields) {
			e.unsupported(hir.Node{}, "materialization requires a data shape: "+t.String())
			return name
		}
		line("if d==nil {return nil};if d.Tag!=tagObject {jsonFail()}")
		var args []string
		for i, f := range c.Fields {
			if f.Static || c.Ctor.Params[i].Name != f.Name || !c.Ctor.Params[i].Type.Equal(f.Type) {
				e.unsupported(c.Node, "materialization requires field constructor ABI: "+t.String())
				return name
			}
			args = append(args, call(f.Type, fmt.Sprintf("d.get(str(%q))", f.Name)))
		}
		line("out:=%s(%s);out.%s().source=d;return out", e.name("new."+c.Name), strings.Join(args, ","), e.getter(c.Name))
	default:
		e.unsupported(hir.Node{}, "materialization of "+t.String())
		return name
	}
	line("}")
	e.extra.WriteString(b.String())
	return name
}

// Discover the actual materialization roots before declaring class storage.
// Recursive materializer construction marks every nested data shape. Ordinary
// instances of those shapes retain source identity; shapes never materialized
// by this program need no permanently nil dynamic-source pointer.
func (e *emitter) prepareMaterializers() {
	visit := func(x, _ *hir.Expr) {
		if x.Kind == hir.RuntimeOp && x.Op == "dynamic.materialize" {
			e.materializer(x.Type)
		}
	}
	for _, c := range e.p.Classes {
		for _, m := range c.Methods {
			walkStmt(m.Body, func(*hir.Stmt) {}, visit)
		}
		if c.Ctor != nil {
			walkStmt(c.Ctor.Body, func(*hir.Stmt) {}, visit)
		}
	}
}
