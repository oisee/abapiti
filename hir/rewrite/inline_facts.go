package rewrite

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// The extractor records syntax and hierarchy, not an inlining decision. Shape
// conversion is shared with the inline action so template guards are exact.
func (r *runner) addInlineFacts() error {
	add := func(p string, a ...string) { r.db.Add(p, a...) }
	classes := map[string]*hir.Class{}
	for _, c := range r.p.Classes {
		classes[c.Name] = c
	}
	for _, c := range r.p.Classes {
		for k := c; k != nil; k = classes[k.Super] {
			add("inline_ancestor", c.Name, k.Name)
		}
		for _, m := range c.Methods {
			id := r.methods[m]
			add("inline_decl", c.Name, m.Name, id)
			for _, other := range c.Methods {
				if strings.HasPrefix(other.Name, m.Name+"_instantiated_") {
					add("inline_variant", c.Name, m.Name)
				}
			}
			// Variants of inherited methods also block dispatch.
			for _, d := range r.p.Classes {
				for _, base := range d.Methods {
					if strings.HasPrefix(m.Name, base.Name+"_instantiated_") {
						add("inline_variant", c.Name, base.Name)
					}
				}
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
			visitTree(m.Body, id+"/body", func(s *hir.Stmt, _ string) {
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
			if _, ok := makeTemplate(m); ok {
				add("inline_template", id)
			}
		}
	}
	for _, n := range r.nodes {
		e := n.expr
		if e == nil || e.Kind != hir.VirtualCall || e.X == nil || e.X.Type.Kind != hir.ClassRef {
			continue
		}
		add("inline_call", r.methods[n.method], n.id, e.X.Type.Name, e.Name)
		add("inline_arguments", n.id, strconv.Itoa(len(e.Args)))
	}
	return nil
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
	r.index()
	if err := r.addInlineFacts(); err != nil {
		return nil, err
	}
	return r.db, nil
}
