package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

func ctorParameterMatchesField(c *hir.Class, i int) bool {
	return c.Ctor.Params[i].Type.Equal(c.Ctor.Body.List[i].X.Type) && c.Ctor.Body.List[i].Y.Kind != hir.Narrow
}

// Nil means unproven; an allocated empty table proves an empty literal.
func (b *body) jsonInitializeKeys(receiver string, x *hir.Expr) {
	c := b.e.classBy(x.Type.Name)
	if !b.e.jsonOwnKeys || c == nil || !c.Structural || c.Super != "" || x.OwnKeys == nil {
		return
	}
	keys := receiver + "->" + b.e.name("json.ownKeys")
	b.line("CREATE OBJECT " + keys + ".")
	b.line(keys + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_object.")
	for _, key := range *x.OwnKeys {
		b.jsonRecordKey(receiver, b.expr(hir.L(hir.T(hir.String), key)))
	}
}

func (b *body) jsonRecordKey(receiver, key string) {
	keys := receiver + "->" + b.e.name("json.ownKeys")
	absent := b.temp(hir.T(hir.Dynamic))
	b.line("IF " + keys + " IS BOUND.")
	b.line(keys + "->put( p0 = " + key + " p1 = " + absent + " ).")
	b.line("ENDIF.")
}

func jsonKeyNamesSafe(c *hir.Class) bool {
	for _, field := range c.Fields {
		if strings.IndexFunc(field.Name, func(r rune) bool { return r < 32 }) >= 0 {
			return false
		}
	}
	return true
}
