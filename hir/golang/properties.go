package golang

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// propertyLookup emits metadata only for an operation's nominal receiver.
// Fields initialized by base constructors are still own instance properties;
// class methods and Object.prototype members are prototype properties.
func (e *emitter) propertyLookup(t hir.Type) string {
	name := e.name("property.lookup." + t.Name)
	if e.propertyLookups == nil {
		e.propertyLookups = map[string]bool{}
	}
	if e.propertyLookups[name] {
		return name
	}
	e.propertyLookups[name] = true
	if t.Name == hir.RootObject {
		return e.structuralPropertyLookup(name)
	}
	c := e.classBy(t.Name)
	type entry struct {
		name, value string
		own         bool
	}
	var entries []entry
	seen := map[string]bool{}
	for base := c; base != nil; base = e.classBy(base.Super) {
		for _, f := range base.Fields {
			if f.Static || seen[f.Name] {
				continue
			}
			seen[f.Name] = true
			value := "self." + e.getter(base.Name) + "()." + e.member(f.Name)
			if f.Type.Kind == hir.Optional && !f.Type.Args[0].IsRef() {
				value = "func() *dynamic {v:=" + value + ";if !v.Has {return nil};return box(v.Value)}()"
			} else {
				value = "box(" + value + ")"
			}
			entries = append(entries, entry{f.Name, value, true})
		}
	}
	// Prototype function values are opaque stable identities. Calling a dynamic
	// function is outside the HIR callable ABI and remains refused by tsfront.
	var b strings.Builder
	line := func(f string, args ...any) { fmt.Fprintf(&b, f+"\n", args...) }
	addPrototype := func(key, owner string, tag int) {
		if seen[key] {
			return
		}
		seen[key] = true
		token := e.name("property.value." + owner + "." + key)
		if !e.propertyLookups[token] {
			e.propertyLookups[token] = true
			line("var %s = &dynamic{Tag:%d,Value:&struct{owner,key string}{%q,%q}}", token, tag, owner, key)
		}
		entries = append(entries, entry{key, token, false})
	}
	for base := c; base != nil; base = e.classBy(base.Super) {
		for _, m := range base.Methods {
			if !m.Static {
				addPrototype(m.Name, base.Name, 10)
			}
		}
	}
	if !seen["constructor"] {
		seen["constructor"] = true
		entries = append(entries, entry{"constructor", "box(&" + e.name("descriptor."+c.Name) + ")", false})
	}
	addPrototype("__proto__", c.Name, 2)
	for _, key := range []string{"hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__"} {
		addPrototype(key, "Object.prototype", 10)
	}
	table := e.name("property.table." + t.Name)
	line("var %s = map[jsString]struct{slot int;own bool}{", table)
	for i, p := range entries {
		line("str(%q):{%d,%t},", p.name, i, p.own)
	}
	line("}")
	line("func %s(self %s,key jsString,read bool) struct{value *dynamic;own,exists bool} {", name, e.typ(t))
	line("if nilRef(self) {panic(newRangeFault())}")
	// A data view's nominal slots are empty: its original record owns the
	// properties and receives all writes. Use that record for both values and
	// presence, including keys outside the nominal config layout.
	if e.liveClasses[c.Name] {
		lookup := e.propertyLookup(hir.Ref(hir.RootObject))
		line("if self.%s().viewSource!=nil {return %s(self,key,read)}", e.getter(c.Name), lookup)
	}
	// The static receiver can hold any lowered subclass. Its actual class
	// selects the corresponding table, rather than losing subclass fields.
	for _, derived := range e.p.Classes {
		if derived.Name == c.Name {
			continue
		}
		inherits := false
		for base := e.classBy(derived.Super); base != nil; base = e.classBy(base.Super) {
			if base.Name == c.Name {
				inherits = true
				break
			}
		}
		if inherits {
			lookup := e.propertyLookup(hir.Ref(derived.Name))
			line("if classOf(self)==&%s {return %s(castRef[%s](self),key,read)}", e.name("descriptor."+derived.Name), lookup, e.typ(hir.Ref(derived.Name)))
		}
	}

	if e.recordDeletion {
		line("if self.%s().deletedKeys[key] {return struct{value *dynamic;own,exists bool}{}}", e.getter(c.Name))
	}
	line("p,ok:=%s[key];var value *dynamic;if !read {return struct{value *dynamic;own,exists bool}{nil,p.own,ok}};switch p.slot {", table)
	for i, p := range entries {
		line("case %d:if ok {value=%s}", i, p.value)
	}
	line("}")
	line("return struct{value *dynamic;own,exists bool}{value,p.own,ok}")
	line("}")
	e.extra.WriteString(b.String())
	return name
}

// The object root has no nominal field table. Reuse original structural own-key
// metadata (including live views) and the ordinary Object.prototype boundary.
// Native prototype-bearing objects and records without presence proof refuse.
func (e *emitter) structuralPropertyLookup(name string) string {
	var b strings.Builder
	line := func(f string, args ...any) { fmt.Fprintf(&b, f+"\n", args...) }
	line("func %s(self any,key jsString,read bool) struct{value *dynamic;own,exists bool} {", name)
	line("if nilRef(self) {panic(newRangeFault())};d:=box(self)")
	line("if d==nil {panic(newRangeFault())};if d.Tag!=tagObject {")
	line("allowed:=false;switch classOf(d.Value) {")
	for _, c := range e.p.Classes {
		if c.Structural {
			line("case &%s:allowed=true", e.name("descriptor."+c.Name))
		}
	}
	line("};if !allowed {panic(newTrap(\"erased property lookup requires original structural storage\"))}}")
	line("keys:=dynamicKeys(d);own:=false;for _,k:=range keys.Items {if k==key {own=true;break}}")
	line("inherited:=false;switch key {")
	for _, k := range []string{"constructor", "__proto__", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__"} {
		line("case str(%q):inherited=true", k)
	}
	line("};var value *dynamic;if read {if own {value=d.get(key)} else if inherited {panic(newTrap(\"erased prototype function reads are unsupported\"))}}")
	line("return struct{value *dynamic;own,exists bool}{value,own,own||inherited}}")
	e.extra.WriteString(b.String())
	return name
}
