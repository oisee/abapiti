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
		line("keys:=dynamicKeys(d);_ = keys")
		for _, f := range e.classBy(a.Payload.Name).Fields {
			if f.Type.Kind == hir.Optional {
				continue
			}
			line("{found:=false;for _,key:=range keys.Items {if key==str(%q) {found=true;break}};if !found {panic(newRangeFault())}}", f.Name)
		}
		line("return d}")
	}
	line("panic(newRangeFault())}")
	e.extra.WriteString(b.String())
	return name
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
			if f.Name == property && f.Type.Kind == hir.String {
				fmt.Fprintf(&b, "case %s: if v==nil {panic(newRangeFault())};if v.viewSource!=nil {return %s(v.viewSource)};return box(v.%s)\n", e.typ(hir.Ref(class)), name, e.member(property))
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
	case "union.widen":
		return e.unionValidator(x.Type.Name, "") + "(" + receiver + ")"
	case "union.project":
		return e.liveView(x.Type) + "(" + e.unionValidator(x.Owner, x.Name) + "(" + receiver + "))"
	case "union.tag", "union.read":
		checked := e.unionValidator(x.X.Type.Name, "") + "(" + receiver + ")"
		return e.viewValue(x.Type, checked+".get(str("+fmt.Sprintf("%q", x.Name)+"))")
	}
	return ""
}
