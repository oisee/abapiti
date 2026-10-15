package golang

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

// Config.getDefault's source pin proves these literal insertion orders. HIR
// data-shape field order alone is not a proof of JavaScript own-key order.
var configJSONOrders = map[string][]string{
	"src/rules/function_module_recommendations.ts.Recommendations": {"name", "replace", "from"},
	"src/version.ts.VersionNew":                                    {"release", "language"},
	"src/_config.ts.IConfig":                                       {"global", "dependencies", "syntax", "rules"},
	"src/_config.ts.IGlobalConfig":                                 {"files", "exclude", "noIssues", "skipGeneratedBOPFInterfaces", "skipGeneratedFunctionGroups", "skipGeneratedGatewayClasses", "skipGeneratedPersistentClasses", "skipGeneratedProxyClasses", "skipGeneratedProxyInterfaces", "useApackDependencies", "skipIncludesWithoutMain", "errorOnDuplicateFilenames"},
	"src/_config.ts.ISyntaxSettings":                               {"version", "errorNamespace", "globalConstants", "ambigiousVoids", "globalMacros"},
	"src/_config.ts.IDependency":                                   {"url", "folder", "files"},
}

func (e *emitter) configJSONClass(c *hir.Class) {
	e.line("func (self *%s) configJSON(w *configJSONWriter) jsString {", e.obj(c.Name))
	if e.liveClasses[c.Name] {
		e.line("if self!=nil && self.viewSource!=nil {s,ok:=w.value(self.viewSource);if !ok {panic(newRangeFault())};return s}")
	}
	order, pinned := configJSONOrders[c.Name]
	// The only anonymous nested object in the pinned default graph is the
	// PreferPragmasConf.check row; each literal writes pseudo before pragma.
	if strings.HasPrefix(c.Name, "shape.") && len(c.Fields) == 2 {
		names := map[string]bool{}
		for _, f := range c.Fields {
			if f.Type.Kind == hir.String {
				names[f.Name] = true
			}
		}
		if names["pseudo"] && names["pragma"] {
			order = []string{"pseudo", "pragma"}
			pinned = true
		}
	}
	var chain []*hir.Class
	ruleConfig := false
	for parent := c; parent != nil; parent = e.classBy(parent.Super) {
		chain = append(chain, parent)
		ruleConfig = ruleConfig || parent.Name == "src/rules/_basic_rule_config.ts.BasicRuleConfig"
	}

	if e.nativeJSON && !c.Structural {
		for _, parent := range chain {
			if parent.UndefinedDefineFields {
				e.line("panic(newTrap(%q))}", "JSON own-key order requires undefined define-field storage: "+c.Name)
				return
			}
		}
	}

	// A literal's own-property proof is per instance, independent of field
	// layout. Read values now so native writes and live-view writes stay visible.
	if e.tracksOwnKeys(c) {
		e.line("if self!=nil && self.ownKeys!=nil {")
		for _, parent := range chain {
			for _, m := range parent.Methods {
				if m.Name == "toJSON" {
					e.line("panic(newTrap(%q))", "structural JSON toJSON is unsupported")
				}
			}
		}
		e.line("w.enter(self);defer w.leave(self);entries:=[]entry[jsString,any]{};for _,key:=range self.nominalOwnKeys().Items {var value any;switch key {")
		seen := map[string]bool{}
		for _, parent := range chain {
			for _, field := range parent.Fields {
				if field.Static || seen[field.Name] {
					continue
				}
				seen[field.Name] = true
				e.line("case str(%q):value=self.%s().%s", field.Name, e.getter(parent.Name), e.member(field.Name))
			}
		}
		if len(e.liveClasses) > 0 && c.Structural {
			e.line("default:if self.viewExtras!=nil {value=self.viewExtras.get(key).Value}")
		} else {
			e.line("default:panic(newTrap(%q))", "structural own key has no lowered storage")
		}
		e.line("};entries=append(entries,entry[jsString,any]{key,value})};return w.object(entries)}")
	}
	// Parsed data shapes retain their original key order, but field storage is
	// live: serialization must not replay the stale input graph after writes.
	materializable := c.Super == "" && !c.Abstract && len(c.Methods) == 0 && c.Ctor != nil && len(c.Ctor.Params) == len(c.Fields)
	if materializable {
		e.line("if self!=nil && self.source!=nil {")
		e.line("w.enter(self);defer w.leave(self);entries:=[]entry[jsString,any]{};seen:=map[jsString]bool{}")
		e.line("for _,item:=range self.source.Value.(*fastOrderedMap[jsString,*dynamic]).Entries {key:=item.Key;seen[key]=true;var value any=item.Value;switch key {")
		for _, f := range c.Fields {
			if !f.Static {
				e.line("case str(%q): value=self.%s", f.Name, e.member(f.Name))
			}
		}
		e.line("};entries=append(entries,entry[jsString,any]{key,value})}")
		// Without a write-order ABI, adding multiple new keys cannot be ordered.
		e.line("added:=0")
		for _, f := range c.Fields {
			if !f.Static {
				e.line("if !seen[str(%q)] {if _,ok:=w.value(self.%s);ok {added++;entries=append(entries,entry[jsString,any]{str(%q),self.%s})}}", f.Name, e.member(f.Name), f.Name, e.member(f.Name))
			}
		}
		e.line("if added>1 {panic(newTrap(%q))};return w.object(entries)", "JSON new own-key insertion order is unproven for "+c.Name)
		e.line("}")
	}
	// Only default rule configuration classes have a declaration-order proof.
	// Other boxed instances must refuse rather than serialize Go implementation fields.
	if !pinned && !ruleConfig {
		e.line("panic(newTrap(%q))", "config JSON own-property order is unproven for "+c.Name)
		e.line("}")
		return
	}
	for _, parent := range chain {
		for _, m := range parent.Methods {
			if m.Name == "toJSON" {
				e.line("panic(newTrap(%q))", "config JSON toJSON is unsupported: "+c.Name)
				e.line("}")
				return
			}
		}
	}
	e.line("if self==nil {return str(\"null\")};w.enter(self);defer w.leave(self)")
	e.line("entries:=[]entry[jsString,any]{}")
	add := func(parent *hir.Class, field hir.Field) {
		access := "self." + e.member(field.Name)
		if parent.Name != c.Name {
			access = "self." + e.getter(parent.Name) + "()." + e.member(field.Name)
		}
		e.line("entries=append(entries,entry[jsString,any]{str(%q),%s})", field.Name, access)
	}
	if pinned {
		for _, key := range order {
			for _, f := range c.Fields {
				if f.Name == key && !f.Static {
					add(c, f)
				}
			}
		}
		// Fields outside the pinned default literal are absent in this adapter.
		for _, f := range c.Fields {
			if !f.Static && !containsJSONKey(order, f.Name) {
				e.line("if _,present:=w.value(self.%s);present {panic(newTrap(%q))}", e.member(f.Name), "config JSON field outside pinned default literal: "+f.Name)
			}
		}
	} else {
		seen := map[string]bool{}
		for i := len(chain) - 1; i >= 0; i-- {
			for _, f := range chain[i].Fields {
				if !f.Static && !seen[f.Name] {
					seen[f.Name] = true
					// A derived declaration replaces the value without moving the key.
					owner, field := chain[i], f
					for j := i - 1; j >= 0; j-- {
						for _, candidate := range chain[j].Fields {
							if !candidate.Static && candidate.Name == f.Name {
								owner, field = chain[j], candidate
							}
						}
					}
					add(owner, field)
				}
			}
		}
	}
	e.line("return w.object(entries)")
	e.line("}")
}
func containsJSONKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

const configJSONRuntimeSource = `
// Usage-gated JSON data-graph writer; native own-key order must be proven.
type configJSONWriter struct { active map[any]bool }
func (w *configJSONWriter) enter(v any) {if w.active[v] {panic(newTrap("cyclic config JSON graph"))};w.active[v]=true}
func (w *configJSONWriter) leave(v any) {delete(w.active,v)}
// A supplied native version object has no retained literal key order. Only
// the function's freshly built version literal or a tagged JSON input is proven.
func stringifyConfigChecked(v any, ver *dynamic, lang optional[jsString]) jsString {
 if !lang.Has && ver!=nil && ver.Tag==2 {
  if native,ok:=ver.Value.(interface{dynamicSource()*dynamic});ok && native.dynamicSource()==nil {
   panic(newTrap("config JSON native version input own-key order is unproven"))
  }
 }
 return stringifyConfig(v)
}
func stringifyConfig(v any) jsString {w:=&configJSONWriter{active:map[any]bool{}};s,ok:=w.value(v);if !ok {panic(newTrap("undefined config JSON root"))};return s}
func (v optional[T]) configJSONOptional() (any,bool) {return v.Value,v.Has}
func (a *array[T]) configJSON(w *configJSONWriter) jsString {
 if a==nil {return str("null")};w.enter(a);defer w.leave(a)
 out:=str("[");for i:=range a.Items {if i>0 {out+=str(",")};s,ok:=w.value(a.get(int32(i)));if !ok {s=str("null")};out+=s};return out+str("]")
}
func (m *orderedMap[K,V]) configJSON(w *configJSONWriter) jsString {
 if m==nil {return str("null")};w.enter(m);defer w.leave(m)
 entries:=[]entry[jsString,any]{};for _,e:=range m.Entries {k,ok:=any(e.Key).(jsString);if !ok {panic(newTrap("config JSON requires string record keys"))};entries=append(entries,entry[jsString,any]{k,e.Value})};return w.object(entries)
}
func jsonIndex(k jsString) (uint64,bool) {s:=k.String();n,err:=strconv.ParseUint(s,10,32);return n,err==nil&&n<4294967295&&strconv.FormatUint(n,10)==s}
func (w *configJSONWriter) object(entries []entry[jsString,any]) jsString {
 // ES own-key ordering: canonical array indices first, other strings in insertion order.
 indexed:=[]entry[jsString,any]{};rest:=[]entry[jsString,any]{}
 for _,e:=range entries {if _,ok:=jsonIndex(e.Key);ok {indexed=append(indexed,e)}else{rest=append(rest,e)}}
 for i:=1;i<len(indexed);i++ {for j:=i;j>0;j-- {a,_:=jsonIndex(indexed[j-1].Key);b,_:=jsonIndex(indexed[j].Key);if a<=b {break};indexed[j-1],indexed[j]=indexed[j],indexed[j-1]}}
 out:=str("{");comma:=false;for _,e:=range append(indexed,rest...) {s,ok:=w.value(e.Value);if !ok {continue};if comma {out+=str(",")};comma=true;out+=jsonQuote(e.Key)+str(":")+s};return out+str("}")
}
func (w *configJSONWriter) value(v any) (jsString,bool) {
 if o,ok:=v.(interface{configJSONOptional()(any,bool)});ok {value,has:=o.configJSONOptional();if !has {return "",false};return w.value(value)}
 if nilRef(v) {return "",false}
 switch x:=v.(type) {
 case jsString:return jsonQuote(x),true
 case bool:return str(strconv.FormatBool(x)),true
 case int32:return str(strconv.FormatInt(int64(x),10)),true
 case int64: if x>9007199254740991||x< -9007199254740991 {panic(newTrap("config JSON integer outside JS safe domain"))};return str(strconv.FormatInt(x,10)),true
 case float64:return jsonNumber(x),true
 case *dynamic:
  if x.Tag==tagNull {return str("null"),true};if x.Tag==10 {return "",false};return w.value(x.Value)
 }
 if s,ok:=v.(interface{configJSON(*configJSONWriter)jsString});ok {return s.configJSON(w),true}
 if s,ok:=v.(interface{dynamicSource()*dynamic});ok {if d:=s.dynamicSource();d!=nil {return w.value(d)}}
 panic(newTrap("unsupported config JSON value"))
}
func jsonNumber(n float64) jsString {
 if math.IsNaN(n)||math.IsInf(n,0) {return str("null")};if n==0 {return str("0")}
 format:=byte('f');a:=math.Abs(n);if a<1e-6||a>=1e21 {format='e'}
 s:=strconv.FormatFloat(n,format,-1,64)
 if at:=strings.IndexByte(s,'e');at>=0 {exponent,_:=strconv.Atoi(s[at+1:]);sign:="";if exponent>=0 {sign="+"};s=s[:at+1]+sign+strconv.Itoa(exponent)}
 return str(s)
}
func jsonQuote(s jsString) jsString {
 out:=str("\"");hex:="0123456789abcdef"
 for i:=int32(0);i<s.length();i++ {c:=s.charCodeAt(i)
  switch c {case 34:out+=str("\\\"");case 92:out+=str("\\\\");case 8:out+=str("\\b");case 9:out+=str("\\t");case 10:out+=str("\\n");case 12:out+=str("\\f");case 13:out+=str("\\r");default:
   lone:=c>=0xd800&&c<=0xdfff
   if c>=0xd800&&c<=0xdbff&&i+1<s.length() {d:=s.charCodeAt(i+1);if d>=0xdc00&&d<=0xdfff {out+=unit(c)+unit(d);i++;continue}}
   if c<32||lone {out+=str("\\u"+string([]byte{hex[(c>>12)&15],hex[(c>>8)&15],hex[(c>>4)&15],hex[c&15]}))}else{out+=unit(c)}
  }
 };return out+str("\"")
}
`
