package hir

import "strings"

func (p *Program) Union(name string) *UnionDecl {
	for _, u := range p.Unions {
		if u != nil && u.Name == name {
			return u
		}
	}
	return nil
}

func (u *UnionDecl) Alternative(id string) *UnionAlternative {
	if u == nil {
		return nil
	}
	for i := range u.Alternatives {
		if u.Alternatives[i].ID == id {
			return &u.Alternatives[i]
		}
	}
	return nil
}

func (v *verifier) unionDeclarations() {
	seen := map[string]bool{}
	for _, u := range v.p.Unions {
		if u == nil {
			v.fail(Node{}, "nil union declaration")
			continue
		}
		if seen[u.Name] || u.Name == "" || !u.Complete || u.Discriminant == "" || len(u.Alternatives) == 0 {
			v.fail(u.Node, "duplicate, empty or incomplete union "+u.Name)
		}
		seen[u.Name] = true
		ids := map[string]bool{}
		for _, a := range u.Alternatives {
			if a.ID == "" || ids[a.ID] || len(a.Tags) == 0 {
				v.fail(u.Node, "invalid union alternative "+a.ID)
			}
			ids[a.ID] = true
			tags := map[string]bool{}
			for _, tag := range a.Tags {
				if tags[tag] {
					v.fail(u.Node, "duplicate union tag in "+a.ID)
				}
				tags[tag] = true
			}
			v.typ(u.Node, a.Payload)
			c := v.classes[a.Payload.Name]
			if a.Payload.Kind != ClassRef || c == nil || c.Ctor == nil || !c.Structural || !DataClassView(v.p, c.Name) {
				v.fail(u.Node, "union payload needs a completed structural data layout")
				continue
			}
			f := v.field(c.Name, u.Discriminant)
			if f == nil || f.Static || !f.Type.Equal(T(String)) {
				v.fail(u.Node, "union discriminant needs a required string field")
			}
		}
	}
}

// UnionIncludes compares names and layouts rather than recursively expanding
// field types. Mutable containers keep their existing invariant typing rules.
func UnionIncludes(dst, src *UnionDecl) bool {
	if dst == nil || src == nil || !dst.Complete || !src.Complete || dst.Discriminant != src.Discriminant {
		return false
	}
	for _, a := range src.Alternatives {
		found := false
		for _, b := range dst.Alternatives {
			if a.Payload.Equal(b.Payload) && strings.Join(a.Tags, "\x00") == strings.Join(b.Tags, "\x00") {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (v *verifier) unionOp(e *Expr, recv Type) {
	if len(e.Args) != 0 {
		v.fail(e.Node, "union operation takes no arguments")
	}
	switch e.Op {
	case "union.admit", "union.project":
		family := v.p.Union(e.Owner)
		a := family.Alternative(e.Name)
		if family == nil || !family.Complete || a == nil {
			v.fail(e.Node, "unresolved union operation alternative")
			return
		}
		union := Type{Kind: UnionRef, Name: family.Name}
		if e.Op == "union.admit" {
			if !recv.Equal(a.Payload) || !e.Type.Equal(union) {
				v.fail(e.Node, "union admission payload mismatch")
			}
			// Do not let an obviously wrong literal hide behind string storage.
			if e.X != nil && e.X.Kind == New {
				ctor := v.p.Constructor(a.Payload.Name)
				if ctor != nil {
					for i, p := range ctor.Params {
						if p.Name == family.Discriminant && i < len(e.X.Args) && e.X.Args[i].Kind == Lit {
							ok := false
							for _, tag := range a.Tags {
								ok = ok || e.X.Args[i].Value == tag
							}
							if !ok {
								v.fail(e.Node, "union admission has wrong literal tag")
							}
						}
					}
				}
			}
		} else {
			if !recv.Equal(union) || !e.Type.Equal(a.Payload) {
				v.fail(e.Node, "union projection payload mismatch")
			}
			for _, other := range family.Alternatives {
				if other.ID == a.ID {
					continue
				}
				for _, tag := range other.Tags {
					for _, own := range a.Tags {
						if tag == own {
							v.fail(e.Node, "union projection requires a unique tag")
						}
					}
				}
			}
		}
	case "union.widen":
		if recv.Kind != UnionRef || e.Type.Kind != UnionRef || !UnionIncludes(v.p.Union(e.Type.Name), v.p.Union(recv.Name)) {
			v.fail(e.Node, "invalid union widening")
		}
	case "union.tag", "union.read":
		u := v.p.Union(recv.Name)
		if recv.Kind != UnionRef || u == nil || !u.Complete {
			v.fail(e.Node, "union read needs a completed family")
			return
		}
		if e.Op == "union.tag" {
			if e.Name != u.Discriminant || !e.Type.Equal(T(String)) {
				v.fail(e.Node, "invalid union tag read")
			}
			return
		}
		for _, a := range u.Alternatives {
			f := v.field(a.Payload.Name, e.Name)
			if f == nil || f.Static || f.Private || !f.Type.Equal(e.Type) {
				v.fail(e.Node, "union field is not compatible in every alternative")
			}
		}
	}
}
