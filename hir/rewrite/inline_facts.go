package rewrite

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// The extractor records syntax and hierarchy, not an inlining decision. Shape
// conversion is shared with the inline action so template guards are exact.
func (r *runner) addInlineFacts() error {
	var addErr error
	add := func(p string, a ...string) {
		if addErr == nil {
			addErr = r.db.Add(p, a...)
		}
	}
	classes := map[string]*hir.Class{}
	for _, c := range r.p.Classes {
		classes[c.Name] = c
	}
	names := map[string]bool{}
	for _, c := range r.p.Classes {
		for _, m := range c.Methods {
			names[m.Name] = true
		}
	}
	for _, c := range r.p.Classes {
		for k := c; k != nil; k = classes[k.Super] {
			add("inline_ancestor", c.Name, k.Name)
		}
		for _, m := range c.Methods {
			if r.refresh != nil && !r.refresh[m] {
				continue
			}
			id := r.methods[m]
			add("inline_decl", c.Name, m.Name, id)
			// Every declared base name, including inherited methods, blocks variants.
			for start := 0; start < len(m.Name); {
				pos := strings.Index(m.Name[start:], "_instantiated_")
				if pos < 0 {
					break
				}
				pos += start
				if names[m.Name[:pos]] {
					add("inline_variant", c.Name, m.Name[:pos])
				}
				start = pos + 1
			}
			add("inline_owner", id, c.Name)
			if m.Virtual {
				add("inline_virtual", id)
			}
			add("inline_name", id, m.Name)
			add("inline_parameters", id, strconv.Itoa(len(m.Params)))
			if m.Abstract {
				add("inline_abstract", id)
			}
			for _, p := range m.Params {
				if p.Variadic {
					add("inline_variadic", id)
				}
			}
			if m.Body == nil {
				continue
			}
			if m.Body.Kind == hir.Block {
				add("inline_block", id)
			}
			size := 0
			visitTree(m.Body, "", func(s *hir.Stmt, _ string) {
				add("inline_stmt", id, string(s.Kind))
				if s.Kind == hir.VarDecl && s.X == nil {
					add("inline_uninitialized", id)
				}
				if s.Kind != hir.Block {
					size++
				}
			}, func(e *hir.Expr, _ string) {
				add("inline_expr", id, string(e.Kind))
				if e.Kind == hir.Seq {
					visitTree(e.Stmt, "", func(s *hir.Stmt, _ string) {
						if s.Kind == hir.Return {
							add("inline_seq_return", id)
						}
					}, func(*hir.Expr, string) {})
				}
			})
			add("inline_size", id, strconv.Itoa(size))
			if r.db.demanded == nil || r.db.demanded["inline_template"] {
				if _, ok := makeTemplate(m); ok {
					add("inline_template", id)
				}
			}
		}
	}
	for _, n := range r.nodes {
		if r.refresh != nil && !r.refresh[n.method] {
			continue
		}
		e := n.expr
		if e == nil || e.Kind != hir.VirtualCall || e.X == nil || e.X.Type.Kind != hir.ClassRef {
			continue
		}
		add("inline_call", r.methods[n.method], n.id, e.X.Type.Name, e.Name)
		add("inline_arguments", n.id, strconv.Itoa(len(e.Args)))
	}
	return addErr
}

// Inline runs the embedded Grace inliner. Its policy is entirely in inline.grace.
func Inline(p *hir.Program) (Stats, error) {
	b, err := ruleFiles.ReadFile("rules/inline.grace")
	if err != nil {
		return Stats{}, err
	}
	_, rs, err := Parse(string(b))
	if err != nil {
		return Stats{}, fmt.Errorf("inline rules: %w", err)
	}
	return Rewrite(p, rs, Limits{})
}

// ExtractRewriteFacts returns a read-only snapshot of base HIR facts plus node
// and native inline shape facts, before any Grace rule is evaluated. This is
// useful for diagnostics and independent evaluators of the rewrite rules.
// Like Analyze, it requires verified input and leaves the program unchanged.
func ExtractRewriteFacts(p *hir.Program) (*DB, error) {
	if es := hir.Verify(p); len(es) > 0 {
		return nil, fmt.Errorf("invalid HIR: %v", es)
	}
	r := &runner{p: p, db: Extract(p)}
	if err := r.index(); err != nil {
		return nil, err
	}
	if err := r.addInlineFacts(); err != nil {
		return nil, err
	}
	return r.db, nil
}

// Custom rule sets can relate arbitrary regions; the fixed inline rule set has
// the method-local ownership and call dependencies declared by inlineRegion.
func isInlineRules(rules *Rules) bool {
	b, err := ruleFiles.ReadFile("rules/inline.grace")
	if err != nil {
		return false
	}
	_, embedded, err := Parse(string(b))
	return err == nil && reflect.DeepEqual(rules, embedded)
}
func (r *runner) inlineRegion(pred string, args Tuple) string {
	return r.inlineRegionValue(pred, args[0])
}
func (r *runner) inlineRegionValue(pred, value string) string {
	switch pred {
	case "inline_call", "inline_dispatch", "inline_forbidden", "inline_candidate", "inline_edge", "inline_path",
		"inline_block", "inline_size", "inline_stmt", "inline_expr", "inline_seq_return", "inline_uninitialized", "inline_template":
		return value
	case "node", "inline_arguments", "inline_arity", "inline_allowed":
		if n, ok := r.nodes[value]; ok {
			return r.methods[n.method]
		}
	}
	// Hierarchy and declaration facts are immutable. inline_overridden is shared
	// between callers, so it is conservatively recomputed as a whole relation.
	return ""
}
func (r *runner) inlineDependants(changed map[string]bool) map[string]bool {
	reverse := map[string][]string{}
	if table := r.db.tables["inline_dispatch"]; table != nil {
		for _, row := range table.rows {
			reverse[row.args[4]] = append(reverse[row.args[4]], row.args[0])
		}
	}
	return dependentRegions(changed, reverse)
}

func inlineRegionalHead(pred string) bool {
	switch pred {
	case "inline_dispatch", "inline_forbidden", "inline_candidate", "inline_edge", "inline_path", "inline_arity", "inline_allowed":
		return true
	}
	return false
}
