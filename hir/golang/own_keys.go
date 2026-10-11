package golang

import "github.com/oisee/abapiti/hir"

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

// Retain the existing declaration-order proof for native nominal field layouts.
// Structural constructors have no missing/present-undefined or insertion proof.
func (e *emitter) ownKeyClass(c *hir.Class) {
	if c.Structural || c.Ctor == nil || len(c.Ctor.Params) != 0 {
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
	for _, key := range keys {
		e.line("bag.set(str(%q),nil)", key)
	}
	e.line("return dynamicKeys(&dynamic{Tag:tagObject,Value:bag})}")
}
