package abap

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

var configJSONOrders = map[string][]string{
	"src/rules/function_module_recommendations.ts.Recommendations": {"name", "replace", "from"},
	"src/version.ts.VersionNew":                                    {"release", "language"},
	"src/_config.ts.IConfig":                                       {"global", "dependencies", "syntax", "rules"},
	"src/_config.ts.IGlobalConfig":                                 {"files", "exclude", "noIssues", "skipGeneratedBOPFInterfaces", "skipGeneratedFunctionGroups", "skipGeneratedGatewayClasses", "skipGeneratedPersistentClasses", "skipGeneratedProxyClasses", "skipGeneratedProxyInterfaces", "useApackDependencies", "skipIncludesWithoutMain", "errorOnDuplicateFilenames"},
	"src/_config.ts.ISyntaxSettings":                               {"version", "errorNamespace", "globalConstants", "ambigiousVoids", "globalMacros"},
	"src/_config.ts.IDependency":                                   {"url", "folder", "files"},
}

func configJSONOrder(e *emitter, c *hir.Class) ([]string, bool) {
	order, ok := configJSONOrders[c.Name]
	if strings.HasPrefix(c.Name, "shape.") && len(c.Fields) == 2 {
		fields := map[string]bool{}
		for _, f := range c.Fields {
			if f.Type.Kind == hir.String {
				fields[f.Name] = true
			}
		}
		if fields["pseudo"] && fields["pragma"] {
			return []string{"pseudo", "pragma"}, true
		}
	}
	if ok {
		return order, true
	}
	var chain []*hir.Class
	rule := false
	for p := c; p != nil; p = e.classBy(p.Super) {
		chain = append(chain, p)
		rule = rule || p.Name == "src/rules/_basic_rule_config.ts.BasicRuleConfig"
	}
	if !rule {
		return nil, false
	}
	seen := map[string]bool{}
	for i := len(chain) - 1; i >= 0; i-- {
		for _, f := range chain[i].Fields {
			if !f.Static && !seen[f.Name] {
				seen[f.Name] = true
				order = append(order, f.Name)
			}
		}
	}
	return order, true
}

// A writer instance owns the recursion stack. An empty result represents
// undefined, distinct from the JSON text for an empty string or null.
func (e *emitter) configJSONRuntime() {
	e.jsonSubsetRuntime()
	// Config projection reads the parser's erased reference table through
	// object, then casts. The pinned generator cannot LOOP into a typed ref.
	for id := range e.types {
		if !strings.HasPrefix(id, "runtime.materialize.") {
			continue
		}
		file := e.name(id) + ".clas.abap"
		source := e.files[file]
		if strings.Contains(source, "LOOP AT p0->items INTO row.") {
			source = strings.Replace(source, "LOOP AT p0->items INTO row.", "DATA row_index TYPE i.\nDATA row_ref TYPE REF TO object.\nWHILE row_index < lines( p0->items ).\nrow_index = row_index + 1.\nREAD TABLE p0->items INDEX row_index INTO row_ref.\nrow ?= row_ref.", 1)
			source = strings.Replace(source, "APPEND item TO result->items.\nENDLOOP.", "APPEND item TO result->items.\nENDWHILE.", 1)
		}
		// Data-shape construction in this adapter must also work on 7.02.
		lines := strings.Split(source, "\n")
		for i, line := range lines {
			if !strings.HasPrefix(line, "result = NEW ") {
				continue
			}
			class, arguments, _ := strings.Cut(strings.TrimPrefix(line, "result = NEW "), "( ")
			arguments = strings.TrimSuffix(arguments, " ).")
			if arguments == ")." {
				arguments = ""
			}
			lines[i] = "CREATE OBJECT result TYPE " + class
			if arguments != "" {
				lines[i] += " EXPORTING " + arguments
			}
			lines[i] += "."
		}
		e.files[file] = strings.Join(lines, "\n")
	}
	// Only the config adapter needs arbitrary UTF-16 escape units. Keep the
	// existing default parser artifact byte-identical when the writer is absent.
	parserFile := e.name("runtime.jsonSubset") + ".clas.abap"
	parser := e.files[parserFile]
	parser = strings.Replace(parser, "result->bval = xsdbool( token = 'true' ).", "IF token = 'true'.\nresult->bval = abap_true.\nELSE.\nresult->bval = abap_false.\nENDIF.", 1)
	parser = strings.Replace(parser, "METHODS hex RETURNING VALUE(result) TYPE i.", "METHODS hex RETURNING VALUE(result) TYPE i.\nMETHODS unicode_unit IMPORTING p0 TYPE i RETURNING VALUE(result) TYPE string.", 1)
	parser = strings.ReplaceAll(parser, "ch = cl_abap_conv_in_ce=>uccp( unit ).", "ch = unicode_unit( code ).")
	parser = strings.Replace(parser, "ELSE.\nRAISE EXCEPTION TYPE "+e.name("exception.RegistryJSONSubsetError")+".\nENDIF.\nELSE.\nunit = code.", "ELSE.\ncursor = cursor - 6.\nch = unicode_unit( code ).\nENDIF.\nELSE.\nunit = code.", 1)
	helper := `METHOD unicode_unit.
DATA unit TYPE x LENGTH 2.
DATA padding TYPE x LENGTH 2.
DATA bytes TYPE xstring.
DATA converter TYPE REF TO cl_abap_conv_in_ce.
IF p0 < 55296 OR p0 > 57343.
result = cl_abap_conv_in_ce=>uccpi( p0 ).
RETURN.
ENDIF.
unit = p0.
CONCATENATE unit+1(1) unit+0(1) INTO bytes IN BYTE MODE.
IF p0 < 56320.
padding = '00DC'.
CONCATENATE bytes padding INTO bytes IN BYTE MODE.
ELSE.
padding = '00D8'.
CONCATENATE padding bytes INTO bytes IN BYTE MODE.
ENDIF.
converter = cl_abap_conv_in_ce=>create( encoding = '4103' ).
converter->convert( EXPORTING input = bytes IMPORTING data = result ).
IF p0 < 56320.
result = result+0(1).
ELSE.
result = result+1(1).
ENDIF.
ENDMETHOD.
`
	end := strings.LastIndex(parser, "ENDCLASS.")
	e.files[parserFile] = parser[:end] + helper + parser[end:]

	name, dyn, exc := e.name("runtime.configJSON"), e.name("runtime.dynamic"), e.name("exception.RegistryJSONSubsetError")
	var declarations, methods strings.Builder
	containerTypes := map[string]hir.Type{}
	var collect func(hir.Type)
	collect = func(t hir.Type) {
		if t.Kind == hir.Array || t.Kind == hir.OrderedMap {
			storage := arrayStorage(t)
			containerTypes[storage.String()] = storage
		}
		for _, a := range t.Args {
			collect(a)
		}
	}
	var expr func(*hir.Expr)
	var stmt func(*hir.Stmt)
	expr = func(x *hir.Expr) {
		if x == nil {
			return
		}
		collect(x.Type)
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, a := range x.Args {
			expr(a)
		}
		stmt(x.Stmt)
	}
	stmt = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		collect(s.Type)
		expr(s.X)
		expr(s.Y)
		stmt(s.Body)
		stmt(s.Else)
		for _, b := range s.List {
			stmt(b)
		}
	}
	for _, c := range e.p.Classes {
		for _, f := range c.Fields {
			collect(f.Type)
		}
		for _, m := range c.Methods {
			stmt(m.Body)
		}
		if c.Ctor != nil {
			stmt(c.Ctor.Body)
		}
	}
	containerKeys := make([]string, 0, len(containerTypes))
	for k := range containerTypes {
		containerKeys = append(containerKeys, k)
	}
	sort.Strings(containerKeys)
	declared := map[string]bool{}
	var write func(hir.Type, bool) string
	write = func(t hir.Type, subset bool) string {
		key := t.String()
		if subset {
			key = "subset." + key
		}
		method := e.name("json.write." + key)
		if declared[key] {
			return method
		}
		declared[key] = true
		fmt.Fprintf(&declarations, "METHODS %s IMPORTING p0 TYPE %s RETURNING VALUE(result) TYPE string.\n", method, e.typ(t))
		var b strings.Builder
		line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
		line("METHOD %s.", method)
		call := func(t hir.Type, v string) string { return write(t, subset) + "( " + v + " )" }
		fail := func() { line("RAISE EXCEPTION TYPE %s.", exc) }
		if t.IsRef() || t.Kind == hir.Dynamic || t.Kind == hir.Optional {
			line("IF p0 IS NOT BOUND.\nRETURN.\nENDIF.")
		}
		switch t.Kind {
		case hir.String:
			line("result = quote( p0 ).")
		case hir.Bool:
			line("IF p0 = abap_true.\nresult = `true`.\nELSE.\nresult = `false`.\nENDIF.")
		case hir.I32, hir.I64:
			if t.Kind == hir.I64 {
				line("IF p0 > 9007199254740991 OR p0 < -9007199254740991.")
				fail()
				line("ENDIF.")
			}
			line("result = |{ p0 }|.")
		case hir.Number:
			line("result = number( p0 ).")
		case hir.Optional:
			if t.Args[0].IsRef() {
				line("result = %s.", call(t.Args[0], "p0"))
			} else if t.Args[0].Kind == hir.Number {
				line("IF p0->has = abap_true.\nIF p0->number_special <> 0.\nresult = `null`.\nELSE.\nresult = %s.\nENDIF.\nENDIF.", call(t.Args[0], "p0->value"))
			} else {
				line("IF p0->has = abap_true.\nresult = %s.\nENDIF.", call(t.Args[0], "p0->value"))
			}
		case hir.Array:
			line("enter( p0 ).\nDATA item TYPE %s.\nDATA text TYPE string.\nDATA comma TYPE string.\nDATA idx TYPE i.\nresult = `[`.", e.typ(t.Args[0]))
			line("p0->dense( ).")
			line("WHILE idx < lines( p0->items ).\nidx = idx + 1.")
			if t.Args[0].IsRef() || t.Args[0].Kind == hir.Optional {
				line("DATA itemref TYPE REF TO object.\nREAD TABLE p0->items INDEX idx INTO itemref.\nitem ?= itemref.")
			} else {
				line("READ TABLE p0->items INDEX idx INTO item.")
			}
			if t.Args[0].Equal(hir.Ref(hir.RootObject)) {
				// Reference-array storage is erased. A dynamic view must inspect
				// each element, rather than guessing a static element type.
				line("DATA boxed TYPE REF TO %s.\nCLEAR boxed.\nIF item IS BOUND.\nIF item IS INSTANCE OF %s.\nboxed ?= item.\nELSE.\nCREATE OBJECT boxed.\nboxed->tag = %s=>tag_ref.\nboxed->oval = item.\nENDIF.\nENDIF.\ntext = %s.", dyn, dyn, dyn, call(hir.T(hir.Dynamic), "boxed"))
			} else {
				line("text = %s.", call(t.Args[0], "item"))
			}
			line("IF text IS INITIAL.\ntext = `null`.\nENDIF.\nCONCATENATE result comma text INTO result RESPECTING BLANKS.\ncomma = `,`.\nENDWHILE.\nCONCATENATE result `]` INTO result.\nleave( p0 ).")
		case hir.OrderedMap:
			if subset {
				fail() // HIR map storage alone cannot distinguish a record from ES Map.
				break
			}
			if t.Args[0].Kind != hir.String {
				fail()
				break
			}
			line("enter( p0 ).\nDATA graph TYPE REF TO %s.\nDATA row TYPE %s=>entry.\nDATA value TYPE REF TO %s.\nDATA text TYPE string.\nCREATE OBJECT graph.\ngraph->tag = %s=>tag_object.", dyn, e.name("runtime."+t.String()), dyn, dyn)
			line("LOOP AT p0->entries INTO row.")
			line("text = %s.\nIF text IS NOT INITIAL.\nvalue = %s=>parse( text ).\ngraph->put( p0 = row-k p1 = value ).\nENDIF.\nENDLOOP.\nresult = %s.\nleave( p0 ).", call(t.Args[1], "row-v"), e.name("runtime.jsonSubset"), call(hir.T(hir.Dynamic), "graph"))
		case hir.ClassValue:
			if !subset {
				fail()
				break
			}
			// JSON calls a function's static/inherited toJSON before omitting
			// that function. Only descriptors proving its absence can skip.
			line("DATA descriptor TYPE REF TO %s.\ndescriptor = p0.\nWHILE descriptor IS BOUND.", e.name("runtime.classvalue"))
			line("IF descriptor->lexical = abap_true OR descriptor->emitted IS INITIAL OR descriptor->static_names IS NOT BOUND.")
			fail()
			line("ENDIF.")
			for _, class := range e.p.Classes {
				if class.CoercionUnsafe || class.ViewUnsafe {
					line("IF descriptor->emitted = `%s`.", e.name(class.Name))
					fail()
					line("ENDIF.")
				}
			}
			line("IF descriptor->has( `toJSON` ) = abap_true OR descriptor->has( `get.toJSON` ) = abap_true.")
			fail()
			line("ENDIF.\ndescriptor = descriptor->parent.\nENDWHILE.\nCLEAR result.")
		case hir.ClassRef:
			c := e.classBy(t.Name)
			if c != nil && strings.HasPrefix(c.Name, "tuple.") {
				line("DATA text TYPE string.\nenter( p0 ).\nresult = `[`. ")
				for i, f := range c.Fields {
					if f.Static {
						continue
					}
					if i > 0 {
						line("result = result && `,`.")
					}
					line("text = %s.\nIF text IS INITIAL.\ntext = `null`.\nENDIF.\nresult = result && text.", call(f.Type, "p0->"+e.member(f.Name)))
				}
				line("result = result && `]`.\nleave( p0 ).")
				break
			}
			if subset {
				_, nominalProven := e.lspTokenJSONOrder(c)
				if c == nil || c.CoercionUnsafe || c.ViewUnsafe && !nominalProven || !jsonKeyNamesSafe(c) {
					fail()
					break
				}
				hasToJSON := false
				for parent := c; parent != nil; parent = e.classBy(parent.Super) {
					for _, m := range parent.Methods {
						hasToJSON = hasToJSON || m.Name == "toJSON" || m.Name == "get.toJSON"
					}
				}
				if hasToJSON {
					fail()
					break
				}
				for parent := c; parent != nil; parent = e.classBy(parent.Super) {
					for _, field := range parent.Fields {
						if field.Static || field.Name != "toJSON" {
							continue
						}
						base := field.Type
						for base.Kind == hir.Optional {
							base = base.Args[0]
						}
						value := "p0->" + e.member(field.Name)
						if base.Kind == hir.Dynamic || base.Kind == hir.ClassValue {
							line("IF %s IS BOUND.", value)
							if base.Kind == hir.Dynamic {
								line("IF %s->tag = %s=>tag_class.", value, dyn)
							}
							fail()
							if base.Kind == hir.Dynamic {
								line("ENDIF.")
							}
							line("ENDIF.")
						}
					}
				}
				if e.liveClasses[c.Name] {
					line("IF p0->%s IS BOUND.\nresult = %s.\nRETURN.\nENDIF.", e.name("view.source"), call(hir.T(hir.Dynamic), "p0->"+e.name("view.source")))
				}
				if e.jsonOwnKeys && c.Structural && c.Super == "" {
					keys := "p0->" + e.name("json.ownKeys")
					line("IF %s IS BOUND.\nenter( p0 ).", keys)
					line("DATA own_graph TYPE REF TO %s.\nDATA own_child TYPE REF TO %s.\nDATA own_row TYPE %s=>entry.\nDATA own_text TYPE string.\nCREATE OBJECT own_graph.\nown_graph->tag = %s=>tag_object.", dyn, dyn, dyn, dyn)
					line("LOOP AT %s->entries INTO own_row.\nCLEAR own_child.\nCASE own_row-k.", keys)
					for _, field := range c.Fields {
						if field.Static {
							continue
						}
						line("WHEN `%s`.\nown_text = %s.\nIF own_text IS NOT INITIAL.\nown_child = %s=>parse( own_text ).\nENDIF.", strings.ReplaceAll(field.Name, "`", "``"), call(field.Type, "p0->"+e.member(field.Name)), e.name("runtime.jsonSubset"))
					}
					line("WHEN OTHERS.")
					// Both source-backed live shapes and native structural shapes
					// retain added own properties in extras. A live class without
					// a source is still a native object, not a reason to omit them.
					if e.types["runtime.liveView"] {
						extras := "p0->" + e.name("view.extras")
						line("IF %s IS NOT BOUND.", extras)
						fail()
						line("ENDIF.")
						line("READ TABLE %s->entries WITH KEY k = own_row-k TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.", extras)
						fail()
						line("ENDIF.")
						line("own_child = %s->get( own_row-k ).", extras)
					} else {
						fail()
					}
					line("ENDCASE.\nown_graph->put( p0 = own_row-k p1 = own_child ).\nENDLOOP.\nresult = %s.\nleave( p0 ).\nRETURN.\nENDIF.", call(hir.T(hir.Dynamic), "own_graph"))
				}
				if order, proven := e.lspTokenJSONOrder(c); proven {
					// Known subclasses serialize their own complete layout. A
					// subclass without a certificate must never fall through.
					for _, child := range e.descendants(c) {
						line("IF p0 IS INSTANCE OF %s.", e.name(child.Name))
						if _, ok := e.lspTokenJSONOrder(child); ok {
							local := e.name("json.native." + child.Name)
							line("DATA %s TYPE REF TO %s.\n%s ?= p0.\nresult = %s.\nRETURN.", local, e.name(child.Name), local, call(hir.Ref(child.Name), local))
						} else {
							fail()
						}
						line("ENDIF.")
					}
					line("enter( p0 ).\nDATA native_graph TYPE REF TO %s.\nDATA native_child TYPE REF TO %s.\nDATA native_text TYPE string.\nCREATE OBJECT native_graph.\nnative_graph->tag = %s=>tag_object.", dyn, dyn, dyn)
					for _, key := range order {
						owner := e.fieldOwner(c, key)
						for _, field := range owner.Fields {
							if field.Static || field.Name != key {
								continue
							}
							if e.recordDelete {
								line("READ TABLE p0->%s WITH KEY table_line = `%s` TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.", e.name("record.deleted"), strings.ReplaceAll(key, "`", "``"))
							}
							line("native_text = %s.\nIF native_text IS NOT INITIAL.\nnative_child = %s=>parse( native_text ).\nnative_graph->put( p0 = `%s` p1 = native_child ).\nENDIF.", call(field.Type, "p0->"+e.member(key)), e.name("runtime.jsonSubset"), strings.ReplaceAll(key, "`", "``"))
							if e.recordDelete {
								line("ENDIF.")
							}
						}
					}
					line("result = %s.\nleave( p0 ).", call(hir.T(hir.Dynamic), "native_graph"))
					break
				}
				if !e.materialized[c.Name] {
					fail()
					break
				}
				source := "p0->" + e.name("builtin.materializedSource")
				line("IF %s IS NOT BOUND.", source)
				fail()
				line("ENDIF.")
				line("enter( p0 ).\nDATA graph TYPE REF TO %s.\nDATA child TYPE REF TO %s.\nDATA row TYPE %s=>entry.\nDATA text TYPE string.\nDATA added TYPE i.\nCREATE OBJECT graph.\ngraph->tag = %s=>tag_object.", dyn, dyn, dyn, dyn)
				line("LOOP AT %s->entries INTO row.\nCASE row-k.", source)
				for _, field := range c.Fields {
					line("WHEN `%s`.\ntext = %s.\nCLEAR child.\nIF text IS NOT INITIAL.\nchild = %s=>parse( text ).\nENDIF.", strings.ReplaceAll(field.Name, "`", "``"), call(field.Type, "p0->"+e.member(field.Name)), e.name("runtime.jsonSubset"))
				}
				line("WHEN OTHERS.\nchild = row-v.\nENDCASE.\ngraph->put( p0 = row-k p1 = child ).\nENDLOOP.")
				for _, field := range c.Fields {
					line("READ TABLE %s->entries WITH KEY k = `%s` TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.\ntext = %s.\nIF text IS NOT INITIAL.\nadded = added + 1.\nchild = %s=>parse( text ).\ngraph->put( p0 = `%s` p1 = child ).\nENDIF.\nENDIF.", source, strings.ReplaceAll(field.Name, "`", "``"), call(field.Type, "p0->"+e.member(field.Name)), e.name("runtime.jsonSubset"), strings.ReplaceAll(field.Name, "`", "``"))
				}
				line("IF added > 1.")
				fail()
				line("ENDIF.")
				line("result = %s.\nleave( p0 ).", call(hir.T(hir.Dynamic), "graph"))
				break
			}
			if c == nil {
				fail()
				break
			}
			if e.liveClasses[c.Name] {
				line("IF p0->%s IS BOUND.\nresult = %s.\nRETURN.\nENDIF.", e.name("view.source"), call(hir.T(hir.Dynamic), "p0->"+e.name("view.source")))
			}
			order, ok := configJSONOrder(e, c)
			if !ok {
				fail()
				break
			}
			toJSON := false
			for p := c; p != nil; p = e.classBy(p.Super) {
				for _, m := range p.Methods {
					toJSON = toJSON || m.Name == "toJSON"
				}
			}
			if toJSON {
				fail()
				break
			}
			if e.materialized[c.Name] {
				line("IF p0->%s IS BOUND.\nresult = %s.\nRETURN.\nENDIF.", e.name("builtin.materializedSource"), call(hir.T(hir.Dynamic), "p0->"+e.name("builtin.materializedSource")))
			}
			line("enter( p0 ).\nDATA text TYPE string.\nDATA graph TYPE REF TO %s.\nDATA child TYPE REF TO %s.\nCREATE OBJECT graph.\ngraph->tag = %s=>tag_object.", dyn, dyn, dyn)
			seen := map[string]bool{}
			for _, key := range order {
				var field *hir.Field
				for p := c; p != nil && field == nil; p = e.classBy(p.Super) {
					for i := range p.Fields {
						if p.Fields[i].Name == key && !p.Fields[i].Static {
							field = &p.Fields[i]
							break
						}
					}
				}
				if field == nil {
					continue
				}
				seen[key] = true
				line("text = %s.\nIF text IS NOT INITIAL.", call(field.Type, "p0->"+e.member(key)))
				line("child = %s=>parse( text ).\ngraph->put( p0 = `%s` p1 = child ).\nENDIF.", e.name("runtime.jsonSubset"), strings.ReplaceAll(key, "`", "``"))
			}
			for _, f := range c.Fields {
				if !f.Static && !seen[f.Name] {
					line("text = %s.\nIF text IS NOT INITIAL.", call(f.Type, "p0->"+e.member(f.Name)))
					fail()
					line("ENDIF.")
				}
			}
			line("result = %s.\nleave( p0 ).", call(hir.T(hir.Dynamic), "graph"))
		case hir.Dynamic:
			line("DATA text TYPE string.\nDATA key TYPE string.\nDATA comma TYPE string.\nDATA child TYPE REF TO %s.\nDATA row TYPE %s=>entry.", dyn, dyn)
			line("TYPES: BEGIN OF indexed_entry, idx TYPE p LENGTH 8 DECIMALS 0, row TYPE %s=>entry, END OF indexed_entry.", dyn)
			line("DATA indexed TYPE STANDARD TABLE OF indexed_entry WITH DEFAULT KEY.\nDATA indexed_row TYPE indexed_entry.\nDATA rest TYPE %s=>entries_type.\nDATA ordered TYPE %s=>entries_type.\nDATA canonical TYPE string.\nDATA idx TYPE p LENGTH 8 DECIMALS 0.", dyn, dyn)
			line("CASE p0->tag.\nWHEN %s=>tag_string.\nresult = quote( p0->sval ).\nWHEN %s=>tag_number.\nIF p0->number_special <> 0.\nresult = `null`.\nELSE.\nresult = number( p0->nval ).\nENDIF.\nWHEN %s=>tag_boolean.", dyn, dyn, dyn)
			line("IF p0->bval = abap_true.\nresult = `true`.\nELSE.\nresult = `false`.\nENDIF.\nWHEN %s=>tag_null.\nresult = `null`.\nWHEN %s=>tag_array.", dyn, dyn)
			line("enter( p0 ).\nDATA itemidx TYPE i.\nDATA childref TYPE REF TO object.\nresult = `[`.\nWHILE itemidx < lines( p0->items ).\nitemidx = itemidx + 1.\nREAD TABLE p0->items INDEX itemidx INTO childref.\nchild ?= childref.\ntext = %s.\nIF text IS INITIAL.\ntext = `null`.\nENDIF.\nCONCATENATE result comma text INTO result RESPECTING BLANKS.\ncomma = `,`.\nENDWHILE.\nCONCATENATE result `]` INTO result.\nleave( p0 ).", call(t, "child"))
			if subset {
				// A function-valued own toJSON is invoked before enumeration
				// by JSON.stringify. This runtime has no certified call ABI.
				line("WHEN %s=>tag_object.", dyn)
				// Fresh literal/assignment __proto__ can change prototypes,
				// whereas JSON.parse creates an own data key. Storage does
				// not retain that distinction, so neither shape is guessed.
				line("READ TABLE p0->entries WITH KEY k = `__proto__` TRANSPORTING NO FIELDS.\nIF sy-subrc = 0.")
				fail()
				line("ENDIF.")
				line("child = p0->get( `toJSON` ).\nIF child IS BOUND.\nIF child->tag = %s=>tag_class.", dyn)
				fail()
				line("ENDIF.\nENDIF.")
			} else {
				line("WHEN %s=>tag_object.", dyn)
			}
			line("enter( p0 ).\nLOOP AT p0->entries INTO row.\nCLEAR canonical.\nIF row-k CO `0123456789` AND row-k IS NOT INITIAL AND strlen( row-k ) <= 10.\nidx = row-k.\ncanonical = |{ idx }|.\nENDIF.\nIF canonical IS NOT INITIAL AND canonical = row-k AND idx < 4294967295.\nindexed_row-idx = idx.\nindexed_row-row = row.\nAPPEND indexed_row TO indexed.\nELSE.\nAPPEND row TO rest.\nENDIF.\nENDLOOP.\nSORT indexed BY idx.\nLOOP AT indexed INTO indexed_row.\nAPPEND indexed_row-row TO ordered.\nENDLOOP.\nAPPEND LINES OF rest TO ordered.\nresult = `{`.")
			line("LOOP AT ordered INTO row.\ntext = %s.\nIF text IS INITIAL.\nCONTINUE.\nENDIF.\nkey = quote( row-k ).\nCONCATENATE result comma key `:` text INTO result RESPECTING BLANKS.\ncomma = `,`.\nENDLOOP.\nCONCATENATE result `}` INTO result.\nleave( p0 ).", call(t, "row-v"))
			line("WHEN %s=>tag_ref.", dyn)
			// Most-derived matches precede bases; casts work on ABAP 7.02.
			classes := append([]*hir.Class(nil), e.p.Classes...)
			depth := func(c *hir.Class) int {
				n := 0
				for ; c != nil; c = e.classBy(c.Super) {
					n++
				}
				return n
			}
			sort.SliceStable(classes, func(i, j int) bool { return depth(classes[i]) > depth(classes[j]) })
			for _, c := range classes {
				if subset {
					if !e.jsonSubsetType(hir.Ref(c.Name)) {
						continue
					}
				} else if _, ok := configJSONOrder(e, c); !ok {
					continue
				}
				local := e.name("json.local." + c.Name)
				line("DATA %s TYPE REF TO %s.\nIF p0->oval IS INSTANCE OF %s.\n%s ?= p0->oval.\nresult = %s.\nRETURN.\nENDIF.", local, e.name(c.Name), e.name(c.Name), local, call(hir.Ref(c.Name), local))
			}
			for _, k := range containerKeys {
				t := containerTypes[k]
				if t.Kind == hir.OrderedMap && t.Args[0].Kind != hir.String {
					continue
				}
				local := e.name("json.local." + k)
				line("DATA %s TYPE %s.\nIF p0->oval IS INSTANCE OF %s.\n%s ?= p0->oval.\nresult = %s.\nRETURN.\nENDIF.", local, e.typ(t), refClass(e.typ(t)), local, call(t, local))
			}
			fail()
			line("WHEN %s=>tag_class.", dyn)
			if subset {
				line("IF p0->cval IS NOT BOUND.")
				fail()
				line("ENDIF.")
				line("result = %s.", call(hir.T(hir.ClassValue), "p0->cval"))
			} else {
				line("RETURN.")
			}
			line("WHEN OTHERS.")
			fail()
			line("ENDCASE.")
		default:
			fail()
		}
		line("ENDMETHOD.")
		methods.WriteString(b.String())
		return method
	}
	// Register roots and tagged references; recursively register field types.
	for _, c := range e.p.Classes {
		if _, ok := configJSONOrder(e, c); ok {
			write(hir.Ref(c.Name), false)
		}
	}
	write(hir.T(hir.Dynamic), false)
	for _, root := range e.subsetJSONRoots {
		write(root, true)
	}
	src := strings.NewReplacer("$W", name, "$E", exc).Replace(configJSONCommon)
	src = strings.Replace(src, "$METHODS", declarations.String(), 1)
	src = strings.Replace(src, "$IMPLEMENTATIONS", methods.String(), 1)
	e.files[name+".clas.abap"] = src
}

//go:embed config_json_runtime.abap.tmpl
var configJSONCommon string
