package tsfront

// This pass belongs to TS lowering, not the ABAP backend. The abstract domain
// distinguishes unreachable/bottom, finite safe integers, and binary64/top.
// Safe-integer bounds also prevent int8 arithmetic from changing JS rounding.
import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

const safeInteger int64 = 9007199254740991
const lengthLimit int64 = math.MaxInt32

type numberInterval struct {
	state  uint8 // 0 bottom, 1 integer interval, 2 unknown binary64
	lo, hi int64
	cap    string // additionally <= immutable length identified by cap + delta
	delta  int64
}

var numberTop = numberInterval{state: 2}

func integerRange(lo, hi int64) numberInterval {
	if lo > hi {
		return numberInterval{}
	}
	if lo < -safeInteger || hi > safeInteger {
		return numberTop
	}
	return numberInterval{state: 1, lo: lo, hi: hi}
}
func (r numberInterval) kind() hir.Kind {
	if r.state != 1 {
		return hir.Number
	}
	if r.lo >= math.MinInt32 && r.hi <= math.MaxInt32 {
		return hir.I32
	}
	return hir.I64
}
func joinNumber(a, b numberInterval) numberInterval {
	if a.state == 0 {
		return b
	}
	if b.state == 0 {
		return a
	}
	if a.state == 2 || b.state == 2 {
		return numberTop
	}
	r := integerRange(min(a.lo, b.lo), max(a.hi, b.hi))
	if a.cap == "" && a.hi <= min(int64(0), b.delta) {
		a.cap = b.cap
		a.delta = b.delta
	}
	if b.cap == "" && b.hi <= min(int64(0), a.delta) {
		b.cap = a.cap
		b.delta = a.delta
	}
	if a.cap != "" && a.cap == b.cap {
		r.cap = a.cap
		r.delta = max(a.delta, b.delta)
	}
	return r
}
func widenNumber(a, b numberInterval) numberInterval {
	r := joinNumber(a, b)
	if a.state != 1 || r.state != 1 {
		return r
	}
	bounds := []int64{-safeInteger, math.MinInt32, -1, 0, math.MaxInt32, safeInteger}
	if r.lo < a.lo {
		r.lo = -safeInteger
		for _, v := range bounds {
			if v <= b.lo {
				r.lo = v
			}
		}
	}
	if r.hi > a.hi {
		r.hi = safeInteger
		for _, v := range bounds {
			if v >= b.hi {
				r.hi = v
				break
			}
		}
	}
	return r
}
func numericLiteral(e *hir.Expr) numberInterval {
	if e == nil || e.Kind != hir.Lit || !numberType(e.Type) {
		return numberTop
	}
	f, err := strconv.ParseFloat(fmt.Sprint(e.Value), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || math.Trunc(f) != f || math.Abs(f) > float64(safeInteger) || f == 0 && math.Signbit(f) {
		return numberTop
	}
	return integerRange(int64(f), int64(f))
}
func numberType(t hir.Type) bool {
	return t.Kind == hir.Number || t.Kind == hir.I32 || t.Kind == hir.I64
}
func unwrapNumber(e *hir.Expr) *hir.Expr {
	for e != nil && e.Kind == hir.RuntimeOp && e.Op == "number.fromI32" {
		e = e.X
	}
	return e
}
func rangeKey(e *hir.Expr) string {
	e = unwrapNumber(e)
	if e == nil {
		return ""
	}
	switch e.Kind {
	case hir.Local:
		return e.Name
	case hir.FieldGet:
		if e.X != nil && e.X.Kind == hir.This {
			return "@" + e.Name
		}
	case hir.StaticGet:
		return "$" + e.Owner + "." + e.Name
	}
	return ""
}
func lengthKey(e *hir.Expr) string {
	e = unwrapNumber(e)
	if e != nil && e.Kind == hir.RuntimeOp && e.Op == "string.length" && e.X != nil && e.X.Kind == hir.FieldGet && e.X.X != nil && e.X.X.Kind == hir.This {
		return e.X.Name
	}
	return ""
}

type numberMethod struct {
	class  *hir.Class
	method *hir.Method
	params []numberInterval
	result numberInterval
	locals map[*hir.Stmt]numberInterval
	exprs  map[*hir.Expr]numberInterval
}
type numberPass struct {
	prog      *hir.Program
	methods   map[*hir.Method]*numberMethod
	fields    map[string]numberInterval
	fixed     map[string]numberInterval
	immutable map[string]bool
	writes    map[string]numberInterval
	incoming  map[*hir.Method][]numberInterval
	results   map[*hir.Method]numberInterval
	current   *numberMethod
	exprs     map[*hir.Expr]numberInterval
	locals    map[*hir.Stmt]numberInterval
	result    numberInterval
	loopFlows []*[]map[string]numberInterval
}

func numberMethods(c *hir.Class) []*hir.Method {
	r := append([]*hir.Method{}, c.Methods...)
	if c.Ctor != nil {
		r = append(r, c.Ctor)
	}
	return r
}
func (p *numberPass) field(e *hir.Expr) string {
	if e == nil {
		return ""
	}
	owner := e.Owner
	if e.Kind == hir.FieldGet && e.X != nil {
		owner = e.X.Type.Name
	} else if e.Kind != hir.StaticGet {
		return ""
	}
	// An inherited receiver writes the declaring class's storage, so its write
	// must participate in the same summary used by reads through the base type.
	for name := owner; name != ""; {
		found := false
		for _, c := range p.prog.Classes {
			if c.Name != name {
				continue
			}
			for _, f := range c.Fields {
				if f.Name == e.Name {
					return c.Name + "." + e.Name
				}
			}
			name = c.Super
			found = true
			break
		}
		if !found {
			break
		}
	}
	return owner + "." + e.Name
}

func (p *numberPass) declaringField(owner, name string) string {
	for current := owner; current != ""; {
		found := false
		for _, class := range p.prog.Classes {
			if class.Name != current {
				continue
			}
			for _, field := range class.Fields {
				if field.Name == name {
					return class.Name + "." + name
				}
			}
			current = class.Super
			found = true
			break
		}
		if !found {
			break
		}
	}
	return owner + "." + name
}

func (p *numberPass) tracksField(key string) bool {
	owner, field, found := strings.Cut(key, ".")
	if !found {
		return false
	}
	for _, class := range p.prog.Classes {
		if class.Name != owner {
			continue
		}
		for _, candidate := range class.Fields {
			if candidate.Name == field && candidate.Type.Kind == hir.Number && (candidate.Private || candidate.Readonly) {
				return true
			}
		}
		return false
	}
	return false
}

func (p *numberPass) resolve(e *hir.Expr) *hir.Method {
	owner := e.Owner
	if e.Kind == hir.New {
		return p.prog.Constructor(e.Type.Name)
	}
	if owner == "" && e.X != nil {
		owner = e.X.Type.Name
	}
	if e.Kind == hir.SuperCall {
		owner = p.current.class.Super
	}
	for _, iface := range p.prog.Interfaces {
		if iface.Name == owner {
			for _, m := range iface.Methods {
				if m.Name == e.Name {
					return m
				}
			}
		}
	}
	for _, c := range p.prog.Classes {
		if c.Name == owner {
			for _, m := range numberMethods(c) {
				if m.Name == e.Name {
					return m
				}
			}
			if c.Super != "" {
				clone := *e
				clone.Owner = c.Super
				clone.Kind = hir.DirectCall
				return p.resolve(&clone)
			}
		}
	}
	return nil
}
func cloneNumbers(e map[string]numberInterval) map[string]numberInterval {
	r := map[string]numberInterval{}
	for k, v := range e {
		r[k] = v
	}
	return r
}
func joinNumbers(a, b map[string]numberInterval) map[string]numberInterval {
	r := cloneNumbers(a)
	for k, v := range b {
		r[k] = joinNumber(r[k], v)
	}
	// A field fact known on one branch only is not a fact: drop it so reads
	// fall back to the field summary (unknown unless tracked).
	for k := range r {
		if len(k) > 0 && (k[0] == '@' || k[0] == '$') {
			_, inA := a[k]
			_, inB := b[k]
			if inA != inB {
				delete(r, k)
			}
		}
	}
	return r
}
func sameNumbers(a, b map[string]numberInterval) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}
func (p *numberPass) invalidateFields(env map[string]numberInterval) {
	for k := range env {
		if len(k) > 0 && (k[0] == '@' || k[0] == '$') {
			if p.current.method.Name == "constructor" || p.current.method.Name == "class_constructor" {
				if env[k].state == 2 {
					continue
				}
			}
			if k[0] == '@' {
				owner, name := p.current.class.Name, k[1:]
				field := p.declaringField(owner, name)
				env[k] = p.fieldSummary(field)
			} else {
				env[k] = p.fieldSummary(k[1:])
			}
		}
	}
}

func (p *numberPass) fieldSummary(key string) numberInterval {
	if value, ok := p.fields[key]; ok {
		return value
	}
	if p.tracksField(key) {
		return missingNumberFact(key)
	}
	return numberTop
}

func missingNumberFact(key string) numberInterval {
	if numberAssertMissingFacts {
		panic("tsfront: missing number fact is not bottom: " + key)
	}
	return numberTop
}
func (p *numberPass) expr(e *hir.Expr, env map[string]numberInterval) numberInterval {
	if e == nil {
		return numberTop
	}
	r := numberTop
	switch e.Kind {
	case hir.CheckedNumericConvert:
		p.expr(e.X, env)
		r = integerRange(e.Range.Min, e.Range.Max)
	case hir.Lit:
		r = numericLiteral(e)
	case hir.Local:
		if v, ok := env[e.Name]; ok {
			r = v
		}
	case hir.FieldGet, hir.StaticGet:
		p.expr(e.X, env)
		key := rangeKey(e)
		if v, ok := env[key]; ok {
			r = v
		} else if v, ok := p.fields[p.field(e)]; ok {
			r = v
		}
		if e.Kind == hir.FieldGet && (e.X == nil || e.X.Kind != hir.This) {
			r.cap = ""
		}
	case hir.NumericMinMax:
		a, b := p.expr(e.X, env), p.expr(e.Y, env)
		if a.state == 0 || b.state == 0 {
			r = numberInterval{}
		} else if a.state == 1 && b.state == 1 {
			if e.Op == "min" {
				r = integerRange(min(a.lo, b.lo), min(a.hi, b.hi))
			} else {
				r = integerRange(max(a.lo, b.lo), max(a.hi, b.hi))
			}
		}
	case hir.Binary:
		a := p.expr(e.X, env)
		if e.Op == "&&" || e.Op == "||" {
			rhs := cloneNumbers(env)
			p.guard(e.X, rhs, e.Op == "&&")
			p.expr(e.Y, rhs)
			merged := joinNumbers(env, rhs)
			for k, v := range merged {
				env[k] = v
			}
		} else {
			b := p.expr(e.Y, env)
			r = numberArithmetic(e.Op, a, b)
		}
	case hir.Unary:
		a := p.expr(e.X, env)
		if e.Op == "-" && a.state == 1 && (a.lo > 0 || a.hi < 0) {
			r = integerRange(-a.hi, -a.lo)
		}
	case hir.Conditional:
		p.expr(e.X, env)
		yes, no := cloneNumbers(env), cloneNumbers(env)
		p.guard(e.X, yes, true)
		p.guard(e.X, no, false)
		r = joinNumber(p.expr(e.Y, yes), p.expr(e.Z, no))
		merge := joinNumbers(yes, no)
		for k, v := range merge {
			env[k] = v
		}
	case hir.Seq:
		next := cloneNumbers(env)
		if e.Stmt != nil {
			for _, s := range e.Stmt.List {
				next, _ = p.stmt(s, next)
			}
		}
		r = p.expr(e.Y, next)
		// Seq blocks can assign outer locals but their declarations do not escape.
		for k := range env {
			if v, ok := next[k]; ok {
				env[k] = v
			}
		}
	case hir.RuntimeOp:
		a := p.expr(e.X, env)
		for _, arg := range e.Args {
			p.expr(arg, env)
		}
		switch e.Op {
		case "number.fromI32":
			r = a
		case "number.index":
			r = integerRange(math.MinInt32, math.MaxInt32)
		case "string.length", "array.length", "map.size", "set.size", "array.push":
			r = integerRange(0, lengthLimit)
			r.cap = lengthKey(e)
			if !p.immutable[p.current.class.Name+"."+r.cap] {
				r.cap = ""
			}
		case "string.indexOf", "array.indexOf":
			r = integerRange(-1, lengthLimit-1)
		case "string.charCodeAt":
			r = integerRange(0, 65535) // existing runtime traps out of bounds
		case "number.remainder2":
			r = numberArithmetic("%", a, integerRange(2, 2))
		}
		if spec, ok := hir.RuntimeSpecs[e.Op]; ok && spec.Mutates {
			p.invalidateFields(env)
		}
	case hir.DirectCall, hir.VirtualCall, hir.SuperCall, hir.New:
		p.expr(e.X, env)
		args := make([]numberInterval, len(e.Args))
		for i, a := range e.Args {
			args[i] = p.expr(a, env)
		}
		m := p.resolve(e)
		if info := p.methods[m]; info != nil {
			ps := p.incoming[m]
			for i, v := range args {
				if i < len(ps) {
					ps[i] = joinNumber(ps[i], v)
				}
			}
			r = info.result
			// Join every possible lowered override: interface/virtual dispatch cannot
			// acquire a range from only one implementation.
			if e.Kind == hir.VirtualCall {
				for _, other := range p.methods {
					if other.method.Name == e.Name && p.descends(other.class.Name, info.class.Name) {
						r = joinNumber(r, other.result)
					}
				}
			}
		}
		p.invalidateFields(env)
	default:
		p.expr(e.X, env)
		p.expr(e.Y, env)
		p.expr(e.Z, env)
		for _, a := range e.Args {
			p.expr(a, env)
		}
	}
	if numberType(e.Type) {
		p.exprs[e] = joinNumber(p.exprs[e], r)
	}
	return r
}
func numberArithmetic(op string, a, b numberInterval) numberInterval {
	if a.state == 0 || b.state == 0 {
		return numberInterval{}
	}
	if a.state != 1 || b.state != 1 {
		return numberTop
	}
	var r numberInterval
	switch op {
	case "+":
		r = integerRange(a.lo+b.lo, a.hi+b.hi)
		if b.lo == b.hi {
			r.cap = a.cap
			r.delta = a.delta + b.lo
		} else if a.lo == a.hi {
			r.cap = b.cap
			r.delta = b.delta + a.lo
		}
	case "-":
		r = integerRange(a.lo-b.hi, a.hi-b.lo)
		if b.lo == b.hi {
			r.cap = a.cap
			r.delta = a.delta - b.lo
		}
	case "*":
		// Zero multiplied by a negative operand produces observable negative zero.
		if (a.lo <= 0 && a.hi >= 0 && b.lo < 0) || (b.lo <= 0 && b.hi >= 0 && a.lo < 0) {
			return numberTop
		}
		lo, hi := float64(a.lo)*float64(b.lo), float64(a.lo)*float64(b.lo)
		for _, v := range []float64{float64(a.lo) * float64(b.hi), float64(a.hi) * float64(b.lo), float64(a.hi) * float64(b.hi)} {
			lo = math.Min(lo, v)
			hi = math.Max(hi, v)
		}
		if lo < -float64(safeInteger) || hi > float64(safeInteger) {
			return numberTop
		}
		r = integerRange(int64(lo), int64(hi))
	case "%":
		if b.lo == b.hi && b.lo > 0 && a.lo >= 0 {
			r = integerRange(0, min(a.hi, b.lo-1))
		} else {
			return numberTop
		}
	default:
		return numberTop
	}
	if r.state != 1 {
		return numberTop
	}
	return r
}
func (p *numberPass) descends(name, base string) bool {
	for name != "" {
		if name == base {
			return true
		}
		found := false
		for _, c := range p.prog.Classes {
			if c.Name == name {
				name = c.Super
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return false
}
func numberFieldEffects(e *hir.Expr) bool {
	changed := false
	walkNumberStmt(&hir.Stmt{Kind: hir.ExprStmt, X: e}, func(s *hir.Stmt) {
		if s.Kind == hir.Assign && s.X != nil && (s.X.Kind == hir.FieldGet || s.X.Kind == hir.StaticGet) {
			changed = true
		}
	}, func(x *hir.Expr) {
		switch x.Kind {
		case hir.DirectCall, hir.VirtualCall, hir.SuperCall, hir.New:
			changed = true
		case hir.RuntimeOp:
			if spec, ok := hir.RuntimeSpecs[x.Op]; ok && spec.Mutates {
				changed = true
			}
		}
	})
	return changed
}

func (p *numberPass) guard(e *hir.Expr, env map[string]numberInterval, truth bool) {
	if e == nil {
		return
	}
	if e.Kind == hir.Unary && e.Op == "!" {
		p.guard(e.X, env, !truth)
		return
	}
	if e.Kind != hir.Binary {
		return
	}
	if e.Op == "&&" && truth || e.Op == "||" && !truth {
		if numberFieldEffects(e.Y) {
			p.guard(e.Y, env, truth)
			return
		}
		p.guard(e.X, env, truth)
		p.guard(e.Y, env, truth)
		return
	}
	key := rangeKey(e.X)
	if key == "" {
		return
	}
	// The comparison sampled its left operand before evaluating the right. A
	// call on the right may have changed the field by the time the branch runs.
	if (key[0] == '@' || key[0] == '$') && numberFieldEffects(e.Y) {
		return
	}
	a := p.peek(e.X, env)
	b := p.peek(e.Y, env)
	if a.state != 1 || b.state != 1 {
		return
	}
	op := e.Op
	if !truth {
		op = map[string]string{"<": ">=", "<=": ">", ">": "<=", ">=": "<", "==": "!=", "!=": "=="}[op]
	}
	switch op {
	case "<":
		a.hi = min(a.hi, b.hi-1)
	case "<=":
		a.hi = min(a.hi, b.hi)
	case ">":
		a.lo = max(a.lo, b.lo+1)
	case ">=":
		a.lo = max(a.lo, b.lo)
	case "==":
		a.lo = max(a.lo, b.lo)
		a.hi = min(a.hi, b.hi)
	case "!=":
		if b.lo == b.hi {
			if a.lo == b.lo {
				a.lo++
			}
			if a.hi == b.hi {
				a.hi--
			}
		}
		if cap := lengthKey(e.Y); cap != "" && a.cap == cap && a.delta == 0 {
			a.delta = -1
			a.hi = min(a.hi, lengthLimit-1)
		}
	}
	if cap := lengthKey(e.Y); cap != "" && (op == "<" || op == "<=") {
		a.cap = cap
		a.delta = 0
		if op == "<" {
			a.delta = -1
		}
	}
	// Fractional public params stay top even under numeric comparisons.
	if a.lo > a.hi {
		a = numberInterval{}
	}
	env[key] = a
}

// Peeking is pure: guards must not replay calls or their side effects.
func (p *numberPass) peek(e *hir.Expr, env map[string]numberInterval) numberInterval {
	e = unwrapNumber(e)
	if e == nil {
		return numberTop
	}
	if key := rangeKey(e); key != "" {
		if r, ok := env[key]; ok {
			return r
		}
		if r, ok := p.fields[p.field(e)]; ok {
			return r
		}
	}
	if r, ok := p.exprs[e]; ok {
		return r
	}
	if e.Kind == hir.Lit {
		return numericLiteral(e)
	}
	if key := lengthKey(e); key != "" {
		r := integerRange(0, lengthLimit)
		if p.immutable[p.current.class.Name+"."+key] {
			r.cap = key
		}
		return r
	}
	return numberTop
}
func (p *numberPass) recordWrite(e *hir.Expr, r numberInterval, env map[string]numberInterval) {
	// An indirect receiver may alias this (including through a parameter).
	// Drop local field refinements; whole-program summaries include the write.
	if e != nil && e.Kind == hir.FieldGet && (e.X == nil || e.X.Kind != hir.This) {
		p.invalidateFields(env)
	}
	key := rangeKey(e)
	if key != "" {
		env[key] = r
		if e.Kind == hir.Local && p.current.method.Internal {
			for i, param := range p.current.method.Params {
				if param.Name == e.Name {
					p.incoming[p.current.method][i] = joinNumber(p.incoming[p.current.method][i], r)
				}
			}
		}
	}
	if field := p.field(e); field != "" {
		if _, ok := p.fields[field]; ok {
			p.writes[field] = joinNumber(p.writes[field], r)
		}
	}
}
func (p *numberPass) stmt(s *hir.Stmt, env map[string]numberInterval) (map[string]numberInterval, bool) {
	if s == nil {
		return env, true
	}
	switch s.Kind {
	case hir.Block:
		shadow := map[string]numberInterval{}
		absent := map[string]bool{}
		for _, child := range s.List {
			if child != nil && child.Kind == hir.VarDecl {
				if old, ok := env[child.Name]; ok {
					shadow[child.Name] = old
				} else {
					absent[child.Name] = true
				}
			}
		}
		defer func() {
			for k, v := range shadow {
				env[k] = v
			}
			for k := range absent {
				delete(env, k)
			}
		}()
		for _, child := range s.List {
			var live bool
			env, live = p.stmt(child, env)
			if !live {
				return env, false
			}
		}
	case hir.VarDecl:
		r := numberTop
		if s.X != nil {
			r = p.expr(s.X, env)
		}
		env[s.Name] = r
		if numberType(s.Type) {
			p.locals[s] = joinNumber(p.locals[s], r)
		}
	case hir.Assign:
		p.expr(s.X, env)
		r := p.expr(s.Y, env)
		p.recordWrite(s.X, r, env)
		if s.X != nil && s.X.Kind == hir.Local {
			for decl := range p.locals {
				if decl.Name == s.X.Name {
					p.locals[decl] = joinNumber(p.locals[decl], r)
				}
			}
		}
	case hir.ExprStmt:
		p.expr(s.X, env)
	case hir.If:
		p.expr(s.X, env)
		yes, no := cloneNumbers(env), cloneNumbers(env)
		p.guard(s.X, yes, true)
		p.guard(s.X, no, false)
		a, alive := p.stmt(s.Body, yes)
		b, blive := p.stmt(s.Else, no)
		if !alive {
			return b, blive
		}
		if !blive {
			return a, alive
		}
		return joinNumbers(a, b), true
	case hir.While:
		flows := []map[string]numberInterval{}
		p.loopFlows = append(p.loopFlows, &flows)
		defer func() { p.loopFlows = p.loopFlows[:len(p.loopFlows)-1] }()
		entry := cloneNumbers(env)
		head := cloneNumbers(env)
		for iteration := 0; iteration < 12; iteration++ {
			body := cloneNumbers(head)
			p.expr(s.X, body)
			p.guard(s.X, body, true)
			end, _ := p.stmt(s.Body, body)
			for _, flow := range flows {
				end = joinNumbers(end, flow)
			}
			next := joinNumbers(entry, end)
			if iteration >= 2 {
				for k, v := range next {
					next[k] = widenNumber(head[k], v)
				}
			}
			if sameNumbers(head, next) {
				head = next
				break
			}
			head = next
			if iteration == 11 {
				for k := range head {
					head[k] = numberTop
				}
			}
		}
		// Re-analyse at the fixed point so every site records the widened interval.
		body := cloneNumbers(head)
		p.expr(s.X, body)
		p.guard(s.X, body, true)
		end, _ := p.stmt(s.Body, body)
		for _, flow := range flows {
			end = joinNumbers(end, flow)
		}
		// Break/continue conservatively contribute every body state to the exit.
		env = joinNumbers(head, end)
	case hir.ForEach:
		flows := []map[string]numberInterval{}
		p.loopFlows = append(p.loopFlows, &flows)
		defer func() { p.loopFlows = p.loopFlows[:len(p.loopFlows)-1] }()
		p.expr(s.X, env)
		entry := cloneNumbers(env)
		head := cloneNumbers(env)
		for i := 0; i < 12; i++ {
			body := cloneNumbers(head)
			body[s.Name] = numberTop
			end, _ := p.stmt(s.Body, body)
			for _, flow := range flows {
				end = joinNumbers(end, flow)
			}
			next := joinNumbers(entry, end)
			for k, v := range next {
				next[k] = widenNumber(head[k], v)
			}
			if sameNumbers(head, next) {
				break
			}
			head = next
			if i == 11 {
				for k := range head {
					head[k] = numberTop
				}
			}
		}
		body := cloneNumbers(head)
		body[s.Name] = numberTop
		p.stmt(s.Body, body)
		env = head
		for _, flow := range flows {
			env = joinNumbers(env, flow)
		}
	case hir.Break, hir.Continue:
		if len(p.loopFlows) > 0 {
			flows := p.loopFlows[len(p.loopFlows)-1]
			*flows = append(*flows, cloneNumbers(env))
		}
		return env, false
	case hir.Return:
		r := p.expr(s.X, env)
		p.result = joinNumber(p.result, r)
		return env, false
	case hir.Throw:
		p.expr(s.X, env)
		return env, false
	case hir.Try:
		a, al := p.stmt(s.Body, cloneNumbers(env))
		catch := joinNumbers(env, a)
		for k := range catch {
			catch[k] = numberTop
		}
		b, bl := p.stmt(s.Else, catch)
		return joinNumbers(a, b), al || bl
	}
	return env, true
}
func (p *numberPass) analyze(info *numberMethod) {
	p.current = info
	p.exprs = map[*hir.Expr]numberInterval{}
	p.locals = map[*hir.Stmt]numberInterval{}
	p.result = numberInterval{}
	env := map[string]numberInterval{}
	for _, f := range info.class.Fields {
		if f.Type.Kind == hir.Number {
			r, ok := p.fields[info.class.Name+"."+f.Name]
			if !ok || info.method.Name == "constructor" || info.method.Name == "class_constructor" {
				r = numberTop
			}
			key := "@" + f.Name
			if f.Static {
				key = "$" + info.class.Name + "." + f.Name
			}
			env[key] = r
		}
	}
	for i, param := range info.method.Params {
		env[param.Name] = info.params[i]
	}
	p.stmt(info.method.Body, env)
	info.locals = p.locals
	info.exprs = p.exprs
	if info.method.Abstract || info.method.Body == nil {
		p.result = numberTop
	}
	p.results[info.method] = p.result
}

// guardedFields proposes inductive invariants only for private fields compared
// with an immutable string length. Every assignment is subsequently checked
// under that invariant; failure discards the proposal completely.
func (p *numberPass) guardedFields(c *hir.Class) {
	readonly := map[string]bool{}
	writes := map[string]int{}
	for _, m := range numberMethods(c) {
		walkNumberStmt(m.Body, func(s *hir.Stmt) {
			if s.Kind == hir.Assign && s.X != nil && s.X.Kind == hir.FieldGet && s.X.X != nil && s.X.X.Type.Name == c.Name {
				writes[s.X.Name]++
			}
		}, func(e *hir.Expr) {})
	}
	for _, f := range c.Fields {
		if f.Private && f.Readonly && f.Type.Kind == hir.String && writes[f.Name] == 1 && numberFieldInitialized(c, f) {
			readonly[f.Name] = true
			p.immutable[c.Name+"."+f.Name] = true
		}
	}
	for _, m := range numberMethods(c) {
		walkNumberStmt(m.Body, func(s *hir.Stmt) {}, func(e *hir.Expr) {
			if e.Kind != hir.Binary || e.Op != "==" {
				return
			}
			key := rangeKey(e.X)
			cap := lengthKey(e.Y)
			if len(key) < 2 || key[0] != '@' || !readonly[cap] {
				return
			}
			for _, f := range c.Fields {
				if f.Name == key[1:] && f.Private && f.Type.Kind == hir.Number && numberFieldInitialized(c, f) {
					r := integerRange(-1, lengthLimit)
					r.cap = cap
					p.fixed[c.Name+"."+f.Name] = r
					p.fields[c.Name+"."+f.Name] = r
				}
			}
		})
	}
}
func walkNumberStmt(s *hir.Stmt, stmt func(*hir.Stmt), expr func(*hir.Expr)) {
	if s == nil {
		return
	}
	stmt(s)
	walkNumberExpr(s.X, stmt, expr)
	walkNumberExpr(s.Y, stmt, expr)
	walkNumberStmt(s.Body, stmt, expr)
	walkNumberStmt(s.Else, stmt, expr)
	for _, c := range s.List {
		walkNumberStmt(c, stmt, expr)
	}
}
func walkNumberExpr(e *hir.Expr, stmt func(*hir.Stmt), expr func(*hir.Expr)) {
	if e == nil {
		return
	}
	expr(e)
	walkNumberExpr(e.X, stmt, expr)
	walkNumberExpr(e.Y, stmt, expr)
	walkNumberExpr(e.Z, stmt, expr)
	for _, a := range e.Args {
		walkNumberExpr(a, stmt, expr)
	}
	walkNumberStmt(e.Stmt, stmt, expr)
}
func (l *lowerer) inferNumberRanges() {
	l.distinguishNumberLocals()
	l.checkIndexFieldBoundaries()
	p := &numberPass{prog: l.out, methods: map[*hir.Method]*numberMethod{}, fields: map[string]numberInterval{}, fixed: map[string]numberInterval{}, immutable: map[string]bool{}}
	for _, c := range l.out.Classes {
		for _, f := range c.Fields {
			if f.Type.Kind == hir.Number && (f.Private || f.Readonly) {
				p.fields[c.Name+"."+f.Name] = numberInterval{}
				if !numberFieldInitialized(c, f) {
					p.fields[c.Name+"."+f.Name] = numberTop
				}
			}
		}
		for _, m := range numberMethods(c) {
			ps := make([]numberInterval, len(m.Params))
			for i := range ps {
				if !m.Internal {
					ps[i] = numberTop
				}
			}
			p.methods[m] = &numberMethod{class: c, method: m, params: ps}
		}
		p.guardedFields(c)
	}
	// Synchronous monotone iteration; interval widening makes recursive call
	// graphs and repeatedly invoked mutable fields converge conservatively.
	for round := 0; round < 24; round++ {
		p.writes = map[string]numberInterval{}
		p.incoming = map[*hir.Method][]numberInterval{}
		p.results = map[*hir.Method]numberInterval{}
		for m := range p.methods {
			p.incoming[m] = make([]numberInterval, len(m.Params))
		}
		for _, c := range l.out.Classes {
			for _, m := range numberMethods(c) {
				p.analyze(p.methods[m])
			}
		}
		changed := false
		for key, old := range p.fields {
			next := joinNumber(old, p.writes[key])
			if round >= 2 {
				next = widenNumber(old, next)
			}
			if fixed, ok := p.fixed[key]; ok {
				w := p.writes[key]
				// A constant <= -1 is below any nonnegative length. Other writes
				// must preserve the symbolic cap, including its delta.
				if w.state == 1 && w.lo >= fixed.lo && w.hi <= fixed.hi && (w.hi <= 0 || w.cap == fixed.cap && w.delta <= 0) {
					next = fixed
				} else {
					delete(p.fixed, key)
					next = numberTop
				}
			}
			if next != old {
				p.fields[key] = next
				changed = true
			}
		}
		for m, info := range p.methods {
			next := joinNumber(info.result, p.results[m])
			if round >= 2 {
				next = widenNumber(info.result, next)
			}
			if next != info.result {
				info.result = next
				changed = true
			}
			if m.Internal {
				for i, old := range info.params {
					next := joinNumber(old, p.incoming[m][i])
					if round >= 2 {
						next = widenNumber(old, next)
					}
					if next != old {
						info.params[i] = next
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
		if round == 23 {
			for k := range p.fields {
				p.fields[k] = numberTop
			}
			for _, info := range p.methods {
				info.result = numberTop
				for i := range info.params {
					info.params[i] = numberTop
				}
			}
		}
	}
	// Final summaries have converged; analyze once more before changing any type.
	p.writes = map[string]numberInterval{}
	p.incoming = map[*hir.Method][]numberInterval{}
	p.results = map[*hir.Method]numberInterval{}
	for m := range p.methods {
		p.incoming[m] = make([]numberInterval, len(m.Params))
	}
	for _, c := range l.out.Classes {
		for _, m := range numberMethods(c) {
			p.analyze(p.methods[m])
		}
	}
	for _, c := range l.out.Classes {
		for i := range c.Fields {
			f := &c.Fields[i]
			if f.Type.Kind == hir.Number && (f.Private || f.Readonly) {
				f.Type = hir.T(p.fields[c.Name+"."+f.Name].kind())
			}
		}
		for _, m := range numberMethods(c) {
			info := p.methods[m]
			if m.Internal {
				if m.Result.Kind == hir.Number {
					m.Result = hir.T(info.result.kind())
				}
				for i := range m.Params {
					if m.Params[i].Type.Kind == hir.Number {
						m.Params[i].Type = hir.T(info.params[i].kind())
					}
				}
			}
		}
	}
	// Retain public result signatures (including virtual/interface slots).
	// Proven return expressions are widened exactly at those boundaries.
	for _, c := range l.out.Classes {
		counts := map[hir.Kind]int{}
		for _, f := range c.Fields {
			if numberType(f.Type) {
				counts[f.Type.Kind]++
			}
		}
		for _, m := range numberMethods(c) {
			info := p.methods[m]
			env := map[string]hir.Type{}
			for _, param := range m.Params {
				env[param.Name] = param.Type
				if numberType(param.Type) {
					counts[param.Type.Kind]++
				}
			}
			p.current = info
			p.rewriteStmt(m.Body, env, info)
			walkNumberStmt(m.Body, func(s *hir.Stmt) {}, func(e *hir.Expr) {
				if !l.integerOptions.AssumeOnlyIntegerCalculations && e.Kind == hir.NumericMinMax && e.Type.Kind == hir.Number {
					l.diags = append(l.diags, LowerDiagnostic{Category: "unsupported-number", Loc: e.Source, Message: "Math.min/max requires proven integers in this phase; exceptional numbers and signed zero need runtime support"})
				}
				if e.Kind == hir.Binary && e.Op == "%" && e.Type.Kind == hir.Number {
					l.diags = append(l.diags, LowerDiagnostic{Category: "unsupported-number", Loc: e.Source, Message: "remainder needs a proven nonnegative integral dividend (negative zero must remain binary64)"})
				}
			})
			for _, r := range info.locals {
				counts[r.kind()]++
			}
			if numberType(m.Result) {
				counts[m.Result.Kind]++
			}
		}
		l.diags = append(l.diags, LowerDiagnostic{Category: "note-number-ranges", Loc: c.Source, Message: fmt.Sprintf("%s: I32=%d I64=%d Number=%d (fields, params, locals, returns)", c.Name, counts[hir.I32], counts[hir.I64], counts[hir.Number])})
	}
}

func rangeConversion(e *hir.Expr, t hir.Type, r numberInterval) *hir.Expr {
	if t.Kind == hir.Optional && len(t.Args) == 1 && numberType(t.Args[0]) {
		t = t.Args[0]
	}
	if e == nil || !numberType(e.Type) || !numberType(t) || e.Type.Equal(t) {
		return e
	}
	if e.Kind == hir.Lit && numericLiteral(e).state == 1 {
		literal := numericLiteral(e)
		if t.Kind == hir.Number || t.Kind == hir.I64 || literal.kind() == hir.I32 {
			copy := *e
			copy.Type = t
			return &copy
		}
	}
	if e.Kind == hir.Seq {
		copy := *e
		copy.Type = t
		copy.Y = rangeConversion(e.Y, t, r)
		return &copy
	}
	proof := &hir.IntegerRange{Min: r.lo, Max: r.hi}
	if r.state != 1 {
		proof = nil
	}
	return &hir.Expr{Node: e.Node, Kind: hir.NumericConvert, Type: t, X: e, Range: proof}
}
func numericJoinType(a, b hir.Type) hir.Type {
	if a.Kind == hir.Number || b.Kind == hir.Number {
		return hir.T(hir.Number)
	}
	if a.Kind == hir.I64 || b.Kind == hir.I64 {
		return hir.T(hir.I64)
	}
	return hir.T(hir.I32)
}
func (p *numberPass) fieldType(e *hir.Expr) hir.Type {
	owner := e.Owner
	if e.Kind == hir.FieldGet && e.X != nil {
		owner = e.X.Type.Name
	}
	for _, c := range p.prog.Classes {
		if c.Name == owner {
			for _, f := range c.Fields {
				if f.Name == e.Name {
					return f.Type
				}
			}
			if c.Super != "" {
				copy := *e
				copy.Owner = c.Super
				copy.Kind = hir.StaticGet
				return p.fieldType(&copy)
			}
		}
	}
	return e.Type
}
func (p *numberPass) rewriteExpr(e *hir.Expr, env map[string]hir.Type, info *numberMethod) *hir.Expr {
	if e == nil {
		return nil
	}
	r := info.exprs[e]
	original := e.Type
	if e.Kind == hir.Seq {
		scope := map[string]hir.Type{}
		for k, v := range env {
			scope[k] = v
		}
		if e.Stmt != nil {
			for _, s := range e.Stmt.List {
				p.rewriteStmt(s, scope, info)
			}
		}
		e.Y = p.rewriteExpr(e.Y, scope, info)
		e.Type = e.Y.Type
		result := e
		if numberType(original) && r.state == 1 {
			result = rangeConversion(e, hir.T(r.kind()), r)
		}
		info.exprs[result] = r
		return result
	}
	e.X = p.rewriteExpr(e.X, env, info)
	e.Y = p.rewriteExpr(e.Y, env, info)
	e.Z = p.rewriteExpr(e.Z, env, info)
	for i := range e.Args {
		e.Args[i] = p.rewriteExpr(e.Args[i], env, info)
	}
	switch e.Kind {
	case hir.Local:
		if t, ok := env[e.Name]; ok {
			e.Type = t
		}
	case hir.FieldGet, hir.StaticGet:
		e.Type = p.fieldType(e)
	case hir.Lit:
		if original.Kind == hir.Number && r.state == 1 {
			e.Type = hir.T(r.kind())
			e.Value = r.lo
		}
	case hir.NumericMinMax:
		e.Type = numericJoinType(hir.T(r.kind()), numericJoinType(e.X.Type, e.Y.Type))
		e.X = rangeConversion(e.X, e.Type, info.exprs[e.X])
		e.Y = rangeConversion(e.Y, e.Type, info.exprs[e.Y])
	case hir.Binary:
		if numberType(original) {
			e.Type = numericJoinType(hir.T(r.kind()), numericJoinType(e.X.Type, e.Y.Type))
			e.X = rangeConversion(e.X, e.Type, info.exprs[e.X])
			e.Y = rangeConversion(e.Y, e.Type, info.exprs[e.Y])
		} else if e.X != nil && e.Y != nil && numberType(e.X.Type) && numberType(e.Y.Type) {
			t := numericJoinType(e.X.Type, e.Y.Type)
			e.X = rangeConversion(e.X, t, info.exprs[e.X])
			e.Y = rangeConversion(e.Y, t, info.exprs[e.Y])
		}
	case hir.Unary:
		if numberType(original) {
			e.Type = hir.T(r.kind())
			e.X = rangeConversion(e.X, e.Type, info.exprs[e.X])
		}
	case hir.Conditional:
		if original.Kind == hir.Optional {
			e.Y = rangeConversion(e.Y, original, info.exprs[e.Y])
			e.Z = rangeConversion(e.Z, original, info.exprs[e.Z])
		}
		if numberType(original) {
			e.Type = hir.T(r.kind())
			e.Y = rangeConversion(e.Y, e.Type, r)
			e.Z = rangeConversion(e.Z, e.Type, r)
		}
	case hir.Seq:
		e.Type = e.Y.Type
	case hir.RuntimeOp:
		if e.Op == "number.fromI32" {
			info.exprs[e.X] = r
			return e.X
		}
		if e.Op == "number.index" && r.state == 1 && e.X.Type.Kind == hir.I32 {
			return e.X
		}
		if e.Op == "number.remainder2" && r.state == 1 {
			e.Kind = hir.Binary
			e.Op = "%"
			e.Type = hir.T(r.kind())
			e.X = rangeConversion(e.X, e.Type, r)
			e.Y = hir.L(e.Type, int64(2))
			return e
		}
		if e.Op == "number.toString" && e.X.Type.Kind == hir.I32 {
			e.Op = "i32.toString"
		}
		// Runtime declarations describe the exact boundary conversions.
		if spec, ok := hir.RuntimeSpecs[e.Op]; ok && spec.Receiver == hir.Number {
			e.X = rangeConversion(e.X, hir.T(hir.Number), r)
		}
		if ps, result, ok := hir.RuntimeSignature(e.Op, e.X.Type); ok {
			e.Type = result
			for i := range e.Args {
				if i < len(ps) {
					e.Args[i] = rangeConversion(e.Args[i], ps[i], info.exprs[e.Args[i]])
				}
			}
		}
	case hir.IndexGet:
		e.Y = rangeConversion(e.Y, hir.T(hir.I32), info.exprs[e.Y])
	case hir.DirectCall, hir.VirtualCall, hir.SuperCall, hir.New:
		if m := p.resolve(e); m != nil {
			if e.Kind != hir.New {
				e.Type = m.Result
			}
			for i := range e.Args {
				if i < len(m.Params) {
					e.Args[i] = rangeConversion(e.Args[i], m.Params[i].Type, info.exprs[e.Args[i]])
				}
			}
		}
	}
	// Flow-proven reads of wider storage keep an explicit proof. This also
	// preserves public binary64 ABIs while eliminating hot index saturation.
	if original.Kind == hir.Number && numberType(e.Type) && r.state == 1 {
		converted := rangeConversion(e, hir.T(r.kind()), r)
		info.exprs[converted] = r
		return converted
	}
	return e
}
func (p *numberPass) rewriteStmt(s *hir.Stmt, env map[string]hir.Type, info *numberMethod) {
	if s == nil {
		return
	}
	switch s.Kind {
	case hir.Block:
		scope := map[string]hir.Type{}
		for k, t := range env {
			scope[k] = t
		}
		for _, child := range s.List {
			p.rewriteStmt(child, scope, info)
		}
	case hir.VarDecl:
		r, ok := info.locals[s]
		if ok && s.Type.Kind == hir.Number {
			s.Type = hir.T(r.kind())
		}
		s.X = p.rewriteExpr(s.X, env, info)
		s.X = rangeConversion(s.X, s.Type, r)
		env[s.Name] = s.Type
	case hir.Assign:
		// Assignment targets must remain lvalues rather than proof conversions.
		switch s.X.Kind {
		case hir.Local:
			if t, ok := env[s.X.Name]; ok {
				s.X.Type = t
			}
		case hir.FieldGet, hir.StaticGet:
			s.X.Type = p.fieldType(s.X)
			s.X.X = p.rewriteExpr(s.X.X, env, info)
		default:
			s.X = p.rewriteExpr(s.X, env, info)
		}
		r := info.exprs[s.Y]
		s.Y = p.rewriteExpr(s.Y, env, info)
		s.Y = rangeConversion(s.Y, s.X.Type, r)
	case hir.ForEach:
		s.X = p.rewriteExpr(s.X, env, info)
		scope := map[string]hir.Type{}
		for k, t := range env {
			scope[k] = t
		}
		scope[s.Name] = s.Type
		p.rewriteStmt(s.Body, scope, info)
	case hir.Return:
		r := info.exprs[s.X]
		s.X = p.rewriteExpr(s.X, env, info)
		s.X = rangeConversion(s.X, info.method.Result, r)
	default:
		s.X = p.rewriteExpr(s.X, env, info)
		s.Y = p.rewriteExpr(s.Y, env, info)
		p.rewriteStmt(s.Body, env, info)
		p.rewriteStmt(s.Else, env, info)
	}
}

// A missing constructor assignment is not zero in JavaScript. Refuse a field
// proof unless initialization dominates all ordinary method calls. Only direct
// constructor/static-initializer assignments are accepted here.
func numberFieldInitialized(c *hir.Class, f hir.Field) bool {
	m := c.Ctor
	if f.Static {
		m = nil
		for _, candidate := range c.Methods {
			if candidate.Name == "class_constructor" {
				m = candidate
			}
		}
	}
	if m == nil {
		return false
	}
	readsField := func(e *hir.Expr) bool {
		return e.Name == f.Name && (e.Kind == hir.StaticGet && e.Owner == c.Name || e.Kind == hir.FieldGet && e.X != nil && e.X.Type.Name == c.Name)
	}
	var assigned func(*hir.Stmt) bool
	assigned = func(s *hir.Stmt) bool {
		if s == nil {
			return false
		}
		if s.Kind == hir.Assign && s.X != nil && s.X.Name == f.Name && (s.X.Kind == hir.FieldGet && s.X.X != nil && s.X.X.Kind == hir.This || s.X.Kind == hir.StaticGet && s.X.Owner == c.Name) {
			unsafe := numberFieldEffects(s.Y)
			walkNumberStmt(&hir.Stmt{Kind: hir.ExprStmt, X: s.Y}, func(s *hir.Stmt) {}, func(e *hir.Expr) {
				if readsField(e) {
					unsafe = true
				}
			})
			return !unsafe
		}
		if s.Kind == hir.Block {
			for _, child := range s.List {
				if assigned(child) {
					return true
				}
				if child == nil {
					continue
				}
				if child.Kind == hir.Return || child.Kind == hir.Throw || child.Kind == hir.If || child.Kind == hir.Try {
					return false
				}
				earlyCall := false
				walkNumberStmt(child, func(s *hir.Stmt) {}, func(e *hir.Expr) {
					if readsField(e) || e.Kind == hir.DirectCall || e.Kind == hir.VirtualCall || e.Kind == hir.SuperCall || e.Kind == hir.New || e.Kind == hir.Seq {
						earlyCall = true
					}
				})
				if earlyCall {
					return false
				}
			}
		}
		return false
	}
	return assigned(m.Body)
}
