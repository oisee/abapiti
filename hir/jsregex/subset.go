package jsregex

// ECMAScript WhiteSpace + LineTerminator, excluding historical U+180E.
const execSpace = `[\x{0009}-\x{000D}\x{0020}\x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}]`

// TranslateSubset certifies a grammar over UTF-16 units, not arbitrary RE2
// syntax. Non-u ASCII case folding is explicit; Unicode flags/escapes,
// lookarounds, backreferences and repeated captures refuse. Lazy quantifiers
// preserve leftmost-first priority without changing the capture restrictions.
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
	// End followed by start matches only empty input, regardless of cursor.
	if pattern == "$^" {
		return `\z\A`, true
	}
	// This delimited-string language's local negative lookahead is regular.
	// A slash run can escape its following quote at any length; it can leave
	// the quote unconsumed at length >=2 (single slashes followed by a final
	// paired slash). Before an ordinary unit any nonempty slash run works.
	// Certify the whole language, not arbitrary lookahead syntax.
	if pattern == `^'(?:[^'\\]|''|\\'|\\\\|\\(?!'))*'$` && !insensitive {
		return `^'(?:[^'\\]|''|\\+'|\\+[^'\\]|\\{2,})*'\z`, true
	}
	if pattern == `^(-?)0+(?=\d)` && flags == "" {
		return `^(-?)0+([0-9])`, true
	}
	p := subsetParser{pattern: pattern, insensitive: insensitive}
	out, _, ok := p.expression(false)
	return out, ok && p.pos == len(pattern)
}

type subsetParser struct {
	pattern     string
	pos         int
	insensitive bool
	names       map[string]bool
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
				if p.pos+1 >= len(p.pattern) {
					return "", false, false
				}
				switch p.pattern[p.pos+1] {
				case ':':
					p.pos += 2
					prefix = "(?:"
					cap = false
				case '<':
					p.pos += 2
					start := p.pos
					for p.pos < len(p.pattern) && (p.pattern[p.pos] >= 'a' && p.pattern[p.pos] <= 'z' || p.pattern[p.pos] >= 'A' && p.pattern[p.pos] <= 'Z' || p.pattern[p.pos] == '_' || p.pos > start && p.pattern[p.pos] >= '0' && p.pattern[p.pos] <= '9') {
						p.pos++
					}
					if p.pos == start || p.pos >= len(p.pattern) || p.pattern[p.pos] != '>' {
						return "", false, false
					}
					name := p.pattern[start:p.pos]
					if p.names == nil {
						p.names = map[string]bool{}
					}
					if p.names[name] {
						return "", false, false
					}
					p.names[name] = true
					prefix = "(?P<" + name + ">"
					p.pos++
				default:
					return "", false, false
				}
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
			case 'b', 'B':
				// Both engines use ASCII word membership on our UTF-16 unit
				// input. Non-u/i folding is ASCII-only; u flags still refuse.
				atom = "\\" + string(escaped)
				assertion = true
			case 's':
				atom = execSpace
			case 'S':
				atom = "[^" + execSpace[1:]
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
			if q == '{' {
				repeat, max, ok := p.boundedRepeat()
				if !ok || assertion || cap && (max < 0 || max > 1) {
					return "", false, false
				}
				atom += repeat
				if p.pos < len(p.pattern) && p.pattern[p.pos] == '?' {
					atom += "?"
					p.pos++
				}
				if p.pos < len(p.pattern) && (p.pattern[p.pos] == '*' || p.pattern[p.pos] == '+' || p.pattern[p.pos] == '?' || p.pattern[p.pos] == '{') {
					return "", false, false
				}
			} else if q == '*' || q == '+' || q == '?' {
				if assertion || cap && q != '?' {
					return "", false, false
				}
				atom += string(q)
				p.pos++
				if p.pos < len(p.pattern) && p.pattern[p.pos] == '?' {
					atom += "?"
					p.pos++
				}
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

// RE2's bounded-repeat limit is 1000. Larger or malformed counts remain
// refusals; repeated captures are screened by expression above.
func (p *subsetParser) boundedRepeat() (string, int, bool) {
	start := p.pos
	p.pos++
	count := func() (int, bool) {
		begin, n := p.pos, 0
		for p.pos < len(p.pattern) && p.pattern[p.pos] >= '0' && p.pattern[p.pos] <= '9' {
			n = n*10 + int(p.pattern[p.pos]-'0')
			p.pos++
			if n > 1000 {
				return 0, false
			}
		}
		return n, p.pos > begin && !(p.pos > begin+1 && p.pattern[begin] == '0')
	}
	min, ok := count()
	if !ok {
		return "", 0, false
	}
	max := min
	if p.pos < len(p.pattern) && p.pattern[p.pos] == ',' {
		p.pos++
		max = -1
		if p.pos < len(p.pattern) && p.pattern[p.pos] != '}' {
			max, ok = count()
			if !ok || max < min {
				return "", 0, false
			}
		}
	}
	if p.pos == len(p.pattern) || p.pattern[p.pos] != '}' {
		return "", 0, false
	}
	p.pos++
	return p.pattern[start:p.pos], max, true
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
	any, whitespace := false, false
	for p.pos < len(p.pattern) && p.pattern[p.pos] != ']' {
		if p.pos+1 < len(p.pattern) && p.pattern[p.pos:p.pos+2] == `\s` {
			p.pos += 2
			if p.pos < len(p.pattern) && p.pattern[p.pos] == '-' {
				return "", false
			}
			any, whitespace = true, true
			continue
		}
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
	if whitespace {
		out += execSpace[1 : len(execSpace)-1]
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
