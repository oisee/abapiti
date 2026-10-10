package golang

import (
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) descriptor(c *hir.Class) {
	parent := "nil"
	if c.Super != "" {
		parent = "&" + e.name("descriptor."+c.Super)
	}
	tsName := c.Name
	if i := strings.LastIndex(tsName, "."); i >= 0 {
		tsName = tsName[i+1:]
	}
	var names []string
	for _, f := range c.Fields {
		if f.Static {
			names = append(names, "str("+strconv.Quote(f.Name)+")")
		}
	}
	for _, m := range c.Methods {
		if m.Static && m.Name != "class_constructor" {
			names = append(names, "str("+strconv.Quote(m.Name)+")")
		}
	}
	ctor, _ := e.method(c, "constructor")
	var args []string
	callable := !c.Abstract
	if ctor != nil {
		for _, p := range ctor.Params {
			if p.Type.Kind != hir.Optional {
				callable = false
			}
			args = append(args, "*new("+e.typ(p.Type)+")")
		}
	}
	factory := "func() any {panic(newTrap(" + strconv.Quote("classvalue.new requires a concrete zero-argument constructor: "+c.Name) + "))}"
	if callable {
		factory = "func() any {return " + e.name("new."+c.Name) + "(" + strings.Join(args, ",") + ")} "
	}
	e.line("var %s = classDescriptor{Name:str(%q),Parent:%s,Statics:[]jsString{%s},Factory:%s}", e.name("descriptor."+c.Name), tsName, parent, strings.Join(names, ","), factory)
	e.line("func (self *%s) descriptor() *classDescriptor {return &%s}", e.obj(c.Name), e.name("descriptor."+c.Name))
}
