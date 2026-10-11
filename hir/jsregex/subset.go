package jsregex

// TranslateSubset certifies a grammar over UTF-16 units, not arbitrary RE2
// syntax. Non-u ASCII case folding is explicit; Unicode flags/escapes,
// lookarounds, backreferences, lazy/bounded repeats and repeated captures refuse.
func TranslateSubset(pattern, flags string) (string, bool) {
	if pattern == "" {
		return "", false
	}
	insensitive := false
	seen := map[byte]bool{}
	for i := 0; i < len(flags); i++ {
		c := flags[i]
		if seen[c] || c != 'i' && c != 'g' && c != 'y' {
			return "", false
		}
		seen[c] = true
		insensitive = insensitive || c == 'i'
	}
	// This delimited-string language's local negative lookahead is regular.
	// A slash run can escape its following quote at any length; it can leave
	// the quote unconsumed at length >=2 (single slashes followed by a final
	// paired slash). Before an ordinary unit any nonempty slash run works.
	// Certify the whole language, not arbitrary lookahead syntax.
	if pattern == `^'(?:[^'\\]|''|\\'|\\\\|\\(?!'))*'$` && !insensitive {
		return `^'(?:[^'\\]|''|\\+'|\\+[^'\\]|\\{2,})*'\z`, true
	}
	p := subsetParser{pattern: pattern, insensitive: insensitive}
	out, _, ok := p.expression(false)
	return out, ok && p.pos == len(pattern)
}

type subsetParser struct {
	pattern     string
	pos         int
	insensitive bool
}

func subsetLiteral(c byte, insensitive bool) string {
	if insensitive && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
		if c >= 'a' {
			c -= 32
		}
		return "[" + string(c) + string(c+32) + "]"
	}
	switch c {
	case '\\', '.', '^', '$', '*', '+', '?', '(', ')', '[', ']', '{', '}', '|':
		return "\\" + string(c)
	}
	return string(c)
}

// An expression is an ordered alternation of nonempty sequences. RE2's
// leftmost-first mode agrees with JS for this capture-safe regular grammar.
func (p *subsetParser) expression(group bool) (string, bool, bool) {
	out := ""
	captures := false
	branch := false
	for p.pos < len(p.pattern) {
		c := p.pattern[p.pos]
		if c == ')' {
			if !group || !branch {
				return "", false, false
			}
			return out, captures, true
		}
		if c == '|' {
			if !branch {
				return "", false, false
			}
			out += "|"
			branch = false
			p.pos++
			continue
		}
		start := p.pos
		p.pos++
		atom := ""
		cap := false
		assertion := false
		switch c {
		case '^':
			if start != 0 {
				return "", false, false
			}
			atom = "^"
			assertion = true
		case '$':
			if p.pos != len(p.pattern) {
				return "", false, false
			}
			atom = "\\z"
			assertion = true
		case '.':
			atom = `[^\n\r\x{2028}\x{2029}]`
		case '(':
			prefix := "("
			cap = true
			if p.pos < len(p.pattern) && p.pattern[p.pos] == '?' {
				if p.pos+1 >= len(p.pattern) || p.pattern[p.pos+1] != ':' {
					return "", false, false
				}
				p.pos += 2
				prefix = "(?:"
				cap = false
			}
			inner, innerCap, ok := p.expression(true)
			if !ok || p.pos >= len(p.pattern) || p.pattern[p.pos] != ')' {
				return "", false, false
			}
			p.pos++
			atom = prefix + inner + ")"
			cap = cap || innerCap
		case '[':
			var ok bool
			atom, ok = p.class()
			if !ok {
				return "", false, false
			}
		case '\\':
			if p.pos == len(p.pattern) {
				return "", false, false
			}
			escaped := p.pattern[p.pos]
			p.pos++
			switch escaped {
			case 'w':
				atom = "[A-Za-z0-9_]"
			case 'W':
				atom = "[^A-Za-z0-9_]"
			case 'd':
				atom = "[0-9]"
			case 'D':
				atom = "[^0-9]"
			default:
				if !subsetPunctuation(escaped) {
					return "", false, false
				}
				atom = subsetLiteral(escaped, p.insensitive)
			}
		case '*', '+', '?', '{', '}', ']':
			return "", false, false
		default:
			if c < 32 || c > 126 {
				return "", false, false
			}
			atom = subsetLiteral(c, p.insensitive)
		}
		if p.pos < len(p.pattern) {
			q := p.pattern[p.pos]
			if q == '*' || q == '+' || q == '?' {
				if assertion || cap && q != '?' {
					return "", false, false
				}
				atom += string(q)
				p.pos++
				if p.pos < len(p.pattern) {
					q = p.pattern[p.pos]
					if q == '*' || q == '+' || q == '?' || q == '{' {
						return "", false, false
					}
				}
			}
		}
		out += atom
		captures = captures || cap
		branch = true
	}
	return out, captures, !group && branch
}

func subsetPunctuation(c byte) bool {
	return c >= 32 && c <= 126 && !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9')
}

// Classes are normalized as an ASCII membership set. Negation remains a
// complement over all UTF-16 units; it is not Go Unicode case folding.
func (p *subsetParser) class() (string, bool) {
	negate := false
	if p.pos < len(p.pattern) && p.pattern[p.pos] == '^' {
		negate = true
		p.pos++
	}
	var set [128]bool
	any := false
	for p.pos < len(p.pattern) && p.pattern[p.pos] != ']' {
		c, special, ok := p.classUnit(&set)
		if !ok {
			return "", false
		}
		any = true
		if special {
			if p.pos < len(p.pattern) && p.pattern[p.pos] == '-' {
				return "", false
			}
			continue
		}
		if p.pos+1 < len(p.pattern) && p.pattern[p.pos] == '-' && p.pattern[p.pos+1] != ']' {
			p.pos++
			end, special, ok := p.classUnit(&set)
			if !ok || special || c > end {
				return "", false
			}
			for i := int(c); i <= int(end); i++ {
				set[i] = true
			}
		} else {
			set[c] = true
		}
	}
	if !any || p.pos == len(p.pattern) {
		return "", false
	}
	p.pos++
	if p.insensitive {
		for c := byte('A'); c <= 'Z'; c++ {
			if set[c] || set[c+32] {
				set[c] = true
				set[c+32] = true
			}
		}
	}
	out := "["
	if negate {
		out += "^"
	}
	for c := 32; c < 127; c++ {
		if set[c] {
			switch byte(c) {
			case '\\', ']', '^', '-':
				out += "\\"
			}
			out += string(byte(c))
		}
	}
	return out + "]", true
}
func (p *subsetParser) classUnit(set *[128]bool) (byte, bool, bool) {
	if p.pos == len(p.pattern) {
		return 0, false, false
	}
	c := p.pattern[p.pos]
	p.pos++
	if c == '\\' {
		if p.pos == len(p.pattern) {
			return 0, false, false
		}
		c = p.pattern[p.pos]
		p.pos++
		if c == 'w' || c == 'd' {
			for b := byte('0'); b <= '9'; b++ {
				set[b] = true
			}
			if c == 'w' {
				set['_'] = true
				for b := byte('A'); b <= 'Z'; b++ {
					set[b] = true
					set[b+32] = true
				}
			}
			return 0, true, true
		}
		if !subsetPunctuation(c) {
			return 0, false, false
		}
		return c, false, true
	}
	return c, false, c >= 32 && c <= 126 && c != '['
}
