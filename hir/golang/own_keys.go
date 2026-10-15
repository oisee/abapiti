package golang

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

const ownKeysSource = `
// Refuse native layouts lacking an own-property presence/order ABI. A boxed
// live class view has already unwrapped to its original structural storage.
func dynamicKeys(d *dynamic) *array[jsString] {
 if d==nil {panic(newRangeFault())}
 if s,ok:=d.Value.(interface{dynamicSource()*dynamic});ok {if source:=s.dynamicSource();source!=nil {return dynamicKeys(source)}}
 if d.Tag==2 {if v,ok:=d.Value.(interface{nominalOwnKeys()*array[jsString]});ok {return v.nominalOwnKeys()}}
 if d.Tag!=tagObject {panic(newTrap("own-key enumeration requires original structural property storage"))}
 out:=d.Value.(*orderedMap[jsString,*dynamic]).keys()
 index:=func(k jsString)(uint64,bool){s:=k.String();n,err:=strconv.ParseUint(s,10,32);return n,err==nil && n<4294967295 && strconv.FormatUint(n,10)==s}
 for i:=1;i<len(out.Items);i++ {for j:=i;j>0;j-- {a,ai:=index(out.Items[j-1]);b,bi:=index(out.Items[j]);if !bi || ai&&a<=b {break};out.Items[j-1],out.Items[j]=out.Items[j],out.Items[j-1]}}
 return out
}
`

// Structural records enumerate their per-instance constructor metadata. Retain
// the existing declaration-order proof for native nominal field layouts.
func (e *emitter) tracksOwnKeys(c *hir.Class) bool {
	return c != nil && (e.extraRuntimeKeys && c.Structural || e.nativeJSON && !c.Structural)
}

func (e *emitter) ownKeyClass(c *hir.Class) {
	if c.Structural || e.nativeJSON {
		e.line("func (self *%s) nominalOwnKeys() *array[jsString] {if self==nil {panic(newRangeFault())};if self.ownKeys==nil {panic(newTrap(%q))};return dynamicKeys(&dynamic{Tag:tagObject,Value:self.ownKeys.materialize()})}", e.obj(c.Name), "structural constructor has no proven own-property order")
		return
	}
	if c.Ctor == nil || len(c.Ctor.Params) != 0 {
		return
	}
	var keys []string
	seen := map[string]bool{}
	var visit func(*hir.Class)
	visit = func(c *hir.Class) {
		if c == nil {
			return
		}
		visit(e.classBy(c.Super))
		for _, f := range c.Fields {
			if !f.Static && !seen[f.Name] {
				seen[f.Name] = true
				keys = append(keys, f.Name)
			}
		}
	}
	visit(c)
	e.line("func (self *%s) nominalOwnKeys() *array[jsString] {if self==nil {panic(newRangeFault())};bag:=&orderedMap[jsString,*dynamic]{}", e.obj(c.Name))
	if e.recordDeletion {
		e.line("if self.deletionOrder!=nil {return self.deletionOrder.keys()}")
	}
	for _, key := range keys {
		if e.recordDeletion {
			e.line("if !self.deletedKeys[str(%q)] {bag.set(str(%q),nil)}", key, key)
		} else {
			e.line("bag.set(str(%q),nil)", key)
		}
	}
	e.line("return dynamicKeys(&dynamic{Tag:tagObject,Value:bag})}")
}

// newOwnKeys points every level of a new nominal instance at one key-order
// state stored in its root struct (no allocation) and gives it the class's
// canonical own-field order (see constructorFieldOrder), bases first.
func (e *emitter) newOwnKeys(c *hir.Class) {
	var chain []*hir.Class
	for base := c; base != nil; base = e.classBy(base.Super) {
		chain = append([]*hir.Class{base}, chain...)
	}
	var names []string
	for _, level := range chain {
		for _, name := range constructorFieldOrder(level) {
			names = append(names, "str("+strconv.Quote(name)+")")
		}
	}
	table := e.name("ownkeys." + c.Name)
	fmt.Fprintf(&e.extra, "var %s = []jsString{%s}\n", table, strings.Join(names, ","))
	e.line("keys:=&self.%s().ownKeyStore;keys.names=%s", e.getter(chain[0].Name), table)
	for base := c; base != nil; base = e.classBy(base.Super) {
		e.line("self.%s().ownKeys=keys", e.getter(base.Name))
	}
}

// constructorFieldOrder is a level's likely own-key creation order: the
// fields its constructor assigns on this, in first-write (pre-order) order
// (initializers come first there under es2017), then the remaining declared
// instance fields. It is only a hint: any other order stays exact through
// ownKeyState's materialised map.
func constructorFieldOrder(c *hir.Class) []string {
	declared := map[string]bool{}
	for _, f := range c.Fields {
		if !f.Static && !strings.HasPrefix(f.Name, "#") {
			declared[f.Name] = true
		}
	}
	seen := map[string]bool{}
	var out []string
	if c.Ctor != nil {
		walkStmt(c.Ctor.Body, func(st *hir.Stmt) {
			if st.Kind == hir.Assign && st.X != nil && st.X.Kind == hir.FieldGet && st.X.X != nil && st.X.X.Kind == hir.This && declared[st.X.Name] && !seen[st.X.Name] {
				seen[st.X.Name] = true
				out = append(out, st.X.Name)
			}
		}, func(*hir.Expr, *hir.Expr) {})
	}
	for _, f := range c.Fields {
		if declared[f.Name] && !seen[f.Name] {
			seen[f.Name] = true
			out = append(out, f.Name)
		}
	}
	return out
}
