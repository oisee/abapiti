package jsregex

// TranslateSubset certifies a small grammar, not arbitrary RE2/PCRE source.
// Non-u JavaScript case folding for ASCII literals is expressed explicitly;
// dot is one UTF-16 unit excluding all four line terminators. Groups, backrefs,
// assertions, Unicode escapes and class folding remain explicit refusals.
func TranslateSubset(pattern, flags string) (string, bool) {
	// Empty RegExp source is canonicalized to (?:) by JS. The current runtime
	// does not retain that form, so refuse rather than expose a guessed source.
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
	out := ""
	atom := false
	quantified := false
	literal := func(c byte) string {
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
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '^':
			if i != 0 {
				return "", false
			}
			out += "^"
			atom = false
		case '$':
			if i != len(pattern)-1 {
				return "", false
			}
			out += "\\z"
			atom = false
		case '*', '+', '?':
			if !atom || quantified {
				return "", false
			}
			out += string(c)
			quantified = true
			continue
		case '.':
			out += `[^\n\r\x{2028}\x{2029}]`
			atom = true
		case '\\':
			i++
			if i == len(pattern) {
				return "", false
			}
			c = pattern[i]
			// Only escaped punctuation is an identity escape in this subset.
			if c < 32 || c > 126 || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
				return "", false
			}
			out += literal(c)
			atom = true
		case '[':
			if insensitive {
				return "", false
			}
			start := i
			i++
			if i < len(pattern) && pattern[i] == '^' {
				i++
			}
			first := i
			for i < len(pattern) && pattern[i] != ']' {
				c = pattern[i]
				if c < 32 || c > 126 || c == '\\' || c == '[' {
					return "", false
				}
				i++
			}
			if i == len(pattern) || i == first {
				return "", false
			}
			out += pattern[start : i+1]
			atom = true
		case '(', ')', '{', '}', '|', ']':
			return "", false
		default:
			if c < 32 || c > 126 {
				return "", false
			}
			out += literal(c)
			atom = true
		}
		quantified = false
	}
	return out, true
}
