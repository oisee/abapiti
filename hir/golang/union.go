package golang

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Validators read the live object's actual UTF-16 discriminant on every use.
// They never cache a tag or copy storage, and require the payload's own fields.
func (e *emitter) unionValidator(family, alternative string) string {
	name := e.name("union.check." + family + "/" + alternative)
	if e.liveViews == nil {
		e.liveViews = map[string]bool{}
	}
	if e.liveViews[name] {
		return name
	}
	e.liveViews[name] = true
	u := e.p.Union(family)
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	line("func %s(d *dynamic) *dynamic {", name)
	line("if d==nil {panic(newRangeFault())}")
	line("if d.Tag!=tagObject {if d.Tag!=2 {panic(newRangeFault())};if _,ok:=d.Value.(interface{structuralRecord()});!ok {panic(newRangeFault())}}")
	line("tag:=%s(d);if tag==nil || tag.Tag!=1 {panic(newRangeFault())};value:=dynString(tag)", e.unionRawTag(u.Discriminant))
	for _, a := range u.Alternatives {
		if alternative != "" && a.ID != alternative {
			continue
		}
		var conditions []string
		for _, tag := range a.Tags {
			conditions = append(conditions, fmt.Sprintf("value==str(%q)", tag))
		}
		line("if %s {", strings.Join(conditions, " || "))
		line("valid:=true")
		line("keys:=dynamicKeys(d);_ = keys")
		for _, f := range e.classBy(a.Payload.Name).Fields {
			if f.Type.Kind == hir.Optional {
				continue
			}
			line("{found:=false;for _,key:=range keys.Items {if key==str(%q) {found=true;break}};if !found {valid=false}}", f.Name)
		}
		for _, f := range e.classBy(a.Payload.Name).Fields {
			ft := f.Type
			optional := ft.Kind == hir.Optional
			if optional {
				ft = ft.Args[0]
			}
			condition := ""
			switch ft.Kind {
			case hir.String:
				condition = "v!=nil && v.Tag==1"
			case hir.Number, hir.I32, hir.I64:
				condition = "v!=nil && (v.Tag==4 || v.Tag==5 || v.Tag==6)"
			case hir.Bool:
				condition = "v!=nil && v.Tag==3"
			case hir.Array:
				condition = "isArray(v)"
			case hir.ClassRef, hir.InterfaceRef, hir.UnionRef:
				condition = refFieldCondition(ft)
			case hir.OrderedMap, hir.OrderedSet:
				condition = "v!=nil && v.Tag==2"
			}
			if condition != "" {
				if optional {
					condition = "v==nil || (" + condition + ")"
				}
				line("{v:=%s(d);if !(%s) {valid=false}}", e.unionRawTag(f.Name), condition)
			}
		}
		for _, f := range a.Literals {
			var domains []string
			for _, value := range f.Values {
				domains = append(domains, fmt.Sprintf("dynString(v)==str(%q)", value))
			}
			line("{v:=%s(d);if v==nil || v.Tag!=1 {valid=false} else if !(%s) {valid=false}}", e.unionRawTag(f.Field), strings.Join(domains, " || "))
		}
		line("if valid {return d}}")
	}
	line("panic(newRangeFault())}")
	e.extra.WriteString(b.String())
	return name
}

// Every JS object is an object: parsed arrays and class values too, the
// carriers dynObject bridges on reads. Other reference fields stay strict.
func refFieldCondition(t hir.Type) string {
	if t.Kind == hir.Optional {
		t = t.Args[0]
	}
	if t.Kind == hir.ClassRef && t.Name == hir.RootObject {
		return "v!=nil && (v.Tag==2 || v.Tag==tagObject || v.Tag==tagArray || v.Tag==10)"
	}
	return "v!=nil && (v.Tag==2 || v.Tag==tagObject)"
}

// Guarded payload getters must not recursively call themselves when checking
// the discriminant. Native records read their actual slot here; live views
// follow the original source. No tag or proof is cached on a view.
func (e *emitter) unionRawTag(property string) string {
	name := e.name("union.rawTag." + property)
	if e.liveViews == nil {
		e.liveViews = map[string]bool{}
	}
	if e.liveViews[name] {
		return name
	}
	e.liveViews[name] = true
	classes := map[string]bool{}
	for _, u := range e.p.Unions {
		for _, a := range u.Alternatives {
			classes[a.Payload.Name] = true
		}
	}
	var names []string
	for class := range classes {
		names = append(names, class)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "func %s(d *dynamic) *dynamic {if d==nil {panic(newRangeFault())};if d.Tag==2 {switch v:=d.Value.(type){\n", name)
	for _, class := range names {
		c := e.classBy(class)
		for _, f := range c.Fields {
			if f.Name == property {
				value := "box(v." + e.member(property) + ")"
				if f.Type.Kind == hir.Optional && !f.Type.Args[0].IsRef() {
					value = "func() *dynamic {if !v." + e.member(property) + ".Has {return nil};return box(v." + e.member(property) + ".Value)}()"
				}
				fmt.Fprintf(&b, "case %s: if v==nil {panic(newRangeFault())};if v.viewSource!=nil {return %s(v.viewSource)};return %s\n", e.typ(hir.Ref(class)), name, value)
			}
		}
	}
	fmt.Fprintf(&b, "}};return d.get(str(%q))}\n", property)
	e.extra.WriteString(b.String())
	return name
}

func (e *emitter) unionPayloadGuard(class string) string {
	for _, u := range e.p.Unions {
		for _, a := range u.Alternatives {
			if a.Payload.Name == class {
				return e.unionValidator(u.Name, a.ID) + "(box(self));"
			}
		}
	}
	return ""
}

func (b *body) unionOperation(x *hir.Expr, receiver string) string {
	e := b.e
	switch x.Op {
	case "union.admit":
		return e.unionValidator(x.Owner, x.Name) + "(box(" + receiver + "))"
	case "union.check", "union.widen":
		return e.unionValidator(x.Type.Name, "") + "(" + receiver + ")"
	case "union.project":
		if x.Type.Kind == hir.UnionRef {
			return e.unionValidator(x.Type.Name, "") + "(" + receiver + ")"
		}
		return e.liveView(x.Type) + "(" + e.unionValidator(x.Owner, x.Name) + "(" + receiver + "))"
	case "union.tag", "union.read":
		checked := e.unionValidator(x.X.Type.Name, "") + "(" + receiver + ")"
		return e.viewValue(x.Type, checked+".get(str("+fmt.Sprintf("%q", x.Name)+"))")
	}
	return ""
}
