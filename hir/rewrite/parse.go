package rewrite

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type term struct {
	value          string
	variable, wild bool
}
type atom struct {
	pred     string
	args     []term
	negative bool
}
type clause struct {
	name            string
	priority, bound int
	head            atom
	body            []atom
}

// Rules is an immutable parsed set of fact and rewrite rules.
type Rules struct {
	clauses  []clause
	rewrites []rewriteRule
}
type sexpr struct {
	text   string
	quoted bool
	list   []sexpr
}

func parseSexpr(src string) ([]sexpr, error) {
	pos := 0
	var read func() (sexpr, error)
	skip := func() {
		for pos < len(src) {
			if unicode.IsSpace(rune(src[pos])) {
				pos++
				continue
			}
			if src[pos] == ';' {
				for pos < len(src) && src[pos] != '\n' {
					pos++
				}
				continue
			}
			break
		}
	}
	read = func() (sexpr, error) {
		skip()
		if pos == len(src) {
			return sexpr{}, fmt.Errorf("unexpected end")
		}
		c := src[pos]
		pos++
		if c == '(' {
			n := sexpr{list: []sexpr{}}
			for {
				skip()
				if pos == len(src) {
					return n, fmt.Errorf("unclosed list")
				}
				if src[pos] == ')' {
					pos++
					return n, nil
				}
				x, e := read()
				if e != nil {
					return n, e
				}
				n.list = append(n.list, x)
			}
		}
		if c == ')' {
			return sexpr{}, fmt.Errorf("unexpected )")
		}
		start := pos - 1
		if c == '"' {
			for pos < len(src) {
				c = src[pos]
				pos++
				if c == '\\' {
					pos++
					continue
				}
				if c == '"' {
					s, e := strconv.Unquote(src[start:pos])
					return sexpr{text: s, quoted: true}, e
				}
			}
			return sexpr{}, fmt.Errorf("unclosed string")
		}
		for pos < len(src) && !strings.ContainsRune("(); \t\r\n", rune(src[pos])) {
			pos++
		}
		return sexpr{text: src[start:pos]}, nil
	}
	var out []sexpr
	for {
		skip()
		if pos == len(src) {
			return out, nil
		}
		n, e := read()
		if e != nil {
			return nil, e
		}
		out = append(out, n)
	}
}
func parseAtom(n sexpr) (atom, error) {
	a := atom{}
	if len(n.list) == 2 && n.list[0].text == "not" {
		a.negative = true
		n = n.list[1]
	}
	if len(n.list) == 0 || n.list[0].text == "" || n.list[0].quoted {
		return a, fmt.Errorf("expected predicate")
	}
	a.pred = n.list[0].text
	for _, s := range n.list[1:] {
		if s.list != nil {
			return a, fmt.Errorf("nested term in %s", a.pred)
		}
		a.args = append(a.args, term{s.text, !s.quoted && strings.HasPrefix(s.text, "?"), !s.quoted && s.text == "_"})
	}
	return a, nil
}

// Parse accepts facts and (rule name priority (head ...) (base ...) (tail ...)
// (bound depth N)). Each base/tail is an alternative conjunction.
func Parse(src string) (*DB, *Rules, error) {
	nodes, err := parseSexpr(src)
	if err != nil {
		return nil, nil, err
	}
	db := NewDB()
	rs := &Rules{}
	for _, n := range nodes {
		if len(n.list) == 0 {
			return nil, nil, fmt.Errorf("expected fact or rule")
		}
		if n.list[0].text == "grace" {
			r, e := parseRewrite(n)
			if e != nil {
				return nil, nil, e
			}
			rs.rewrites = append(rs.rewrites, r)
			continue
		}
		if n.list[0].text == "fact" {
			a, e := parseAtom(sexpr{list: n.list[1:]})
			if e != nil {
				return nil, nil, e
			}
			var args []string
			for _, t := range a.args {
				if t.variable || t.wild {
					return nil, nil, fmt.Errorf("fact must be ground")
				}
				args = append(args, t.value)
			}
			if e = db.Add(a.pred, args...); e != nil {
				return nil, nil, e
			}
			continue
		}
		if n.list[0].text != "rule" || len(n.list) < 5 || n.list[1].list != nil {
			return nil, nil, fmt.Errorf("expected rule name priority head and case")
		}
		c := clause{name: n.list[1].text, bound: -1}
		c.priority, err = strconv.Atoi(n.list[2].text)
		if err != nil {
			return nil, nil, err
		}
		heads, cases := 0, 0
		for _, s := range n.list[3:] {
			if len(s.list) == 0 {
				return nil, nil, fmt.Errorf("empty rule section")
			}
			switch s.list[0].text {
			case "head":
				if len(s.list) != 2 {
					return nil, nil, fmt.Errorf("head needs one atom")
				}
				c.head, err = parseAtom(s.list[1])
				heads++
			case "bound":
				if len(s.list) != 3 || s.list[1].text != "depth" {
					return nil, nil, fmt.Errorf("expected bound depth N")
				}
				c.bound, err = strconv.Atoi(s.list[2].text)
				if c.bound < 0 {
					return nil, nil, fmt.Errorf("negative depth")
				}
			case "base", "tail":
				cases++
			default:
				return nil, nil, fmt.Errorf("unknown section %s", s.list[0].text)
			}
			if err != nil {
				return nil, nil, err
			}
		}
		if heads != 1 || cases == 0 || c.head.negative {
			return nil, nil, fmt.Errorf("rule needs one positive head and cases")
		}
		for _, s := range n.list[3:] {
			if s.list[0].text != "base" && s.list[0].text != "tail" {
				continue
			}
			cc := c
			for _, b := range s.list[1:] {
				a, e := parseAtom(b)
				if e != nil {
					return nil, nil, e
				}
				cc.body = append(cc.body, a)
			}
			rs.clauses = append(rs.clauses, cc)
		}
	}
	sort.SliceStable(rs.clauses, func(i, j int) bool { return rs.clauses[i].priority > rs.clauses[j].priority })
	sort.SliceStable(rs.rewrites, func(i, j int) bool { return rs.rewrites[i].priority > rs.rewrites[j].priority })
	return db, rs, nil
}
