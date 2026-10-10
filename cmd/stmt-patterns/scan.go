package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// splitABAP ignores periods in quoted strings, string templates and comments.
// This is a lexical scanner for generated ABAP, not a general ABAP parser.
func splitABAP(src string) []statement {
	var out []statement
	var b strings.Builder
	line, start := 1, 1
	quote := byte(0)
	comment := false
	lineStart := true
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == '\n' {
			line++
			comment = false
			lineStart = true
			if quote == 0 {
				b.WriteByte(' ')
			} else {
				b.WriteByte(c)
			}
			continue
		}
		if comment {
			continue
		}
		if quote != 0 {
			b.WriteByte(c)
			if c == quote {
				if quote != '|' && i+1 < len(src) && src[i+1] == quote {
					b.WriteByte(src[i+1])
					i++
				} else {
					quote = 0
				}
			} else if quote == '|' && c == '\\' && i+1 < len(src) {
				i++
				b.WriteByte(src[i])
			}
			continue
		}
		if c == '"' || (lineStart && c == '*') {
			comment = true
			continue
		}
		if unicode.IsSpace(rune(c)) {
			b.WriteByte(c)
			continue
		}
		if strings.TrimSpace(b.String()) == "" {
			start = line
		}
		lineStart = false
		if c == '\'' || c == '`' || c == '|' {
			quote = c
			b.WriteByte(c)
			continue
		}
		b.WriteByte(c)
		if c == '.' && !(i > 0 && i+1 < len(src) && src[i-1] >= '0' && src[i-1] <= '9' && src[i+1] >= '0' && src[i+1] <= '9') {
			out = append(out, statement{Text: strings.Join(strings.Fields(b.String()), " "), Line: start})
			b.Reset()
		}
	}
	return out
}
func parseMethods(src string) []*method {
	var out []*method
	var m *method
	depth := 0
	for _, s := range splitABAP(src) {
		u := strings.ToUpper(s.Text)
		if strings.HasPrefix(u, "METHOD ") {
			m = &method{ABAP: strings.TrimSuffix(strings.TrimSpace(s.Text[len("METHOD "):]), ".")}
			depth = 0
			continue
		}
		if u == "ENDMETHOD." {
			if m != nil {
				out = append(out, m)
			}
			m = nil
			continue
		}
		if m == nil {
			continue
		}
		if u == "ENDLOOP." || u == "ENDWHILE." || u == "ENDDO." {
			depth--
			if depth < 0 {
				depth = 0
			}
		}
		s.Depth = depth
		m.Statements = append(m.Statements, s)
		if strings.HasPrefix(u, "LOOP ") || strings.HasPrefix(u, "WHILE ") || strings.HasPrefix(u, "DO ") || u == "DO." {
			depth++
		}
	}
	return out
}
func mix(s string) string {
	u := strings.ToUpper(s)
	switch {
	case strings.HasPrefix(u, "DATA"):
		if strings.Contains(u, "= CAST ") {
			return "data_cast"
		}
		if strings.Contains(u, "= VALUE ") {
			return "data_value_init"
		}
		if strings.Contains(u, "= XSDBOOL(") {
			return "data_bool"
		}
		if strings.Contains(u, "=") {
			return "data_expression"
		}
		return "declaration"
	case strings.Contains(u, " ?= "):
		return "cast_assignment"
	case strings.HasPrefix(u, "IF ") || strings.HasPrefix(u, "ELSEIF "):
		return "condition"
	case strings.HasPrefix(u, "LOOP ") || strings.HasPrefix(u, "WHILE ") || strings.HasPrefix(u, "DO ") || u == "DO.":
		return "loop_entry"
	case strings.HasPrefix(u, "END") || u == "ELSE." || u == "TRY." || strings.HasPrefix(u, "CATCH "):
		return "control_marker"
	case strings.HasPrefix(u, "READ TABLE "):
		return "table_read"
	case strings.HasPrefix(u, "APPEND ") || strings.HasPrefix(u, "INSERT "):
		return "table_write"
	case assignRE.MatchString(s):
		return "assignment"
	case strings.HasPrefix(u, "RETURN") || strings.HasPrefix(u, "CONTINUE") || strings.HasPrefix(u, "EXIT") || strings.HasPrefix(u, "RAISE"):
		return "transfer"
	case strings.Contains(u, "->") || strings.Contains(u, "=>"):
		return "call"
	default:
		return strings.ToLower(strings.Fields(strings.TrimSuffix(s, "."))[0])
	}
}

// maskStrings preserves length, avoiding false variable uses inside literals.
func maskStrings(s string) string {
	b := []byte(s)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\'' && c != '`' && c != '|' {
			continue
		}
		q := c
		b[i] = ' '
		i++
		for i < len(s) {
			c = s[i]
			b[i] = ' '
			if q == '|' && c == '\\' && i+1 < len(s) {
				i++
				b[i] = ' '
				i++
				continue
			}
			if q == '|' && c == '{' {
				start := i + 1
				j := start
				for j < len(s) && s[j] != '}' {
					j++
				}
				if j < len(s) {
					copy(b[start:j], maskStrings(s[start:j]))
					i = j
					b[i] = ' '
				}
			}
			if c == q {
				if q != '|' && i+1 < len(s) && s[i+1] == q {
					i++
					b[i] = ' '
				} else {
					break
				}
			}
			i++
		}
	}
	return string(b)
}

func uses(s, v string) int {
	n := 0
	for _, w := range wordRE.FindAllString(maskStrings(s), -1) {
		if strings.EqualFold(w, v) {
			n++
		}
	}
	return n
}
func totalUses(ss []statement, v string) int {
	n := 0
	for _, s := range ss {
		n += uses(s.Text, v)
	}
	return n
}
func isTemp(v string) bool { return tempRE.FindString(v) == v }
func barrier(s string) bool {
	u := strings.ToUpper(s)
	for _, p := range []string{"IF ", "ELSE", "END", "LOOP ", "WHILE ", "DO ", "TRY", "CATCH", "RETURN", "CONTINUE", "EXIT", "RAISE"} {
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	return false
}
func detect(m *method) []occurrence {
	var out []occurrence
	add := func(p string, i int, ex string) {
		s := m.Statements[i]
		v := m.Patterns[p]
		if s.Depth > 0 {
			v[1]++
		} else {
			v[0]++
		}
		m.Patterns[p] = v
		out = append(out, occurrence{identity(m), p, ex, s.Line, s.Depth, 1})
	}
	for i, s := range m.Statements {
		d := declRE.FindStringSubmatch(s.Text)
		if d != nil {
			v, expr := d[1], d[2]
			once := totalUses(m.Statements[i+1:], v) == 1
			if i+1 < len(m.Statements) && once && isTemp(v) && m.Statements[i+1].Depth == s.Depth {
				next := m.Statements[i+1].Text
				a := assignRE.FindStringSubmatch(next)
				adj := !barrier(next)
				if adj && a != nil && strings.EqualFold(strings.TrimSpace(a[3]), v) && a[2] == "=" {
					add("temp_assign", i, s.Text+" "+next)
					if strings.EqualFold(a[1], "result") {
						add("return_copy", i, s.Text+" "+next)
					}
				} else if adj && uses(next, v) == 1 && strings.Contains(next, "(") {
					add("temp_argument", i, s.Text+" "+next)
				}

				if strings.Contains(strings.ToLower(next), strings.ToLower(v)+"->") && uses(next, v) == 1 && strings.Contains(next, "(") && (strings.HasPrefix(strings.ToUpper(expr), "CAST ") || !strings.Contains(expr, "(")) {
					add("receiver_copy", i, s.Text+" "+next)
				}
				if strings.HasPrefix(strings.ToUpper(expr), "XSDBOOL(") && strings.EqualFold(next, "IF "+v+" = abap_true.") {
					add("bool_if", i, s.Text+" "+next)
				}
			}
			if strings.HasPrefix(strings.ToUpper(expr), "VALUE ") && regexp.MustCompile(`(?i)^VALUE\s+\w+(?:::\w+)?\(\s*\)$`).MatchString(expr) {
				// Only a straight-line overwrite before any read. Any intervening call
				// or field-symbol/reference access stops this deliberately conservative scan.
				for j := i + 1; j < len(m.Statements); j++ {
					n := m.Statements[j]
					a := assignRE.FindStringSubmatch(n.Text)
					if n.Depth != s.Depth || barrier(n.Text) {
						break
					}
					if a != nil && strings.EqualFold(a[1], v) && a[2] == "=" && uses(a[3], v) == 0 {
						add("value_init_overwritten", i, s.Text+" ... "+n.Text)
						break
					}
					if uses(n.Text, v) > 0 || strings.HasPrefix(strings.ToUpper(n.Text), "ASSIGN ") || strings.HasPrefix(strings.ToUpper(n.Text), "GET REFERENCE") {
						break
					}
				}
			}
			if literalConstructor(expr) {
				add("literal_constructor", i, s.Text)
			}
			if once && isTemp(v) && strings.HasPrefix(strings.ToUpper(expr), "CAST ") {
				add("cast_temp", i, s.Text)
			}
			if literalScalar(expr) {
				add("literal_scalar", i, s.Text)
			}
		}
		if a := assignRE.FindStringSubmatch(s.Text); a != nil {
			v, op, expr := a[1], a[2], a[3]
			if strings.HasPrefix(v, "range_int") {
				add("range_bound_assignment", i, s.Text)
			}
			if isTemp(v) && uses(expr, v) == 0 && totalUses(m.Statements[i+1:], v) == 1 && i+1 < len(m.Statements) {
				next := m.Statements[i+1].Text
				if op == "?=" {
					add("cast_temp", i, s.Text+" "+next)
				}
				if op == "=" {
					if dest := assignRE.FindStringSubmatch(next); dest != nil && dest[2] == "=" && strings.EqualFold(dest[3], v) {
						add("temp_assign", i, s.Text+" "+next)
						if strings.EqualFold(dest[1], "result") {
							add("return_copy", i, s.Text+" "+next)
						}
					}
				}
				if strings.HasPrefix(strings.ToUpper(expr), "XSDBOOL(") && strings.EqualFold(next, "IF "+v+" = abap_true.") {
					add("bool_if", i, s.Text+" "+next)
				}
			}
		}
		if strings.HasPrefix(strings.ToUpper(s.Text), "CLEAR ") {
			v := strings.TrimSuffix(strings.TrimSpace(s.Text[len("CLEAR "):]), ".")
			if isTemp(v) {
				for j := i + 1; j < len(m.Statements); j++ {
					n := m.Statements[j]
					if n.Depth != s.Depth || barrier(n.Text) {
						break
					}
					a := assignRE.FindStringSubmatch(n.Text)
					if a != nil && strings.EqualFold(a[1], v) && uses(a[3], v) == 0 {
						add("clear_before_write", i, s.Text+" ... "+n.Text)
						break
					}
					if strings.EqualFold(n.Text, "CREATE OBJECT "+v+".") {
						add("clear_before_write", i, s.Text+" ... "+n.Text)
						break
					}
					if uses(n.Text, v) > 0 || strings.HasPrefix(strings.ToUpper(n.Text), "ASSIGN ") {
						break
					}
				}
			}
		}
		// Explicitly declared reference, downcast, then a single adjacent use.

		if strings.HasPrefix(strings.ToUpper(s.Text), "READ TABLE ") && i+1 < len(m.Statements) {
			next := m.Statements[i+1].Text
			if assignRE.MatchString(next) && strings.Contains(s.Text, " INTO ") {
				add("table_read_copy", i, s.Text+" "+next)
			}
		}
		if strings.HasPrefix(strings.ToUpper(s.Text), "APPEND VALUE ") {
			expr := strings.TrimSpace(s.Text[len("APPEND "):])
			if at := strings.LastIndex(strings.ToUpper(expr), " TO "); at >= 0 {
				expr = expr[:at]
			}
			if literalConstructor(expr) {
				add("literal_range_build", i, s.Text)
			}
		}
	}
	return out
}
func literalConstructor(expr string) bool {
	u := strings.ToUpper(expr)
	if !strings.HasPrefix(u, "VALUE ") {
		return false
	}
	pos := strings.Index(expr, "(")
	if pos < 0 || strings.TrimSpace(expr[pos+1:]) == ")" {
		return false
	}
	body := maskStrings(expr[pos+1:])
	body = regexp.MustCompile(`\b\w+\s*=`).ReplaceAllString(body, "")
	for _, w := range wordRE.FindAllString(body, -1) {
		switch strings.ToLower(w) {
		case "abap_true", "abap_false", "initial":
		default:
			return false
		}
	}
	return true
}
func normalize(s string) string {
	// Replace literal values, local temp identities and hashed names, retaining
	// constructor/call/assignment shapes. N-grams may cross control boundaries.
	var b strings.Builder
	q := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if q != 0 {
			if c == q {
				if q != '|' && i+1 < len(s) && s[i+1] == q {
					i++
				} else {
					q = 0
					b.WriteString("LIT")
				}
			} else if q == '|' && c == '\\' && i+1 < len(s) {
				i++
			}
			continue
		}
		if c == '\'' || c == '`' || c == '|' {
			q = c
			continue
		}
		b.WriteByte(c)
	}
	s = tempRE.ReplaceAllString(b.String(), "TMP")
	s = nameRE.ReplaceAllString(s, "NAME")
	s = numberRE.ReplaceAllString(s, "NUM")
	return strings.ToUpper(strings.Join(strings.Fields(s), " "))
}
func validateMethods(ms []*method) error {
	for _, m := range ms {
		if m.TS == 0 && m.Source != "" {
			return fmt.Errorf("missing TS body for %s (%s)", identity(m), m.Source)
		}
	}
	return nil
}

func literalScalar(expr string) bool {
	d := regexp.MustCompile(`(?i)^CONV\s+\w+\(\s*(.+)\s*\)$`).FindStringSubmatch(expr)
	if d == nil {
		return false
	}
	inner := strings.TrimSpace(d[1])
	if inner == "" {
		return false
	}
	if regexp.MustCompile(`(?i)^(?:[-+]?[0-9]+|abap_true|abap_false)$`).MatchString(inner) {
		return true
	}
	return strings.TrimSpace(maskStrings(inner)) == "" && (inner[0] == '\'' || inner[0] == '`' || inner[0] == '|')
}
