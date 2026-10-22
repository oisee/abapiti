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
 if isArray(d) {
  a,ok:=d.Value.(interface{liveLength() int32;liveGet(int32)(*dynamic,bool)});if !ok {panic(newTrap("array lacks indexed storage"))}
  out:=&array[jsString]{};for i:=int32(0);i<a.liveLength();i++ {if _,present:=a.liveGet(i);present {out.push(integerString(int64(i)))}};return out
 }
 if d.Tag==2 {if v,ok:=d.Value.(interface{nominalOwnKeys()*array[jsString]});ok {return v.nominalOwnKeys()}}
 if d.Tag!=tagObject {panic(newTrap("own-key enumeration requires original structural property storage"))}
 return sortOwnKeys(d.Value.(*orderedMap[jsString,*dynamic]).keys())
}
// Integer-like keys enumerate first, ascending; the rest keep insertion order.
// A canonical index starts with an ASCII digit, so others skip the decode.
func sortOwnKeys(out *array[jsString]) *array[jsString] {
 index:=func(k jsString)(uint64,bool){if len(k)<2 || k[1]!=0 || k[0]<'0' || k[0]>'9' {return 0,false};n,ok:=parseCanonicalIndex(k.String(),32);return n,ok&&n<4294967295}
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
		e.line("func (self *%s) nominalOwnKeys() *array[jsString] {if self==nil {panic(newRangeFault())};if self.ownKeys==nil {panic(newTrap(%q))};return sortOwnKeys(self.ownKeys.keys())}", e.obj(c.Name), "structural constructor has no proven own-property order")
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
			if !f.Static && !f.PrivateSlot && !seen[f.Name] {
				seen[f.Name] = true
				keys = append(keys, f.Name)
			}
		}
	}
	visit(c)
	e.line("func (self *%s) nominalOwnKeys() *array[jsString] {if self==nil {panic(newRangeFault())};bag:=&fastOrderedMap[jsString,*dynamic]{}", e.obj(c.Name))
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

// literalOwnKeys gives a fresh structural record its statically proven key
// sequence: a shared package-level names table (one per distinct sequence)
// with every bit set, so no ordered map exists until an out-of-order write,
// unknown key or delete materialises one. The table is only ever read.
func (e *emitter) literalOwnKeys(b *body, result string, keys []string) {
	if len(keys) == 0 || len(keys) > 64 {
		b.line("%s.ownKeys=&ownKeyState{}", result)
		for _, key := range keys {
			b.line("%s.ownKeys.set(str(%q),nil)", result, key)
		}
		return
	}
	names := make([]string, len(keys))
	for i, key := range keys {
		names[i] = "str(" + strconv.Quote(key) + ")"
	}
	id := "ownkeys.literal." + strings.Join(keys, "\x00")
	table := e.name(id)
	if e.ownKeyTables == nil {
		e.ownKeyTables = map[string]bool{}
	}
	if !e.ownKeyTables[id] {
		e.ownKeyTables[id] = true
		fmt.Fprintf(&e.extra, "var %s = []jsString{%s}\n", table, strings.Join(names, ","))
	}
	b.line("%s.ownKeys=&ownKeyState{names:%s,mask:%#x}", result, table, ^uint64(0)>>(64-len(keys)))
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
		if !f.Static && !f.PrivateSlot {
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
