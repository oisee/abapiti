package golang

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func externGoType(t hir.Type) string {
	switch t.Kind {
	case hir.String:
		return "string"
	case hir.Bool:
		return "bool"
	case hir.Number:
		return "float64"
	case hir.Array:
		if t.Args[0].Kind == hir.String {
			return "[]string"
		}
		return "[]struct{ F0 string; F1 bool }"
	case hir.Optional:
		return "string, bool"
	}
	return ""
}
func (b *body) externCall(x *hir.Expr) string {
	e := b.e
	if e.externImports == nil {
		e.externImports = map[string]string{}
	}
	alias := e.externImports[x.Owner]
	if alias == "" {
		alias = fmt.Sprintf("externPkg%d", len(e.externImports))
		e.externImports[x.Owner] = alias
	}
	adapterIndex := len(e.externAdapters)
	e.externAdapters = append(e.externAdapters, "")
	name := fmt.Sprintf("externBridge%d", adapterIndex)
	ps, callArgs, decls, actual := []string{}, []string{}, []string{}, []string{}
	for i, p := range x.Extern.Params {
		g := externGoType(p)
		arg := fmt.Sprintf("a%d", i)
		if x.Extern.Rest && i == len(x.Extern.Params)-1 {
			g = "...string"
			callArgs = append(callArgs, arg+"...")
		} else {
			callArgs = append(callArgs, arg)
		}
		ps = append(ps, g)
		decls = append(decls, arg+" "+g)
	}
	for i, a := range x.Args {
		v := b.externToGo(a)
		if x.Extern.Spread && i == len(x.Args)-1 {
			v += "..."
		}
		actual = append(actual, v)
	}
	if x.Extern.Spread {
		min := len(x.Extern.Params) - 1
		last := len(actual) - 1
		spread := strings.TrimSuffix(actual[last], "...")
		actual = append(actual[:min], "append([]string{"+strings.Join(actual[min:last], ",")+"},"+spread+"...)...")
	}
	result := externGoType(x.Type)
	plain, withError := "", "error"
	if result != "" {
		plain = "(" + result + ")"
		withError = "(" + result + ", error)"
	}
	signature := "func(" + strings.Join(ps, ",") + ")"
	ret := e.result(x.Type)
	s := fmt.Sprintf("func %s[F interface{ %s%s | %s%s }](f F", name, signature, plain, signature, withError)
	if len(decls) > 0 {
		s += "," + strings.Join(decls, ",")
	}
	s += ")" + ret + " {\n"
	// F's closed type set enforces the complete host ABI at the call site.
	s += "switch fn := any(f).(type) {\n"
	assign := ""
	switch x.Type.Kind {
	case hir.Void:
	case hir.Optional:
		assign = "v, ok := "
	default:
		assign = "v := "
	}
	convert := b.externFromGo(x.Type, "v")
	s += "case " + signature + plain + ":\n" + assign + "fn(" + strings.Join(callArgs, ",") + ")\n" + convert + "\n"
	assign = "err := "
	switch x.Type.Kind {
	case hir.Void:
	case hir.Optional:
		assign = "v, ok, err := "
	default:
		assign = "v, err := "
	}
	e.usesTypedCatch = true
	errType := hir.Ref("builtin.Error")
	throw := fmt.Sprintf("panic(payload[%s]{Type:%q,Value:%s(present(str(err.Error())))})", e.typ(errType), errType.String(), e.name("new.builtin.Error"))
	s += "case " + signature + withError + ":\n" + assign + "fn(" + strings.Join(callArgs, ",") + ")\nif err != nil {" + throw + "}\n" + convert + "\n}\npanic(newTrap(\"extern adapter type\"))\n}\n"
	s += "var " + name + "Func = " + alias + "." + x.Name + "\n"
	e.externAdapters[adapterIndex] = s
	call := name + "(" + name + "Func"
	if len(actual) > 0 {
		call += "," + strings.Join(actual, ",")
	}
	call += ")"
	if x.Type.Kind == hir.Void {
		b.line("%s", call)
		return ""
	}
	return b.temp(x.Type, call)
}
func (b *body) externToGo(x *hir.Expr) string {
	value := b.expr(x)
	switch x.Type.Kind {
	case hir.String:
		return value + ".String()"
	case hir.Array:
		t := x.Type
		el := t.Args[0]
		convert := "v.String()"
		if el.Kind != hir.String {
			convert = "struct{F0 string;F1 bool}{v.(" + b.e.typ(el) + ")." + b.e.member("f0") + ".String(),v.(" + b.e.typ(el) + ")." + b.e.member("f1") + "}"
		}
		return fmt.Sprintf("func(a %s) %s { out:=make(%s,0,len(a.Items));for _,v:=range a.Items {out=append(out,%s)};return out }(%s)", b.e.typ(t), externGoType(t), externGoType(t), convert, value)
	}
	return value
}
func (b *body) externFromGo(t hir.Type, value string) string {
	switch t.Kind {
	case hir.Void:
		return "return"
	case hir.String:
		return "return str(" + value + ")"
	case hir.Optional:
		return "if ok {return present(str(" + value + "))};return optional[jsString]{}"
	case hir.Array:
		el := t.Args[0]
		convert := "str(item)"
		if el.Kind != hir.String {
			convert = "&" + b.e.obj(el.Name) + "{" + b.e.member("f0") + ":str(item.F0)," + b.e.member("f1") + ":item.F1}"
		}
		return "out := &array[" + b.e.typ(arrayStorage(t).Args[0]) + "]{};for _,item:=range " + value + " {out.Items=append(out.Items," + convert + ")};return out"
	}
	return "return " + value
}
func (e *emitter) externSource(pkg string) string {
	if len(e.externAdapters) == 0 {
		return ""
	}
	imports := make([]string, len(e.externImports))
	for path, alias := range e.externImports {
		var i int
		fmt.Sscanf(alias, "externPkg%d", &i)
		imports[i] = alias + " " + strconv.Quote(path)
	}
	return "package " + pkg + "\nimport (" + strings.Join(imports, ";") + ")\n" + strings.Join(e.externAdapters, "\n")
}
