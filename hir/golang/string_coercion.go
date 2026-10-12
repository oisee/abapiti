package golang

import "github.com/oisee/abapiti/hir"

// A capability on the concrete runtime class, not an assertion to the checker's
// nominal class. Reject custom default-hint hooks rather than treating + as
// string-hint conversion. Supported toString methods always return a string.
func (e *emitter) stringCoercion(c *hir.Class) {
	e.line("func (self *%s) nativeString(depth int) jsString {", e.obj(c.Name))
	e.line("if self==nil {return str(\"undefined\")}")
	if e.liveClasses[c.Name] {
		e.line("if self.viewSource!=nil {return dynStringCoercion(self.viewSource,depth)}")
	}
	unsafe := e.fieldOwner(c, "toString") != nil || e.fieldOwner(c, "valueOf") != nil
	for base := c; base != nil; base = e.classBy(base.Super) {
		unsafe = unsafe || base.CoercionUnsafe || base.Name == "builtin.Error"
	}
	valueOf, _ := e.method(c, "valueOf")
	method, _ := e.method(c, "toString")
	unsafe = unsafe || valueOf != nil || method != nil && (method.Static || len(method.Params) != 0 || method.Result.Kind != hir.String)
	if unsafe {
		e.line("panic(newTrap(\"custom ToPrimitive hook outside certified string coercion\"))")
	} else if method != nil {
		e.line("return self.%s()", e.member(method.Name))
	} else {
		e.line("return str(\"[object Object]\")")
	}
	e.line("}")
}
