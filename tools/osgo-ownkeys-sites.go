//go:build ignore

// Supplement the production --gap-only report: already catalogued operations
// with runtime ABI refusals are absent from its unsupported-op inventory.
// Usage: go run tools/osgo-ownkeys-sites.go CLOSURE > supported-sites.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront"
)

func visit(s *hir.Stmt, f func(*hir.Expr)) {
	if s == nil {
		return
	}
	var expr func(*hir.Expr)
	expr = func(x *hir.Expr) {
		if x == nil {
			return
		}
		f(x)
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, a := range x.Args {
			expr(a)
		}
		visit(x.Stmt, f)
	}
	expr(s.X)
	expr(s.Y)
	visit(s.Body, f)
	visit(s.Else, f)
	for _, child := range s.List {
		visit(child, f)
	}
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: osgo-ownkeys-sites CLOSURE")
	}
	root, err := filepath.Abs(os.Args[1])
	if err != nil {
		panic(err)
	}
	lowering, err := tsfront.LowerAbaplintTests(root)
	if err != nil {
		panic(err)
	}
	if err := lowering.Err(); err != nil {
		panic(err)
	}
	p := lowering.Prog
	hir.AssignSiteIDs(p)
	classes := map[string]*hir.Class{}
	for _, c := range p.Classes {
		classes[c.Name] = c
	}
	sites := map[string]map[string]bool{}
	for _, c := range p.Classes {
		methods := append([]*hir.Method{}, c.Methods...)
		if c.Ctor != nil {
			methods = append([]*hir.Method{c.Ctor}, methods...)
		}
		for _, m := range methods {
			ordinal := 0
			visit(m.Body, func(x *hir.Expr) {
				ordinal++
				if x.Kind != hir.RuntimeOp {
					return
				}
				reason := ""
				switch x.Op {
				case "dynamic.keys", "dynamic.get", "dynamic.put":
					reason = x.Op
				case "dynamic.classView":
					for base := classes[x.Type.Name]; base != nil; base = classes[base.Super] {
						if base.PrimitiveReads {
							reason = "dynamic.classView/PrimitiveReads"
							break
						}
					}
				}
				if reason == "" {
					return
				}
				if sites[reason] == nil {
					sites[reason] = map[string]bool{}
				}
				source := strings.TrimPrefix(x.Source, root+string(filepath.Separator))
				if filepath.IsAbs(source) {
					source = filepath.Base(source)
				}
				sites[reason][fmt.Sprintf("%s.%s:%d:%d (%s)", c.Name, m.Name, x.ID, ordinal, source)] = true
			})
		}
	}
	type row struct {
		Reason   string   `json:"reason"`
		Sites    int      `json:"sites"`
		Examples []string `json:"examples"`
	}
	out := []row{}
	for reason, ss := range sites {
		examples := []string{}
		for site := range ss {
			examples = append(examples, site)
		}
		sort.Strings(examples)
		n := len(examples)
		if n > 3 {
			examples = examples[:3]
		}
		out = append(out, row{reason, n, examples})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Reason < out[j].Reason })
	result := struct {
		Files, Classes, Interfaces int
		Uses                       []row
	}{len(lowering.Files), len(p.Classes), len(p.Interfaces), out}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		panic(err)
	}
}
