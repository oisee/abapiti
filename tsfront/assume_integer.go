package tsfront

import (
	"fmt"
	"math"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// IntegerException identifies exactly one TS expression by its token offset.
// SHA256 fingerprints the exact source span; Reason is required review evidence.
// Unsupported TS constructs remain blocking even when listed here.
type IntegerException struct {
	File   string `json:"file"`
	Start  int    `json:"start"`
	SHA256 string `json:"sha256"`
	Reason string `json:"reason"`
}

func (l *lowerer) validateIntegerExceptions(files []string) error {
	if !l.integerOptions.AssumeOnlyIntegerCalculations {
		return nil
	}
	entries := map[string]IntegerException{}
	for _, e := range l.integerOptions.IntegerExceptions {
		key := fmt.Sprintf("%s:%d", e.File, e.Start)
		if e.File == "" || e.Start < 0 || len(e.SHA256) != 64 || strings.TrimSpace(e.Reason) == "" {
			return fmt.Errorf("invalid integer exception %s", key)
		}
		if _, ok := entries[key]; ok {
			return fmt.Errorf("duplicate integer exception %s", key)
		}
		entries[key] = e
	}
	seen := map[string]int{}
	selected := map[string]bool{}
	for _, name := range files {
		f, ok := l.prog.File(name)
		if !ok {
			continue
		}
		rel, _ := filepath.Rel(l.prog.configDir, f.FileName())
		rel = filepath.ToSlash(rel)
		selected[rel] = true
		var failure error
		var walk func(*ast.Node)
		walk = func(n *ast.Node) {
			if n == nil || failure != nil {
				return
			}
			start := scanner.GetTokenPosOfNode(n, f, false)
			key := fmt.Sprintf("%s:%d", rel, start)
			if e, ok := entries[key]; ok && integerHazard(n) != "" {
				if overrides.Fingerprint(f.Text()[start:n.End()]) != e.SHA256 {
					failure = fmt.Errorf("integer exception stale at %s", locString(f, start))
					return
				}
				seen[key]++
				l.integerExceptions[n] = e
			}
			n.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
		}
		walk(f.AsNode())
		if failure != nil {
			return failure
		}
	}
	for key, e := range entries {
		if selected[e.File] && seen[key] != 1 {
			return fmt.Errorf("integer exception stale: %s occurs %d times", key, seen[key])
		}
	}
	return nil
}

func integerHazard(n *ast.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind {
	case ast.KindNumericLiteral:
		f := ast.GetSourceFileOfNode(n)
		text := strings.ToLower(f.Text()[scanner.GetTokenPosOfNode(n, f, false):n.End()])
		if !strings.HasPrefix(text, "0x") && !strings.HasPrefix(text, "0b") && !strings.HasPrefix(text, "0o") && strings.ContainsAny(text, ".e") {
			return "fractional or exponent numeric literal"
		}
		text = strings.ReplaceAll(text, "_", "")
		base := 10
		if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "0o") {
			base = 0
		}
		if v, ok := new(big.Int).SetString(text, base); ok && v.Cmp(big.NewInt(safeInteger)) > 0 {
			return "numeric literal outside the JS safe-integer range"
		}
	case ast.KindBinaryExpression:
		b := n.AsBinaryExpression()
		switch b.OperatorToken.Kind {
		case ast.KindSlashToken, ast.KindSlashEqualsToken:
			return "division can produce a fraction"
		case ast.KindAsteriskAsteriskToken, ast.KindAsteriskAsteriskEqualsToken:
			if b.Right.Kind != ast.KindNumericLiteral {
				return "exponent is negative or non-literal"
			}
			v, err := strconv.ParseFloat(b.Right.Text(), 64)
			if err != nil || v < 0 || math.Trunc(v) != v {
				return "exponent is not a nonnegative integer literal"
			}
		}
	case ast.KindCallExpression:
		callee := n.Expression()
		if callee.Kind == ast.KindIdentifier {
			if callee.Text() == "parseFloat" || callee.Text() == "Number" {
				return "numeric conversion may produce a fraction"
			}
		}
		if callee.Kind == ast.KindPropertyAccessExpression {
			a := callee.AsPropertyAccessExpression()
			name := a.Name().Text()
			if a.Expression.Kind == ast.KindIdentifier && a.Expression.Text() == "Math" && name != "min" && name != "max" && name != "abs" && name != "sign" && name != "imul" && name != "clz32" {
				return "Math." + name + " requires integral-result evidence"
			}
			if name == "toFixed" || name == "toPrecision" || name == "toExponential" {
				return "fractional numeric formatting"
			}
		}
	}
	return ""
}

func (l *lowerer) checkIntegerExpression(n *ast.Node) bool {
	if _, listed := l.integerExceptions[n]; listed && l.floatSites[l.locOf(n)] {
		return true
	}
	if l.integerNumberIdentity(n) != nil {
		return true
	}
	why := integerHazard(n)
	if why == "" {
		return true
	}
	start := scanner.GetTokenPosOfNode(n, l.file, false)
	span := l.file.Text()[start:n.End()]
	if e, ok := l.integerExceptions[n]; ok {
		l.floatSites[l.locOf(n)] = true
		l.diagf(n, "note-integer-exception", "%s: %s; %s; sha256=%s", span, why, e.Reason, e.SHA256)
		return true
	}
	l.diagf(n, "assume-integer", "%s: %s; requires a fingerprinted integer exception (sha256=%s)", span, why, overrides.Fingerprint(span))
	return false
}

// The existing range pass runs unchanged first. Only its remaining Number
// storage defaults to I64. Runtime signatures and approved floating sites
// retain explicit conversions; a floating result must pass an exact check.
func integerType(t hir.Type) hir.Type {
	if t.Kind == hir.Number {
		t.Kind = hir.I64
	}
	args := make([]hir.Type, len(t.Args))
	for i, a := range t.Args {
		args[i] = integerType(a)
	}
	t.Args = args
	return t
}

func checkedInteger(e *hir.Expr, t hir.Type) *hir.Expr {
	if e == nil || e.Type.Equal(t) {
		return e
	}
	if e.Type.Kind == hir.I32 && t.Kind == hir.I64 {
		return &hir.Expr{Node: e.Node, Kind: hir.NumericConvert, Type: t, X: e}
	}
	lo, hi := int64(-safeInteger), int64(safeInteger)
	if t.Kind == hir.I32 {
		lo, hi = math.MinInt32, math.MaxInt32
	}
	return &hir.Expr{Node: e.Node, Kind: hir.CheckedNumericConvert, Type: t, X: e, Range: &hir.IntegerRange{Min: lo, Max: hi}}
}

func integerFloat(e *hir.Expr) *hir.Expr {
	// An approved floating consumer extends the floating region. Check the
	// result when it crosses back into integer storage, rather than rounding or
	// rejecting an intermediate fraction that the consumer may cancel.
	if floatingIntegerBoundary(e) {
		return e.X
	}
	if e == nil || e.Type.Kind == hir.Number {
		return e
	}
	// Binary64 boundaries accept only the safe integer domain, so conversion is exact.
	checked := &hir.Expr{Node: e.Node, Kind: hir.CheckedNumericConvert, Type: hir.T(hir.I64), X: e, Range: &hir.IntegerRange{Min: -safeInteger, Max: safeInteger}}
	return &hir.Expr{Node: e.Node, Kind: hir.NumericConvert, Type: hir.T(hir.Number), X: checked}
}

func (l *lowerer) assumeIntegerTypes() {
	var expr func(*hir.Expr) *hir.Expr
	var stmt func(*hir.Stmt)
	expr = func(e *hir.Expr) *hir.Expr {
		if e == nil {
			return nil
		}
		original := e.Type
		e.X, e.Y, e.Z = expr(e.X), expr(e.Y), expr(e.Z)
		for i := range e.Args {
			e.Args[i] = expr(e.Args[i])
		}
		stmt(e.Stmt)
		e.Type = integerType(e.Type)
		e.CheckIntegerOverflow = e.Type.Kind == hir.I64 && (e.Kind == hir.Binary && (e.Op == "+" || e.Op == "-" || e.Op == "*") || e.Kind == hir.Unary && e.Op == "-")
		if e.Kind == hir.NumericConvert || e.Kind == hir.CheckedNumericConvert {
			if e.X.Type.Equal(e.Type) {
				return e.X
			}
			if e.X.Type.Kind == hir.Number {
				return checkedInteger(e.X, e.Type)
			}
		}
		if e.Kind == hir.Binary && e.Op == "/" {
			if !l.floatSites[e.Source] {
				l.diags = append(l.diags, LowerDiagnostic{Category: "assume-integer", Loc: e.Source, Message: "HIR division requires a fingerprinted TS exception"})
			}
			e.Type = hir.T(hir.Number)
			e.X, e.Y = integerFloat(e.X), integerFloat(e.Y)
			// The backend's literal-divisor contract remains intact.
			if e.Y.Kind == hir.NumericConvert && e.Y.X.X.Kind == hir.Lit {
				e.Y = e.Y.X.X
				e.Y.Type = hir.T(hir.Number)
			}
			return checkedInteger(e, integerType(original))
		}
		if e.Kind == hir.Binary && numberType(e.X.Type) && numberType(e.Y.Type) && (floatingIntegerBoundary(e.X) || floatingIntegerBoundary(e.Y)) {
			e.X, e.Y = integerFloat(e.X), integerFloat(e.Y)
			e.CheckIntegerOverflow = false
			if numberType(original) {
				e.Type = hir.T(hir.Number)
				return checkedInteger(e, integerType(original))
			}
		}
		if e.Kind == hir.NumericMinMax && (floatingIntegerBoundary(e.X) || floatingIntegerBoundary(e.Y)) {
			e.X, e.Y = integerFloat(e.X), integerFloat(e.Y)
			e.Type = hir.T(hir.Number)
			return checkedInteger(e, integerType(original))
		}
		if e.Kind == hir.Unary && e.Op == "-" && floatingIntegerBoundary(e.X) {
			e.X = integerFloat(e.X)
			e.Type = hir.T(hir.Number)
			e.CheckIntegerOverflow = false
			return checkedInteger(e, integerType(original))
		}
		if e.Kind == hir.Lit && numberType(original) && l.floatSites[e.Source] {
			e.Type = hir.T(hir.Number)
			return checkedInteger(e, integerType(original))
		}
		if e.Kind == hir.Lit && original.Kind == hir.Number {
			v, err := strconv.ParseFloat(fmt.Sprint(e.Value), 64)
			if err != nil || math.Trunc(v) != v || v < -float64(safeInteger) || v > float64(safeInteger) {
				if !l.floatSites[e.Source] {
					l.diags = append(l.diags, LowerDiagnostic{Category: "assume-integer", Loc: e.Source, Message: "non-integral or unsafe HIR literal requires a fingerprinted TS exception"})
				}
				e.Type = hir.T(hir.Number)
				return checkedInteger(e, hir.T(hir.I64))
			}
			e.Value = int64(v)
		}
		if e.Kind == hir.RuntimeOp {
			switch e.Op {
			case "number.fromI32":
				return checkedInteger(e.X, e.Type)
			case "number.index":
				return checkedInteger(e.X, hir.T(hir.I32))
			case "number.toString":
				if e.X.Type.Kind == hir.I32 {
					e.Op = "i32.toString"
				} else {
					e.Op = "i64.toString"
				}
			case "string.parseInt10":
				e.Op = "string.parseInt10i64"
			case "number.remainder2":
				if floatingIntegerBoundary(e.X) {
					e.X = integerFloat(e.X)
				} else {
					e.Op = "i64.remainder2"
					e.X = checkedInteger(e.X, hir.T(hir.I64))
				}
			}
			if ps, result, ok := hir.RuntimeSignature(e.Op, e.X.Type); ok {
				for i := range e.Args {
					if i < len(ps) && numberType(ps[i]) && numberType(e.Args[i].Type) {
						e.Args[i] = checkedInteger(e.Args[i], ps[i])
					}
				}
				e.Type = result
				if original.Kind == hir.Number && result.Kind == hir.Number {
					return checkedInteger(e, hir.T(hir.I64))
				}
			}
		}
		return e
	}
	stmt = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		s.Type = integerType(s.Type)
		s.X, s.Y = expr(s.X), expr(s.Y)
		stmt(s.Body)
		stmt(s.Else)
		for _, child := range s.List {
			stmt(child)
		}
	}
	for _, c := range l.out.Classes {
		for i := range c.Fields {
			c.Fields[i].Type = integerType(c.Fields[i].Type)
		}
		for _, m := range numberMethods(c) {
			m.Result = integerType(m.Result)
			for i := range m.Params {
				m.Params[i].Type = integerType(m.Params[i].Type)
			}
			stmt(m.Body)
		}
	}
	for _, c := range l.out.Interfaces {
		for _, m := range c.Methods {
			m.Result = integerType(m.Result)
			for i := range m.Params {
				m.Params[i].Type = integerType(m.Params[i].Type)
			}
		}
	}
}

func (l *lowerer) integerNumberIdentity(n *ast.Node) *ast.Node {
	if n == nil || n.Kind != ast.KindCallExpression || n.Expression().Kind != ast.KindIdentifier || n.Expression().Text() != "Number" {
		return nil
	}
	file := l.fileOfSymbol(l.resolve(n.Expression()))
	if file == nil || !l.prog.prog.IsSourceFileDefaultLibrary(file.Path()) {
		return nil
	}
	args := n.Arguments()
	if len(args) == 1 && l.ck.GetTypeAtLocation(args[0]).Flags()&checker.TypeFlagsNumberLike != 0 {
		return args[0]
	}
	return nil
}

func floatingIntegerBoundary(e *hir.Expr) bool {
	return e != nil && e.Kind == hir.CheckedNumericConvert && e.X != nil && e.X.Type.Kind == hir.Number
}
