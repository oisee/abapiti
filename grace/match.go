package grace

import (
	"fmt"
	"strconv"
)

// RewriteRule is an immutable, opaque parsed match/guard/action declaration.
type RewriteRule struct {
	name            string
	priority, bound int
	match           atom
	where           []atom
	action          atom
}

func parseRewrite(n sexpr) (RewriteRule, error) {
	r := RewriteRule{bound: -1}
	if len(n.list) < 6 {
		return r, fmt.Errorf("grace needs name, priority, match, where and action")
	}
	r.name = n.list[1].text
	var err error
	r.priority, err = strconv.Atoi(n.list[2].text)
	if err != nil {
		return r, err
	}
	seen := map[string]bool{}
	for _, s := range n.list[3:] {
		if len(s.list) == 0 {
			return r, fmt.Errorf("empty grace section")
		}
		k := s.list[0].text
		if seen[k] {
			return r, fmt.Errorf("duplicate grace %s", k)
		}
		seen[k] = true
		switch k {
		case "match", "action":
			if len(s.list) != 2 {
				return r, fmt.Errorf("%s needs one atom", k)
			}
			a, e := parseAtom(s.list[1])
			if e != nil {
				return r, e
			}
			if a.negative {
				return r, fmt.Errorf("negative %s", k)
			}
			if k == "match" {
				r.match = a
			} else {
				r.action = a
			}
		case "where":
			for _, v := range s.list[1:] {
				a, e := parseAtom(v)
				if e != nil {
					return r, e
				}
				r.where = append(r.where, a)
			}
		case "bound":
			if len(s.list) != 3 || s.list[1].text != "depth" {
				return r, fmt.Errorf("expected bound depth N")
			}
			r.bound, err = strconv.Atoi(s.list[2].text)
			if err != nil || r.bound < 0 {
				return r, fmt.Errorf("invalid rewrite depth")
			}
		default:
			return r, fmt.Errorf("unknown grace section %s", k)
		}
	}
	if !seen["match"] || !seen["where"] || !seen["action"] {
		return r, fmt.Errorf("grace needs match, where and action")
	}
	if r.match.pred != "node" || len(r.match.args) != 2 {
		return r, fmt.Errorf("match expects (node site kind)")
	}
	if (r.action.pred != "inline" || len(r.action.args) != 1) && (r.action.pred != "replace" || len(r.action.args) != 2) {
		return r, fmt.Errorf("unknown rewrite action")
	}
	bound := map[string]bool{}
	for _, a := range append([]atom{r.match}, r.where...) {
		if !a.negative && !comparison(a.pred) {
			for _, t := range a.args {
				if t.variable {
					bound[t.value] = true
				}
			}
		}
	}
	for _, a := range append([]atom{r.action}, r.where...) {
		if a.negative || comparison(a.pred) || a.pred == r.action.pred {
			for _, t := range a.args {
				if t.wild || t.variable && !bound[t.value] {
					return r, fmt.Errorf("%s: unbound action/guard", r.name)
				}
			}
		}
	}
	if r.action.args[0] != r.match.args[0] {
		return r, fmt.Errorf("action must rewrite the matched node")
	}
	return r, nil
}

// Rewrites returns opaque rewrite declarations in priority order. Syntax and
// join internals stay private; the adapter interprets the resulting actions.
func (r *Rules) Rewrites() []RewriteRule { return append([]RewriteRule(nil), r.rewrites...) }

// Action returns the native action name.
func (r RewriteRule) Action() string { return r.action.pred }

// Bound returns the callee depth bound, or -1 for no bound.
func (r RewriteRule) Bound() int { return r.bound }

// Kind returns the matched node kind and whether any kind can match.
func (r RewriteRule) Kind() (string, bool) {
	t := r.match.args[1]
	return t.value, t.variable || t.wild
}

// Validate checks match/guard arities against the current database.
func (r RewriteRule) Validate(db *DB) error {
	for _, a := range append([]atom{r.match}, r.where...) {
		if comparison(a.pred) && len(a.args) != 2 {
			return fmt.Errorf("%s needs two terms", a.pred)
		}
		if t := db.tables[a.pred]; t != nil && t.arity != len(a.args) {
			return fmt.Errorf("%s: inconsistent arity", a.pred)
		}
	}
	return nil
}

// Matcher is an opaque reusable indexed query, bound to one database. It must
// be rebuilt after invalidation and is not safe for concurrent use.
type Matcher struct {
	db   *DB
	rule RewriteRule
	plan *joinPlan
}

// Matcher compiles this declaration's guards and action against db. Validate
// the declaration first. The caller supplies the node match tuple to Match.
func (r RewriteRule) Matcher(db *DB) *Matcher {
	c := clause{head: r.action, body: r.where, bound: -1}
	return &Matcher{db: db, rule: r, plan: planJoin(db, c, -1, r.match.args)}
}

// Match returns action arguments in deterministic join order for a node tuple.
func (m *Matcher) Match(node Tuple) []Tuple {
	if len(node) != len(m.rule.match.args) {
		return nil
	}
	env, ok := matches(m.rule.match, &row{args: node}, map[string]string{})
	if !ok {
		return nil
	}
	var choices []Tuple
	m.plan.run(m.db, m.db, env, func(a Tuple, _ int) { choices = append(choices, append(Tuple(nil), a...)) })
	return choices
}
