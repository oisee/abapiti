package golang

import "strings"

// dynamicTypeof recognizes compiled closures without changing their call ABI
// or the box representation. Programs without compiled closures retain the
// established runtime call.
func (e *emitter) dynamicTypeof() string {
	var cases []string
	for _, c := range e.p.Classes {
		if !strings.HasPrefix(c.Name, "closure.") {
			continue
		}
		for _, i := range c.Implements {
			if i == "closure.thunk" || strings.HasPrefix(i, "closure.function.") {
				cases = append(cases, "*"+e.obj(c.Name))
				break
			}
		}
	}
	if len(cases) == 0 {
		return "dynTypeof"
	}
	name := e.name("dynamic.typeof")
	if !e.typeofUsed {
		e.typeofUsed = true
		e.extra.WriteString("func " + name + "(d *dynamic) jsString {if d!=nil && d.Tag==2 {switch d.Value.(type) {case " + strings.Join(cases, ",") + ":return str(\"function\")}};return dynTypeof(d)}\n")
	}
	return name
}
