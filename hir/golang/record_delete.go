package golang

import "github.com/oisee/abapiti/hir"

// Live views delete from their original storage. Native slots must carry
// undefined, and presence is tracked separately from a stored undefined value.
// Capture the existing own-key order before the first native deletion so a
// later write appends the key, just as it does for an ordinary JS object.
func (e *emitter) deleteFields(c *hir.Class, fields map[string]hir.Field, order []string) {
	e.line("func (self *%s) dynamicDelete(key jsString) bool {", e.obj(c.Name))
	e.line("if self==nil {panic(newRangeFault())}")
	if e.liveClasses[c.Name] {
		e.line("if self.viewSource!=nil {return self.viewSource.deleteProperty(key)}")
	}
	e.line("switch key {")
	for _, name := range order {
		f := fields[name]
		e.line("case str(%q):", name)
		if f.Type.Kind != hir.Optional && f.Type.Kind != hir.Dynamic {
			e.line("panic(newTrap(%q))", "delete cannot represent absence in required native field "+name)
			continue
		}
		e.line("if self.deletedKeys[key] {return true};if self.deletionOrder==nil {if v,ok:=any(self).(interface{nominalOwnKeys()*array[jsString]});ok {keys:=v.nominalOwnKeys();self.deletionOrder=&fastOrderedMap[jsString,*dynamic]{};for _,k:=range keys.Items {self.deletionOrder.set(k,nil)}}}")
		e.line("var absent %s;self.%s=absent", e.typ(f.Type), e.member(name))
		e.line("if self.deletedKeys==nil {self.deletedKeys=map[jsString]bool{}};self.deletedKeys[key]=true;if self.deletionOrder!=nil {self.deletionOrder.delete(key)}")
		if e.tracksOwnKeys(c) {
			e.line("if self.ownKeys!=nil {self.ownKeys.delete(key)}")
		}
		e.line("return true")
	}
	e.line("}")
	if len(e.liveClasses) > 0 && c.Structural {
		e.line("if self.viewExtras!=nil {self.viewExtras.delete(key)};if self.ownKeys!=nil {self.ownKeys.delete(key)}")
	}
	e.line("return true}")
}

const recordDeletionSource = `
func (d *dynamic) deleteProperty(k jsString) bool {
 if d==nil {panic(newRangeFault())}
 if d.Tag==tagObject {d.Value.(*orderedMap[jsString,*dynamic]).delete(k);return true}
 if d.Tag==2 {if v,ok:=d.Value.(interface{dynamicDelete(jsString) bool});ok {return v.dynamicDelete(k)}}
 panic(newTrap("delete requires storage that represents property absence"))
}
`
