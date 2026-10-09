package abap

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// SourceEntry maps one emitted ABAP name back to its HIR identity and, for
// classes, interfaces and methods, the TypeScript source location.
type SourceEntry struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Source   string   `json:"source,omitempty"`
	Declared []string `json:"declared,omitempty"` // member: Class.method@file:line
}

// relSource drops the build directory: src/…, harness/… or node_modules/… with line:col.
func relSource(s string) string {
	for _, root := range []string{"/node_modules/", "/harness/", "/src/"} {
		if i := strings.Index(s, root); i >= 0 {
			return s[i+1:]
		}
	}
	return s
}

// SourceMap returns names.json: every emitted name (upper case, as the ABAP
// kernel shows it in traces and dumps) with its HIR identity and location.
func SourceMap(p *hir.Program, names *hir.Names) ([]byte, error) {
	classes := map[string]*hir.Class{}
	for _, c := range p.Classes {
		classes[c.Name] = c
	}
	ifaces := map[string]*hir.Interface{}
	for _, i := range p.Interfaces {
		ifaces[i.Name] = i
	}
	declared := map[string][]string{}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			declared["member."+m.Name] = append(declared["member."+m.Name], c.Name+"."+m.Name+"@"+relSource(m.Source))
		}
		for _, f := range c.Fields {
			declared["member."+f.Name] = append(declared["member."+f.Name], c.Name+"."+f.Name+"@"+relSource(f.Source))
		}
	}
	out := map[string]SourceEntry{}
	for name, id := range names.Pairs() {
		e := SourceEntry{ID: id, Kind: "runtime"}
		switch {
		case classes[id] != nil:
			e.Kind, e.Source = "class", relSource(classes[id].Source)
		case ifaces[id] != nil:
			e.Kind, e.Source = "interface", relSource(ifaces[id].Source)
		case strings.HasPrefix(id, "member."):
			e.Kind = "member"
			e.Declared = declared[id]
			sort.Strings(e.Declared)
		case strings.HasPrefix(id, "param."):
			e.Kind = "parameter"
		case strings.HasPrefix(id, "exception."):
			e.Kind = "exception"
		case strings.HasPrefix(id, "builtin."):
			e.Kind = "builtin"
		}
		out[strings.ToUpper(name)] = e
	}
	return json.MarshalIndent(out, "", " ")
}
