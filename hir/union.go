package hir

// UnionLiteral retains checker-proven string domains, including secondary tags.
type UnionLiteral struct {
	Field  string
	Values []string
}

func unionDomainIncludes(dst, src []string) bool {
	for _, s := range src {
		found := false
		for _, d := range dst {
			found = found || s == d
		}
		if !found {
			return false
		}
	}
	return true
}

func unionLiteralsInclude(dst, src []UnionLiteral) bool {
	for _, d := range dst {
		found := false
		for _, s := range src {
			if s.Field == d.Field && unionDomainIncludes(d.Values, s.Values) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

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
			literalFields := map[string]bool{}
			for _, f := range a.Literals {
				slot := v.field(a.Payload.Name, f.Field)
				if f.Field == "" || literalFields[f.Field] || len(f.Values) == 0 || slot == nil || !slot.Type.Equal(T(String)) {
					v.fail(u.Node, "invalid union literal field")
				}
				literalFields[f.Field] = true
				values := map[string]bool{}
				for _, value := range f.Values {
					if values[value] {
						v.fail(u.Node, "duplicate union literal value")
					}
					values[value] = true
				}
				if f.Field == u.Discriminant && (!unionDomainIncludes(f.Values, a.Tags) || !unionDomainIncludes(a.Tags, f.Values)) {
					v.fail(u.Node, "union tag literal domain mismatch")
				}
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
			if a.Payload.Equal(b.Payload) && unionDomainIncludes(b.Tags, a.Tags) && unionLiteralsInclude(b.Literals, a.Literals) {
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
	case "union.check":
		u := v.p.Union(e.Type.Name)
		if recv.Kind != Dynamic || e.Type.Kind != UnionRef || u == nil || !u.Complete {
			v.fail(e.Node, "checked union admission needs Dynamic and a complete family")
		}
	case "union.admit", "union.project":
		if e.Op == "union.project" && e.Type.Kind == UnionRef {
			if recv.Kind != UnionRef || !UnionIncludes(v.p.Union(recv.Name), v.p.Union(e.Type.Name)) {
				v.fail(e.Node, "invalid union subset projection")
			}
			return
		}
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
						if i < len(e.X.Args) && e.X.Args[i].Kind == Lit {
							for _, field := range a.Literals {
								if field.Field == p.Name {
									ok := false
									for _, value := range field.Values {
										ok = ok || e.X.Args[i].Value == value
									}
									if !ok {
										v.fail(e.Node, "union admission has wrong literal field")
									}
								}
							}
						}
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
			if f == nil && e.Type.Kind == Optional {
				continue // absent in this member: the read yields undefined
			}
			if f == nil || f.Static || f.Private || !f.Type.Equal(e.Type) && !(e.Type.Kind == Dynamic && unionScalarField(f.Type)) && !(e.Type.Kind == Optional && len(e.Type.Args) == 1 && f.Type.Equal(e.Type.Args[0])) && !UnionReadAccepts(v.p, e.Type, f.Type) {
				v.fail(e.Node, "union field is not compatible in every alternative")
			}
		}
	}
}

// UnionReadAccepts reports whether a family field stored as have may be read
// through the common view want: the same type, a completed subset of the
// family want names, or one of its alternative payloads. The read validates
// the stored value against want, so membership is checked, never assumed.
func UnionReadAccepts(p *Program, want, have Type) bool {
	if want.Kind == Dynamic && have.Kind != Void && have.Kind != "" {
		return true // a read boxes the actual value, including absence
	}
	if want.Equal(have) {
		return true
	}
	if want.Kind == Optional && len(want.Args) == 1 {
		if have.Kind == Optional && len(have.Args) == 1 {
			have = have.Args[0]
		}
		return UnionReadAccepts(p, want.Args[0], have)
	}
	if want.Kind != UnionRef || p == nil {
		return false
	}
	u := p.Union(want.Name)
	if u == nil || !u.Complete {
		return false
	}
	switch have.Kind {
	case UnionRef:
		return UnionIncludes(u, p.Union(have.Name))
	case ClassRef:
		for _, a := range u.Alternatives {
			if a.Payload.Equal(have) {
				return true
			}
		}
	}
	return false
}

func unionScalarField(t Type) bool {
	switch t.Kind {
	case String, Number, I32, I64, Bool, Dynamic:
		return true
	}
	return false
}
