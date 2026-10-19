package hir

import (
	"math"
	"strconv"
)

func (p *Program) StructuralUnion(name string) []StructuralMember {
	for _, i := range p.Interfaces {
		if i.Name == name {
			return i.StructuralMembers
		}
	}
	return nil
}

func (p *Program) StructuralArrayUnion(t Type) bool {
	if t.Kind != Array || len(t.Args) != 1 {
		return false
	}
	elem := t.Args[0]
	for elem.Kind == Optional || elem.Kind == Array {
		if len(elem.Args) != 1 {
			return false
		}
		elem = elem.Args[0]
	}
	return elem.Kind == InterfaceRef && len(p.StructuralUnion(elem.Name)) > 0
}

func (v *verifier) structuralUnions() {
	for _, i := range v.p.Interfaces {
		if i == nil || len(i.StructuralMembers) == 0 {
			continue
		}
		if len(i.StructuralMembers) < 2 {
			v.fail(i.Node, "structural union needs multiple alternatives")
		}
		seen := map[string]bool{}
		for _, m := range i.StructuralMembers {
			v.typ(i.Node, m.Type)
			c := v.classes[m.Type.Name]
			if m.Type.Kind != ClassRef || c == nil || !c.Structural || !DataClassView(v.p, c.Name) || seen[m.Type.Name] {
				v.fail(i.Node, "invalid structural union alternative")
				continue
			}
			seen[m.Type.Name] = true
			implements := false
			for _, name := range c.Implements {
				implements = implements || name == i.Name
			}
			if !implements {
				v.fail(i.Node, "structural union payload does not implement its interface")
			}
			fields := map[string]Type{}
			ancestors := map[string]bool{}
			for base := c; base != nil && !ancestors[base.Name]; base = v.classes[base.Super] {
				ancestors[base.Name] = true
				for _, f := range base.Fields {
					if !f.Static {
						fields[f.Name] = f.Type
					}
				}
			}
			for _, f := range m.Fields {
				v.typ(i.Node, f.Type)
				t, ok := fields[f.Name]
				if !ok || !t.Equal(f.Type) {
					v.fail(i.Node, "structural union field does not match payload layout")
				}
				delete(fields, f.Name)
				base := f.Type
				if base.Kind == Optional {
					if len(base.Args) != 1 {
						continue
					}
					base = base.Args[0]
				}
				if f.Optional && f.Type.Kind != Optional && !base.IsRef() {
					v.fail(i.Node, "structural optional field needs optional storage")
				}
				for _, d := range f.Domains {
					compatible := base.Kind == Dynamic
					switch d.Kind {
					case String, Bool, Array, ClassValue:
						compatible = compatible || base.Kind == d.Kind
					case Number:
						compatible = compatible || base.Kind == Number || base.Kind == I32 || base.Kind == I64
					case ClassRef:
						compatible = compatible || base.Kind == ClassRef || base.Kind == InterfaceRef || base.Kind == UnionRef || base.Kind == OrderedMap || base.Kind == OrderedSet || base.Kind == RegExp
					case "undefined":
						compatible = compatible || f.Type.Kind == Optional
					case "null":
					case Dynamic:
					}
					if !compatible {
						v.fail(i.Node, "structural union domain exceeds its field storage")
					}
					switch d.Kind {
					case Dynamic, String, Number, Bool, Array, ClassRef, ClassValue, "null", "undefined":
					default:
						v.fail(i.Node, "invalid structural union domain")
					}
					if d.Exact && d.Kind == Number {
						n, err := strconv.ParseFloat(d.Literal, 64)
						if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
							v.fail(i.Node, "invalid structural numeric literal")
						}
					}
					if d.Exact && d.Kind == Bool && d.Literal != "true" && d.Literal != "false" {
						v.fail(i.Node, "invalid structural boolean literal")
					}
					if d.Exact && d.Kind != String && d.Kind != Number && d.Kind != Bool {
						v.fail(i.Node, "invalid structural union literal domain")
					}
				}
			}
			if len(fields) > 0 {
				v.fail(i.Node, "incomplete structural union fields")
			}
		}
	}
}
