package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// materializer projects a parsed graph into a native data shape, retaining the
// complete original graph as backing metadata. It does not cast a graph node
// into a native reference or discard unknown input fields.
func (e *emitter) materializer(t hir.Type) string {
	id := "runtime.materialize." + t.String()
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	e.classvalueRuntime()
	e.dynamicRuntime()
	dynamic := e.name("runtime.dynamic")
	exception := e.name("exception.RegistryJSONSubsetError")
	e.jsonSubsetRuntime()
	var code strings.Builder
	line := func(f string, args ...any) { fmt.Fprintf(&code, f+"\n", args...) }
	line("CLASS %s DEFINITION PUBLIC CREATE PRIVATE.", name)
	line("PUBLIC SECTION.")
	line("CLASS-METHODS project IMPORTING p0 TYPE REF TO %s RETURNING VALUE(result) TYPE %s.", dynamic, e.typ(t))
	line("PROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.")
	line("CLASS %s IMPLEMENTATION.\nMETHOD project.", name)
	fail := func() { line("RAISE EXCEPTION TYPE %s.", exception) }
	call := func(typ hir.Type, input string) string { return e.materializer(typ) + "=>project( " + input + " )" }
	switch t.Kind {
	case hir.Dynamic:
		line("result = p0.")
	case hir.Optional:
		base := t.Args[0]
		line("IF p0 IS NOT BOUND.\nRETURN.\nENDIF.")
		if base.IsRef() {
			line("result = %s.", call(base, "p0"))
		} else {
			line("CREATE OBJECT result.\nresult->has = abap_true.")
			line("result->value = %s.", call(base, "p0"))
		}
	case hir.String, hir.Bool, hir.Number:
		line("IF p0 IS NOT BOUND.")
		fail()
		line("ENDIF.")
		tag, field := "tag_string", "sval"
		if t.Kind == hir.Bool {
			tag, field = "tag_boolean", "bval"
		}
		if t.Kind == hir.Number {
			tag, field = "tag_number", "nval"
		}
		line("IF p0->tag <> %s=>%s.", dynamic, tag)
		fail()
		line("ENDIF.")
		line("result = p0->%s.", field)
	case hir.Array:
		line("IF p0 IS NOT BOUND.\nRETURN.\nENDIF.")
		line("IF p0->tag <> %s=>tag_array.", dynamic)
		fail()
		line("ENDIF.")
		line("CREATE OBJECT result.")
		line("DATA row TYPE REF TO %s.", dynamic)
		line("DATA item TYPE %s.", e.typ(t.Args[0]))
		line("LOOP AT p0->items INTO row.")
		line("item = %s.", call(t.Args[0], "row"))
		line("APPEND item TO result->items.\nENDLOOP.")
	case hir.ClassRef:
		c := e.classBy(t.Name)
		if c == nil || c.Super != "" || c.Abstract || len(c.Methods) > 0 || c.Ctor == nil || len(c.Ctor.Params) != len(c.Fields) {
			e.err = fmt.Errorf("materialization requires a data shape, got %s", t.String())
			return name
		}
		if e.materialized == nil {
			e.materialized = map[string]bool{}
		}
		e.materialized[t.Name] = true
		line("IF p0 IS NOT BOUND.\nRETURN.\nENDIF.")
		line("IF p0->tag <> %s=>tag_object.", dynamic)
		fail()
		line("ENDIF.")
		var arguments []string
		for i, field := range c.Fields {
			if field.Static || c.Ctor.Params[i].Name != field.Name || !c.Ctor.Params[i].Type.Equal(field.Type) {
				e.err = fmt.Errorf("materialization requires field constructor ABI for %s", t.String())
				return name
			}
			local := fmt.Sprintf("field%d", i)
			line("DATA %s TYPE %s.", local, e.typ(field.Type))
			// Use a string template literal to preserve case and blanks in field names.
			key := strings.ReplaceAll(field.Name, "`", "``")
			line("%s = %s.", local, call(field.Type, "p0->get( `"+key+"` )"))
			arguments = append(arguments, e.param(field.Name)+" = "+local)
		}
		line("result = NEW %s( %s ).", e.name(c.Name), strings.Join(arguments, " "))
		line("result->%s = p0.", e.name("builtin.materializedSource"))
	default:
		e.err = fmt.Errorf("materialization does not support %s", t.String())
		return name
	}
	line("ENDMETHOD.\nENDCLASS.")
	e.files[name+".clas.abap"] = code.String()
	return name
}
