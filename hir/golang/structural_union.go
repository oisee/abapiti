package golang

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// The common interface is a calling ABI, not a membership proof. Decode only
// declared structural alternatives and keep each wrapper on its source graph.
func (e *emitter) structuralUnion(t hir.Type) string {
	return e.structuralUnionProjection(t, "")
}

func (e *emitter) structuralUnionProjection(t hir.Type, member string) string {
	members := e.p.StructuralUnion(t.Name)
	if len(members) == 0 {
		return ""
	}
	allMembers := members
	result := t
	if member != "" {
		var selected []hir.StructuralMember
		for _, m := range members {
			if m.Type.Name == member {
				selected = append(selected, m)
				result = m.Type
			}
		}
		if len(selected) != 1 {
			return ""
		}
		members = selected
	}
	active := member != ""
	for _, m := range members {
		active = active || e.liveClasses[m.Type.Name]
	}
	if !active {
		return ""
	}
	key := "view.union." + t.Name
	if member != "" {
		key += "/" + member
	}
	name := e.name(key)
	if e.liveViews == nil {
		e.liveViews = map[string]bool{}
	}
	if e.liveViews[name] {
		return name
	}
	e.liveViews[name] = true
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	check := name
	if member != "" {
		check = e.name(key + ".check")
		line("func %s(d *dynamic) *dynamic {", check)
	} else {
		line("func %s(d *dynamic) %s {", name, e.typ(result))
	}
	line("if d==nil {panic(newRangeFault())}")
	line("if d.Tag!=tagObject {if d.Tag!=2 {panic(newRangeFault())};if _,ok:=d.Value.(interface{structuralRecord()});!ok {panic(newRangeFault())}}")
	// Dispatch native carriers by exact concrete type before the shape scan.
	// Live wrappers still validate their source graph through the existing scan.
	line("if d.Tag==2 {if source,ok:=d.Value.(interface{dynamicSource()*dynamic});!ok || source.dynamicSource()==nil {switch v:=d.Value.(type) {")
	for _, m := range allMembers {
		line("case *%s:", e.obj(m.Type.Name))
		if member != "" && m.Type.Name != member {
			line("panic(newRangeFault())")
			continue
		}
		for _, f := range m.Fields {
			if len(f.Domains) != 0 {
				line("{v:=d.get(str(%q));_ = v;if !(%s) {panic(newRangeFault())}}", f.Name, structuralFieldCondition(f))
			}
		}
		if member != "" {
			line("_ = v;return d")
		} else {
			line("return v")
		}
	}
	line("}}}")
	for _, m := range members {
		// Exact native values may carry callable or otherwise erased fields that
		// cannot be decoded from dynamic storage. Their concrete Go type proves
		// those field declarations, while every retained source-domain constraint
		// still has to be checked against the live value on each projection.
		line("{typed:=false;if d.Tag==2 {if _,ok:=d.Value.(%s);ok {typed=true}};var keys *array[jsString];if !typed {keys=dynamicKeys(d)};_ = keys;valid:=true", e.typ(m.Type))
		for _, f := range m.Fields {
			condition := structuralFieldCondition(f)
			if len(f.Domains) == 0 {
				if f.Optional {
					line("if !typed {if d.get(str(%q))!=nil {valid=false}}", f.Name)
				} else {
					line("if !typed {valid=false}")
				}
				continue
			}
			if !f.Optional {
				line("if !typed {found:=false;for _,key:=range keys.Items {if key==str(%q) {found=true;break}};if !found {valid=false}}", f.Name)
			}
			line("{v:=d.get(str(%q));_ = v;if !(%s) {valid=false}}", f.Name, condition)
		}
		if member != "" {
			line("if valid {return d}}")
		} else {
			line("if valid && d.Tag==2 {if v,ok:=d.Value.(%s);ok {return v}}", e.typ(m.Type))
			if e.liveClasses[m.Type.Name] {
				line("if valid {return %s(d)}}", e.liveView(m.Type))
			} else {
				line("if valid {panic(newRangeFault())}}")
			}
		}
	}
	line("panic(newRangeFault())}")
	if member != "" {
		line("func %s(d *dynamic) %s {d=%s(d);out:=%s(d)", name, e.typ(result), check, e.liveViewMode(result, true))
		for c := e.classBy(result.Name); c != nil; c = e.classBy(c.Super) {
			line("out.%s().viewCheck=func(source *dynamic){%s(source)}", e.getter(c.Name), check)
		}
		line("return out}")
	}
	e.extra.WriteString(b.String())
	return name
}

func structuralFieldCondition(f hir.StructuralField) string {
	var choices []string
	for _, d := range f.Domains {
		c := "false"
		switch d.Kind {
		case hir.Dynamic:
			c = "true"
		case "undefined":
			c = "v==nil"
		case "null":
			c = "v!=nil && v.Tag==tagNull"
		case hir.String:
			c = "v!=nil && v.Tag==1"
			if d.Exact {
				c += fmt.Sprintf(" && dynString(v)==str(%q)", d.Literal)
			}
		case hir.Number:
			c = "v!=nil && (v.Tag==4 || v.Tag==5 || v.Tag==6)"
			if d.Exact {
				c += " && dynNumber(v)==" + d.Literal
			}
		case hir.Bool:
			c = "v!=nil && v.Tag==3"
			if d.Exact {
				c += " && dynBoolean(v)==" + d.Literal
			}
		case hir.Array:
			c = "isArray(v)"
		case hir.ClassRef:
			c = "v!=nil && (v.Tag==2 || v.Tag==tagObject)"
		case hir.ClassValue:
			c = "v!=nil && v.Tag==10"
		}
		choices = append(choices, "("+c+")")
	}
	if f.Optional {
		choices = append(choices, "v==nil")
	}
	if len(choices) == 0 {
		return "false"
	}
	return strings.Join(choices, " || ")
}
