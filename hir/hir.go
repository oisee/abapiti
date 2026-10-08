// Package hir defines the resolved, semantic object IR shared by frontends and backends.
package hir

import "strings"

type Kind string

const (
	Bool         Kind = "bool"
	Number       Kind = "number"
	I32          Kind = "i32"
	I64          Kind = "i64"
	String       Kind = "string"
	Optional     Kind = "optional"
	ClassRef     Kind = "classref"
	InterfaceRef Kind = "interfaceref"
	ClassValue   Kind = "classvalue"
	Array        Kind = "array"
	OrderedMap   Kind = "map"
	OrderedSet   Kind = "set"
	RegExp       Kind = "regexp"
	// Dynamic is a tagged union value (string, class value or object
	// reference in one variable): JavaScript `typeof` dispatch works on it.
	Dynamic Kind = "dynamic"
	Void    Kind = "void"
)

// Type describes meaning, never a target spelling. Number is binary64 and
// String consists of UTF-16 code units. Generic arguments are resolved.
type Type struct {
	Kind Kind
	Name string
	Args []Type
}

func T(k Kind, args ...Type) Type { return Type{Kind: k, Args: args} }
func Ref(name string) Type        { return Type{Kind: ClassRef, Name: name} }
func (t Type) String() string {
	s := string(t.Kind)
	if t.Name != "" {
		s += "<" + t.Name + ">"
	}
	if len(t.Args) > 0 {
		a := []string{}
		for _, x := range t.Args {
			a = append(a, x.String())
		}
		s += "<" + strings.Join(a, ",") + ">"
	}
	return s
}
func (t Type) Equal(u Type) bool { return t.String() == u.String() }
func (t Type) IsRef() bool {
	return t.Kind == ClassRef || t.Kind == InterfaceRef || t.Kind == Array || t.Kind == OrderedMap || t.Kind == OrderedSet || t.Kind == RegExp || t.Kind == Dynamic || t.Kind == ClassValue
}

type Node struct {
	ID     int
	Source string
}
type Program struct {
	Classes    []*Class
	Interfaces []*Interface
}
type Class struct {
	Node
	Name, Super string
	Implements  []string
	Abstract    bool
	Fields      []Field
	Methods     []*Method
	Ctor        *Method
}
type Interface struct {
	Node
	Name    string
	Methods []*Method
}
type Field struct {
	Node
	Name              string
	Type              Type
	Static            bool
	Private, Readonly bool
}
type Param struct {
	Name string
	Type Type
	// Variadic marks the trailing `...rest` parameter: callers may pass any
	// number of extra arguments, collected into this array.
	Variadic bool
}
type Method struct {
	Node
	Name                      string
	Params                    []Param
	Result                    Type
	Static, Virtual, Abstract bool
	Internal                  bool // private or unexported TS callable
	Body                      *Stmt
}

// Tagged nodes make traversal and serialization explicit. X/Y/Z are operands,
// Args are evaluated left to right. Conditional and boolean &&/|| are lazy.
type ExprKind string

const (
	Lit                   ExprKind = "lit"
	Local                 ExprKind = "local"
	This                  ExprKind = "this"
	FieldGet              ExprKind = "field"
	StaticGet             ExprKind = "static"
	IndexGet              ExprKind = "index"
	DirectCall            ExprKind = "call"
	VirtualCall           ExprKind = "virtual"
	SuperCall             ExprKind = "super"
	New                   ExprKind = "new"
	Binary                ExprKind = "binary"
	Unary                 ExprKind = "unary"
	Conditional           ExprKind = "conditional"
	InstanceOf            ExprKind = "instanceof"
	IsUndefined           ExprKind = "undefined"
	ToBoolean             ExprKind = "boolean"
	Narrow                ExprKind = "narrow"
	Cast                  ExprKind = "cast"
	NumericConvert        ExprKind = "numeric_convert"
	CheckedNumericConvert ExprKind = "checked_numeric_convert"
	NumericMinMax         ExprKind = "numeric_minmax"
	RuntimeOp             ExprKind = "runtime"
	// ClassOf is the class value of a statically known class (a descriptor
	// reference at run time).
	ClassOf ExprKind = "classof"
	// Seq runs X (a Block) for its effect, then yields Y: the front end's
	// home for statements that must precede an expression (inlined loops).
	Seq ExprKind = "seq"
)

// IntegerRange is a frontend proof of an integral, negative-zero-free value.
// Bounds are confined to JS safe integers even when represented as int8.
type IntegerRange struct{ Min, Max int64 }

type Expr struct {
	// CheckIntegerOverflow requires a JS safe-integer range check after I64 arithmetic.
	// ABAP raises natively; this also enforces the contract in bigint runtimes.
	CheckIntegerOverflow bool
	Node
	Kind            ExprKind
	Type            Type
	Name, Owner, Op string
	Value           any // nil denotes undefined; primitive Optional literals denote present values
	X, Y, Z         *Expr
	Args            []*Expr
	Range           *IntegerRange // NumericConvert narrowing proof
	Stmt            *Stmt         // Seq: the statements evaluated before Y
}
type StmtKind string

const (
	Block    StmtKind = "block"
	VarDecl  StmtKind = "var"
	Assign   StmtKind = "assign"
	ExprStmt StmtKind = "expr"
	If       StmtKind = "if"
	While    StmtKind = "while"
	ForEach  StmtKind = "foreach"
	Break    StmtKind = "break"
	Continue StmtKind = "continue"
	Return   StmtKind = "return"
	// Trap raises a dedicated, uncatchable-by-TS-payload coverage exception.
	Trap  StmtKind = "trap"
	Throw StmtKind = "throw"
	Try   StmtKind = "try"
)

// Try uses Body and Else for the try and catch bodies. Name/Type bind the
// caught payload (not the target exception object). ForEach ranges over Array.
type Stmt struct {
	Node
	Kind       StmtKind
	Name       string
	Type       Type
	X, Y       *Expr
	Body, Else *Stmt
	List       []*Stmt
}

func B(stmts ...*Stmt) *Stmt      { return &Stmt{Kind: Block, List: stmts} }
func L(t Type, v any) *Expr       { return &Expr{Kind: Lit, Type: t, Value: v} }
func V(name string, t Type) *Expr { return &Expr{Kind: Local, Name: name, Type: t} }

// Constructor resolves an inherited constructor after ancestry verification.
func (p *Program) Constructor(name string) *Method {
	for name != "" {
		var found *Class
		for _, c := range p.Classes {
			if c.Name == name {
				found = c
				break
			}
		}
		if found == nil {
			return nil
		}
		if found.Ctor != nil {
			return found.Ctor
		}
		name = found.Super
	}
	return nil
}
