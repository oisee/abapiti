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

func MeasureABAPTestGap(lowering *RegistryLowering) ABAPTestGap {
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
	for _, err := range abap.Audit(lowering.Prog) {
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
// exactly as the Go test harness does. Structural errors cannot be hidden.
func TrapABAPTestRefusals(lowering *RegistryLowering) error {
	hir.AssignSiteIDs(lowering.Prog)
	// The catalogue gate is stricter than individual emitter switch arms.
	// In particular ID-zero number.ceil sites must not escape merely because
	// an old switch arm happens to print some ABAP for them.
	testMethods(lowering.Prog, func(_ *hir.Class, m *hir.Method) {
		messages := []string{}
		visitTestBody(m.Body, func(x *hir.Expr) {
			if x.Kind == hir.RuntimeOp && !abap.SupportsRuntimeOp(x.Op) {
				messages = append(messages, fmt.Sprintf("node %d (%s): runtime operation %s has no ABAP emitter", x.ID, x.Source, x.Op))
			}
		})
		if len(messages) > 0 {
			sort.Strings(messages)
			m.Body = failureBody(m.Node, strings.Join(messages, "; "))
		}
	})
	for round := 0; round < 20; round++ {
		errors := abap.Audit(lowering.Prog)
		if len(errors) == 0 {
			return nil
		}
		ids := map[int]string{}
		owners := map[string][]string{}
		key := func(m *hir.Method) string { return fmt.Sprintf("%s:%d:%s:%s", m.SiteOwner, m.ID, m.Name, m.Source) }
		for _, err := range errors {
			if r, ok := err.(abap.Refusal); ok && r.Method != nil {
				owners[key(r.Method)] = append(owners[key(r.Method)], r.Error())
				continue
			}
			match := abapNodeError.FindStringSubmatch(err.Error())
			if match == nil {
				return fmt.Errorf("structural ABAP test refusal: %w", err)
			}
			id, _ := strconv.Atoi(match[1])
			if id == 0 {
				return err
			}
			ids[id] = err.Error()
		}
		n := 0
		testMethods(lowering.Prog, func(_ *hir.Class, m *hir.Method) {
			body := bodyIDs(m.Body)
			messages := append([]string{}, owners[key(m)]...)
			for id, message := range ids {
				if body[id] {
					messages = append(messages, message)
				}
			}
			if len(messages) > 0 {
				sort.Strings(messages)
				m.Body = failureBody(m.Node, strings.Join(messages, "; "))
				n++
			}
		})
		if n == 0 {
			return fmt.Errorf("unattributable ABAP test refusal: %s", errors[0])
		}
		(&lowerer{out: lowering.Prog}).repairFailedBodies()
	}
	return fmt.Errorf("ABAP test refusal trapping did not converge")
}
