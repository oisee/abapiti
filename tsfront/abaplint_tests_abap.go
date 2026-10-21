package tsfront

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

// ABAPTestGap measures the unpruned test HIR before any refusal is trapped.
// Runtime sites are counted before the ABAP passes; emitter diagnostics after
// those passes are deduplicated by diagnostic (inline copies are not new sites).
type ABAPTestGap struct {
	Files                 int          `json:"files"`
	NonfiniteLiteralSites int          `json:"nonfiniteLiteralSites"`
	UnsupportedCatalogue  []string     `json:"unsupportedCatalogue"`
	Runtime               []ABAPGapRow `json:"runtime"`
	Refusals              []ABAPGapRow `json:"refusals"`
}
type ABAPGapRow struct {
	Reason   string   `json:"reason"`
	Sites    int      `json:"sites"`
	Examples []string `json:"examples"`
}

func visitTestBody(s *hir.Stmt, f func(*hir.Expr)) {
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
		visitTestBody(x.Stmt, f)
	}
	expr(s.X)
	expr(s.Y)
	visitTestBody(s.Body, f)
	visitTestBody(s.Else, f)
	for _, child := range s.List {
		visitTestBody(child, f)
	}
}
func testMethods(p *hir.Program, f func(*hir.Class, *hir.Method)) {
	for _, c := range p.Classes {
		if c.Ctor != nil {
			f(c, c.Ctor)
		}
		for _, m := range c.Methods {
			f(c, m)
		}
	}
}

var abapNodeError = regexp.MustCompile(`^node ([0-9]+)(?: \([^\n]*?\))?: `)

var abapOperation = regexp.MustCompile(`runtime operation ([A-Za-z0-9_.]+)`)

var abapNodeLocated = regexp.MustCompile(`^node ([0-9]+) \(([^\n]*?)\): `)

func MeasureABAPTestGap(lowering *RegistryLowering) ABAPTestGap {
	return measureABAPTestGap(lowering, false)
}

// MeasureABAPTestGapKeepGoing audits with the ownerless-operation gate open:
// the Refusals inventory then survives union families and lists every
// located emitter refusal instead of the first catalogue error.
func MeasureABAPTestGapKeepGoing(lowering *RegistryLowering) ABAPTestGap {
	return measureABAPTestGap(lowering, true)
}

// UnattributableRefusal is an ABAP refusal with no owning body to trap:
// type- or declaration-level sites such as union.declarations. Measurement
// records them; the unemittable declaration is omitted when the emitter
// writes nothing for it, and owning bodies keep the whole-body trap policy
// (module initializers: from the first refused top-level statement, trapBody).
type UnattributableRefusal struct {
	Node      int    `json:"node"`
	Location  string `json:"location"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

func measureABAPTestGap(lowering *RegistryLowering, keepGoing bool) ABAPTestGap {
	hir.AssignSiteIDs(lowering.Prog)
	gap := ABAPTestGap{Files: len(lowering.Files), Runtime: []ABAPGapRow{}, Refusals: []ABAPGapRow{}}
	_, gap.UnsupportedCatalogue = abap.RuntimeOps()
	sites := map[string]map[string]bool{}
	add := func(reason, site string) {
		if sites[reason] == nil {
			sites[reason] = map[string]bool{}
		}
		sites[reason][site] = true
	}
	testMethods(lowering.Prog, func(c *hir.Class, m *hir.Method) {
		ordinal := 0
		visitTestBody(m.Body, func(x *hir.Expr) {
			ordinal++
			if x.Kind == hir.Lit {
				if v, ok := x.Value.(float64); ok && (math.IsNaN(v) || math.IsInf(v, 0)) {
					gap.NonfiniteLiteralSites++
				}
			}
			if x.Kind == hir.RuntimeOp && !abap.SupportsRuntimeOp(x.Op) {
				add(x.Op, fmt.Sprintf("%s.%s:%d:%d (%s)", c.Name, m.Name, x.ID, ordinal, x.Source))
			}
		})
	})
	rows := func() []ABAPGapRow {
		out := []ABAPGapRow{}
		for reason, ss := range sites {
			examples := []string{}
			for s := range ss {
				examples = append(examples, s)
			}
			sort.Strings(examples)
			n := len(examples)
			if len(examples) > 3 {
				examples = examples[:3]
			}
			out = append(out, ABAPGapRow{reason, n, examples})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Reason < out[j].Reason })
		return out
	}
	gap.Runtime = rows()
	sites = map[string]map[string]bool{}
	for _, err := range audit(lowering, keepGoing) {
		message := err.Error()
		// Runtime-operation sites already have their exact pre-pass inventory.
		if strings.Contains(message, "unsupported ABAP runtime op ") {
			continue
		}
		reason := abapNodeError.ReplaceAllString(message, "")
		if r, ok := err.(abap.Refusal); ok && r.Method != nil {
			message += fmt.Sprintf(" [%s@%s; node %d %s]", r.Method.Name, r.Method.Source, r.Node.ID, r.Node.Source)
		}
		add(reason, message)
	}
	gap.Refusals = rows()
	return gap
}

// TrapABAPTestRefusals preserves signatures and refuses complete owning bodies,
// exactly as the Go test harness does; module initializers trap from their
// first refused top-level statement on (trapBody). Structural errors cannot
// be hidden.
func TrapABAPTestRefusals(lowering *RegistryLowering) error {
	_, err := trapABAPTestRefusals(lowering, false)
	return err
}

// TrapABAPTestRefusalsKeepGoing is the measurement variant: refusals with no
// owning body (type or declaration level) are recorded and the unemittable
// declaration omitted — safe exactly when the emitter writes no ABAP for it,
// as for union families — so emission finishes and lists every refusal.
// Body-level refusals keep the TS-HA whole-owning-body policy (trapBody).
func TrapABAPTestRefusalsKeepGoing(lowering *RegistryLowering) ([]UnattributableRefusal, error) {
	return trapABAPTestRefusals(lowering, true)
}

func audit(lowering *RegistryLowering, keepGoing bool) []error {
	if keepGoing {
		return abap.AuditKeepGoing(lowering.Prog)
	}
	return abap.Audit(lowering.Prog)
}

func trapABAPTestRefusals(lowering *RegistryLowering, keepGoing bool) ([]UnattributableRefusal, error) {
	hir.AssignSiteIDs(lowering.Prog)
	// The catalogue gate is stricter than individual emitter switch arms.
	// In particular ID-zero array.has and number.ceil sites must not escape
	// merely because an old switch arm happens to print some ABAP for them.
	testMethods(lowering.Prog, func(_ *hir.Class, m *hir.Method) {
		var refusals []bodyRefusal
		visit := func(s *hir.Stmt, statement int) {
			visitTestBody(s, func(x *hir.Expr) {
				if x.Kind == hir.RuntimeOp && !abap.SupportsRuntimeOp(x.Op) {
					refusals = append(refusals, bodyRefusal{x.ID, statement, fmt.Sprintf("node %d (%s): runtime operation %s has no ABAP emitter", x.ID, x.Source, x.Op)})
				}
			})
		}
		// Synthetic ID-zero operations are located by their top-level
		// statement directly, so they too trap only from that statement on.
		if moduleInitializer(m) {
			for i, s := range m.Body.List {
				visit(s, i)
			}
		} else {
			visit(m.Body, -1)
		}
		if len(refusals) > 0 {
			trapBody(m, refusals)
		}
	})
	var unattributable []UnattributableRefusal
	record := func(node hir.Node, operation, message string) {
		unattributable = append(unattributable, UnattributableRefusal{Node: node.ID, Location: node.Source, Operation: operation, Message: message})
	}
	if keepGoing {
		// Tagged union families emit through the ABAP union ABI (payload
		// objects behind one payload interface), so nothing is erased here.
		// Ownerless catalogue operations: with every owning body already
		// trapped, whatever UsedRuntimeOps still reports lives outside the
		// test bodies (union declarations, interface signatures). Interfaces
		// emit signatures only, so omission keeps the ABAP syntactically
		// complete.
		owned := map[int]bool{}
		testMethods(lowering.Prog, func(_ *hir.Class, m *hir.Method) {
			for id := range bodyIDs(m.Body) {
				owned[id] = true
			}
		})
		ops := []string{}
		used := hir.UsedRuntimeOps(lowering.Prog)
		for op := range used {
			if !abap.SupportsRuntimeOp(op) {
				ops = append(ops, op)
			}
		}
		sort.Strings(ops)
		for _, op := range ops {
			if node := used[op]; !owned[node.ID] {
				record(node, op, fmt.Sprintf("node %d (%s): runtime operation %s has no emitter for selected target ABAP", node.ID, node.Source, op))
			}
		}
	}
	for round := 0; round < 20; round++ {
		errors := audit(lowering, keepGoing)
		if len(errors) == 0 {
			return unattributable, nil
		}
		ids := map[int]string{}
		owners := map[string][]bodyRefusal{}
		key := func(m *hir.Method) string { return fmt.Sprintf("%s:%d:%s:%s", m.SiteOwner, m.ID, m.Name, m.Source) }
		for _, err := range errors {
			if r, ok := err.(abap.Refusal); ok && r.Method != nil {
				id := r.Node.ID
				if match := abapNodeError.FindStringSubmatch(r.Error()); id == 0 && match != nil {
					id, _ = strconv.Atoi(match[1])
				}
				owners[key(r.Method)] = append(owners[key(r.Method)], bodyRefusal{id, -1, r.Error()})
				continue
			}
			match := abapNodeError.FindStringSubmatch(err.Error())
			if match == nil {
				return unattributable, fmt.Errorf("structural ABAP test refusal: %w", err)
			}
			id, _ := strconv.Atoi(match[1])
			if id == 0 {
				return unattributable, err
			}
			ids[id] = err.Error()
		}
		n := 0
		testMethods(lowering.Prog, func(_ *hir.Class, m *hir.Method) {
			body := bodyIDs(m.Body)
			refusals := append([]bodyRefusal{}, owners[key(m)]...)
			for id, message := range ids {
				if body[id] {
					refusals = append(refusals, bodyRefusal{id, -1, message})
				}
			}
			if len(refusals) > 0 {
				trapBody(m, refusals)
				n++
			}
		})
		if n == 0 {
			if !keepGoing {
				return unattributable, fmt.Errorf("unattributable ABAP test refusal: %s", errors[0])
			}
			// Nothing more is attributable: remaining located errors have no
			// owning body. Record them; emission may still refuse them, which
			// then names the next blocking construct precisely.
			seen := map[string]bool{}
			for _, row := range unattributable {
				seen[fmt.Sprintf("%d\x00%s", row.Node, row.Message)] = true
			}
			for _, err := range errors {
				row, ok := unattributableRefusal(err)
				if !ok {
					continue
				}
				if k := fmt.Sprintf("%d\x00%s", row.Node, row.Message); !seen[k] {
					seen[k] = true
					unattributable = append(unattributable, row)
				}
			}
			return unattributable, nil
		}
		(&lowerer{out: lowering.Prog}).repairFailedBodies()
	}
	return unattributable, fmt.Errorf("ABAP test refusal trapping did not converge")
}

// refusalOperation extracts the runtime operation a located refusal names, if
// any, so unattributable rows can be grouped by operation.
func refusalOperation(message string) string {
	if match := abapOperation.FindStringSubmatch(message); match != nil {
		return match[1]
	}
	return ""
}

// unattributableRefusal parses a located "node N (source): ..." refusal into a
// recordable row; false when the error carries no location.
func unattributableRefusal(err error) (UnattributableRefusal, bool) {
	message := err.Error()
	match := abapNodeLocated.FindStringSubmatch(message)
	if match == nil {
		return UnattributableRefusal{}, false
	}
	id, _ := strconv.Atoi(match[1])
	return UnattributableRefusal{Node: id, Location: match[2], Operation: refusalOperation(message), Message: message}, true
}

// bodyRefusal is one located refusal inside an owning body: node ID 0 is
// unknown; statement is the module initializer's top-level statement index
// when the caller already knows it, else -1.
type bodyRefusal struct {
	id, statement int
	message       string
}

// trapBody replaces a refused owning body by a located trap. A module
// initializer instead keeps the top-level statements before its first refused
// statement and traps there: initialization proceeds exactly as far as
// JavaScript would if that statement threw. The unreachable rest is dropped,
// and the failure is rethrown on every later access (hir.Method.InitFailure),
// so no reader ever sees the partially initialized module state.
func trapBody(m *hir.Method, refusals []bodyRefusal) {
	if first, ok := firstRefusedStatement(m, refusals); ok {
		list := m.Body.List
		messages := []string{}
		statement := bodyIDs(list[first])
		for _, r := range refusals {
			if r.statement == first || r.statement < 0 && statement[r.id] {
				messages = append(messages, r.message)
			}
		}
		sort.Strings(messages)
		kept := append(append([]*hir.Stmt{}, list[:first]...), failureBody(list[first].Node, strings.Join(messages, "; ")).List...)
		m.Body = &hir.Stmt{Node: m.Body.Node, Kind: hir.Block, List: kept}
		m.InitFailure = true
		return
	}
	messages := []string{}
	for _, r := range refusals {
		messages = append(messages, r.message)
	}
	sort.Strings(messages)
	m.Body = failureBody(m.Node, strings.Join(messages, "; "))
}

// firstRefusedStatement locates the earliest top-level statement of a module
// initializer that contains a refusal; false when any refusal is not inside
// a top-level statement (or the method is no module initializer).
func firstRefusedStatement(m *hir.Method, refusals []bodyRefusal) (int, bool) {
	if !moduleInitializer(m) {
		return 0, false
	}
	statements := make([]map[int]bool, len(m.Body.List))
	for i, s := range m.Body.List {
		statements[i] = bodyIDs(s)
	}
	first := len(statements)
	for _, r := range refusals {
		if r.statement >= 0 {
			first = min(first, r.statement)
			continue
		}
		found := false
		for i, ids := range statements {
			if r.id != 0 && ids[r.id] {
				first, found = min(first, i), true
				break
			}
		}
		if !found {
			return 0, false
		}
	}
	return first, true
}

// moduleInitializer: a static initializer whose top-level statements run in
// order, a module's or a class's static fields (both are class_constructor).
func moduleInitializer(m *hir.Method) bool {
	return m.Static && m.Name == "class_constructor" && m.Body != nil && m.Body.Kind == hir.Block && !isFailureBody(m.Body)
}
