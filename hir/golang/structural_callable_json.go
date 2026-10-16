package golang

import "github.com/oisee/abapiti/hir"

// Compiled closures are JS function values: object properties omit them and
// array elements serialize as null. Their captured fields are never JSON data.
// Keep the pinned config writer's ABI unchanged when subset JSON is unused.
func (e *emitter) structuralCallableJSON(c *hir.Class) {
	if e.nativeJSON && e.callableClass(c) {
		e.line("func (self *%s) configJSONOptional() (any,bool) {return nil,false}", e.obj(c.Name))
	}
}
