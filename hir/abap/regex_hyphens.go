package abap

import (
	"strings"
	"unicode/utf8"
)

// Class ranges are recognized BEFORE expanding JS shorthands. In non-u JS,
// a set escape at either endpoint makes the intervening '-' literal (Annex B).
// A '-' after a consumed range is also literal. Put literal hyphens first so
// neither POSIX nor PCRE can reinterpret them as ranges after expansion.
func regexClassHyphens(pattern string) string {
	var out strings.Builder
	for i := 0; i < len(pattern); {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			out.WriteString(pattern[i : i+2])
			i += 2
			continue
		}
		if pattern[i] != '[' {
			out.WriteByte(pattern[i])
			i++
			continue
		}
		start := i
		i++
		negated := i < len(pattern) && pattern[i] == '^'
		if negated {
			i++
		}
		type atom struct {
			text string
			set  bool
		}
		var atoms []atom
		for i < len(pattern) && pattern[i] != ']' {
			_, width := utf8.DecodeRuneInString(pattern[i:])
			end := i + width
			set := false
			if pattern[i] == '\\' && end < len(pattern) {
				c := pattern[end]
				end++
				set = strings.ContainsRune("dDsSwW", rune(c))
				if c == 'x' && end+2 <= len(pattern) {
					end += 2
				} else if c == 'u' && end+4 <= len(pattern) {
					end += 4
				}
			}
			atoms = append(atoms, atom{pattern[i:end], set})
			i = end
		}
		if i == len(pattern) {
			out.WriteString(pattern[start:])
			break
		}
		var body strings.Builder
		hyphen := false
		for j := 0; j < len(atoms); {
			a := atoms[j]
			if j+2 < len(atoms) && atoms[j+1].text == "-" && !a.set && !atoms[j+2].set {
				body.WriteString(a.text + "-" + atoms[j+2].text)
				j += 3
			} else {
				if a.text == "-" || a.text == `\-` {
					hyphen = true
				} else {
					body.WriteString(a.text)
				}
				j++
			}
		}
		out.WriteByte('[')
		if negated {
			out.WriteByte('^')
		}
		if hyphen && !strings.HasPrefix(body.String(), "-") {
			out.WriteByte('-')
		}
		out.WriteString(body.String())
		if hyphen && strings.HasPrefix(body.String(), "-") {
			out.WriteByte('-')
		}
		out.WriteByte(']')
		i++
	}
	return out.String()
}
