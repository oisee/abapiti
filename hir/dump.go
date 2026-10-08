package hir

import (
	"fmt"
	"strings"
)

func Dump(p *Program) string {
	var b strings.Builder
	b.WriteString("hir v1\n")
	method := func(m *Method) {
		fmt.Fprintf(&b, "  method %s", m.Name)
		if m.Static {
			b.WriteString(" static")
		}
		if m.Virtual {
			b.WriteString(" virtual")
		}
		if m.Abstract {
			b.WriteString(" abstract")
		}
		b.WriteString("(")
		for i, p := range m.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s:%s", p.Name, p.Type)
		}
		fmt.Fprintf(&b, "):%s\n", m.Result)
		dumpStmt(&b, m.Body, "    ")
	}
	for _, i := range p.Interfaces {
		fmt.Fprintf(&b, "interface %s\n", i.Name)
		for _, m := range i.Methods {
			method(m)
		}
	}
	for _, c := range p.Classes {
		fmt.Fprintf(&b, "class %s", c.Name)
		if c.Super != "" {
			fmt.Fprintf(&b, " extends %s", c.Super)
		}
		if c.Abstract {
			b.WriteString(" abstract")
		}
		if len(c.Implements) > 0 {
			fmt.Fprintf(&b, " implements %s", strings.Join(c.Implements, ", "))
		}
		b.WriteByte('\n')
		for _, f := range c.Fields {
			s := ""
			if f.Static {
				s = " static"
			}
			fmt.Fprintf(&b, "  field %s:%s%s\n", f.Name, f.Type, s)
		}
		if c.Ctor != nil {
			b.WriteString("  constructor\n")
			method(c.Ctor)
		}
		for _, m := range c.Methods {
			method(m)
		}
	}
	return b.String()
}
func dumpStmt(b *strings.Builder, s *Stmt, indent string) {
	if s == nil {
		return
	}
	line := func(f string, a ...any) { fmt.Fprintf(b, indent+f+"\n", a...) }
	switch s.Kind {
	case Block:
		for _, x := range s.List {
			dumpStmt(b, x, indent)
		}
	case VarDecl:
		line("var %s:%s = %s", s.Name, s.Type, dumpExpr(s.X))
	case Assign:
		line("%s = %s", dumpExpr(s.X), dumpExpr(s.Y))
	case ExprStmt:
		line("%s", dumpExpr(s.X))
	case If, While:
		line("%s %s", s.Kind, dumpExpr(s.X))
		dumpStmt(b, s.Body, indent+"  ")
		if s.Else != nil {
			line("else")
			dumpStmt(b, s.Else, indent+"  ")
		}
		line("end %s", s.Kind)
	case ForEach:
		line("foreach %s:%s in %s", s.Name, s.Type, dumpExpr(s.X))
		dumpStmt(b, s.Body, indent+"  ")
		line("end foreach")
	case Try:
		line("try")
		dumpStmt(b, s.Body, indent+"  ")
		line("catch %s:%s", s.Name, s.Type)
		dumpStmt(b, s.Else, indent+"  ")
		line("end try")
	case Trap:
		line("trap %s", s.Name)
	case Return, Throw:
		line("%s %s", s.Kind, dumpExpr(s.X))
	default:
		line("%s", s.Kind)
	}
}
func dumpExpr(e *Expr) string {
	if e == nil {
		return "_"
	}
	s := ""
	args := []string{}
	for _, a := range e.Args {
		args = append(args, dumpExpr(a))
	}
	a := strings.Join(args, ", ")
	switch e.Kind {
	case Lit:
		s = fmt.Sprintf("%v", e.Value)
		if x, ok := e.Value.(string); ok {
			s = fmt.Sprintf("%q", x)
		}
		if e.Value == nil {
			s = "undefined"
		}
	case NumericMinMax:
		s = "Math." + e.Op + "(" + dumpExpr(e.X) + ", " + dumpExpr(e.Y) + ")"
	case CheckedNumericConvert:
		if e.Range == nil {
			s = "checked_integer[invalid](" + dumpExpr(e.X) + ")"
		} else {
			s = fmt.Sprintf("checked_integer[%d,%d](%s)", e.Range.Min, e.Range.Max, dumpExpr(e.X))
		}
	case NumericConvert:
		s = "numeric_convert(" + dumpExpr(e.X) + ")"
	case Local:
		s = e.Name
	case This:
		s = "this"
	case FieldGet:
		s = dumpExpr(e.X) + "." + e.Name
	case StaticGet:
		s = e.Owner + "." + e.Name
	case IndexGet:
		s = dumpExpr(e.X) + "[" + dumpExpr(e.Y) + "]"
	case New:
		s = "new " + e.Type.String() + "(" + a + ")"
	case DirectCall, VirtualCall, SuperCall:
		s = string(e.Kind) + " " + e.Owner
		if e.X != nil {
			s += " " + dumpExpr(e.X)
		}
		s += "." + e.Name + "(" + a + ")"
	case Binary:
		s = dumpExpr(e.X) + " " + e.Op + " " + dumpExpr(e.Y)
	case Unary:
		s = e.Op + dumpExpr(e.X)
	case Conditional:
		s = "if " + dumpExpr(e.X) + " then " + dumpExpr(e.Y) + " else " + dumpExpr(e.Z)
	case InstanceOf:
		s = dumpExpr(e.X) + " instanceof " + e.Owner
		if e.Y != nil {
			s = dumpExpr(e.X) + " instanceof " + dumpExpr(e.Y)
		}
	case Narrow:
		s = "narrow " + dumpExpr(e.X) + " to " + e.Type.String()
	case Cast:
		s = "cast " + dumpExpr(e.X) + " to " + e.Type.String()
	case ClassOf:
		s = "classof " + e.Owner
	case Seq:
		s = "seq " + dumpExpr(e.Y)
	case RuntimeOp:
		s = e.Op + "(" + dumpExpr(e.X)
		if a != "" {
			s += ", " + a
		}
		s += ")"
	default:
		s = string(e.Kind) + "(" + dumpExpr(e.X) + ")"
	}
	return "(" + s + "):" + e.Type.String()
}
