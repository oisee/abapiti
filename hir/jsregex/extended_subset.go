package jsregex

import "strings"

// TranslateExtendedSubset extends TranslateIteratorSubset with three
// constructs whose JavaScript meaning is fixed and regular over UTF-16 units:
// the dotAll flag (s: "." also matches line terminators), the control
// escapes \t \n \v \f \r, and "^" at the start of a later alternative or
// group (without m it asserts the start of input, exactly RE2's default).
// Each construct is replaced by a printable placeholder unit absent from the
// pattern, certified by the existing grammar as an ordinary literal, and
// substituted back afterwards. Its source is emitted only for programs that
// need it, so earlier runtimes stay byte-identical.
func TranslateExtendedSubset(pattern, flags string) (string, bool) {
	if translated, ok := TranslateIteratorSubset(pattern, flags); ok {
		return translated, true
	}
	dotAll := strings.Contains(flags, "s")
	plain := strings.Replace(flags, "s", "", 1)
	if strings.Contains(plain, "s") {
		return "", false
	}
	free := []byte{}
	for _, c := range []byte("!\"#%&',/:;<=>@`~") {
		if !strings.ContainsRune(pattern, rune(c)) {
			free = append(free, c)
		}
	}
	if len(free) < 6 {
		return "", false
	}
	controls := map[byte]string{'t': `\t`, 'n': `\n`, 'v': `\v`, 'f': `\f`, 'r': `\r`}
	slots := map[byte]string{}
	slotFor := map[string]byte{}
	slot := func(out string) byte {
		if c, ok := slotFor[out]; ok {
			return c
		}
		c := free[len(slotFor)]
		slotFor[out] = c
		slots[c] = out
		return c
	}
	var b strings.Builder
	inClass, changed, anchored := false, false, false
	// branchStart: the next unit begins an alternative (after an unescaped
	// "|", "(" or "(?:" outside a class).
	branchStart := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		wasStart := branchStart
		branchStart = false
		switch {
		case c == '\\' && i+1 < len(pattern):
			if out, ok := controls[pattern[i+1]]; ok {
				// A placeholder cannot be a class range endpoint.
				if inClass && (i > 0 && pattern[i-1] == '-' || i+2 < len(pattern) && pattern[i+2] == '-') {
					return "", false
				}
				b.WriteByte(slot(out))
				changed = true
			} else {
				b.WriteString(pattern[i : i+2])
			}
			i++
		case c == '[' && !inClass:
			inClass = true
			b.WriteByte(c)
		case c == ']' && inClass:
			inClass = false
			b.WriteByte(c)
		case c == '^' && !inClass && wasStart:
			if i+1 < len(pattern) && strings.IndexByte("*+?{", pattern[i+1]) >= 0 {
				return "", false
			}
			b.WriteByte(slot(`\A`))
			changed, anchored = true, true
		case !inClass && (c == '|' || c == '('):
			b.WriteByte(c)
			if c == '(' && i+2 < len(pattern) && pattern[i+1:i+3] == "?:" {
				b.WriteString("?:")
				i += 2
			}
			branchStart = true
		default:
			b.WriteByte(c)
		}
	}
	// The sticky cursor wrapper anchors at lastIndex, not at input start.
	if !changed && !dotAll || anchored && strings.Contains(plain, "y") {
		return "", false
	}
	translated, ok := TranslateIteratorSubset(b.String(), plain)
	if !ok {
		return "", false
	}
	var out strings.Builder
	for i := 0; i < len(translated); i++ {
		if translated[i] == '\\' && i+1 < len(translated) {
			if s, ok := slots[translated[i+1]]; ok {
				out.WriteString(s)
				i++
				continue
			}
			out.WriteString(translated[i : i+2])
			i++
			continue
		}
		if s, ok := slots[translated[i]]; ok {
			out.WriteString(s)
			continue
		}
		out.WriteByte(translated[i])
	}
	if dotAll {
		return strings.ReplaceAll(out.String(), `[^\n\r\x{2028}\x{2029}]`, `(?s:.)`), true
	}
	return out.String(), true
}
