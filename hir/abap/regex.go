package abap

import (
	"regexp"
	"strings"
)

var excludedWords = regexp.MustCompile(`^\^\(\?!\(\?:([A-Za-z0-9_|]+)\)\$\)`)

// regexPattern preserves the matching language while expressing two bounded
// negative assertions without lookahead. Whole-input exclusions remain a
// separate match, so replacing an excluded whole input returns it unchanged.
func regexPattern(pattern string, flags ...string) (string, string) {
	reject := ""
	if m := excludedWords.FindStringSubmatchIndex(pattern); m != nil && strings.HasSuffix(pattern, "$") {
		reject = "^(?:" + pattern[m[2]:m[3]] + ")$"
		pattern = "^" + pattern[m[1]:]
	}
	var bounded strings.Builder
	for i := 0; i < len(pattern); {
		// (?!\*)[...\*...]+ is exactly [without *][original]*.
		if strings.HasPrefix(pattern[i:], `(?!\`) && i+7 < len(pattern) && pattern[i+5] == ')' && pattern[i+6] == '[' {
			excluded := pattern[i+4]
			j := i + 7
			for j < len(pattern) {
				if pattern[j] == '\\' {
					j += 2
					continue
				}
				if pattern[j] == ']' {
					break
				}
				j++
			}
			if j+1 < len(pattern) && pattern[j+1] == '+' {
				body := pattern[i+7 : j]
				token := "\\" + string(excluded)
				if strings.ContainsRune(`*+?.$%~/#`, rune(excluded)) && !strings.HasPrefix(body, "^") && strings.Contains(body, token) {
					first := strings.ReplaceAll(body, token, "")
					if first != "" {
						bounded.WriteString("[" + first + "][" + body + "]*")
						i = j + 2
						continue
					}
				}
			}
		}
		bounded.WriteByte(pattern[i])
		i++
	}
	return regexClasses(bounded.String(), flags...), regexClasses(reject, flags...)
}

// Literal members avoid hex escapes, which the pinned POSIX host refuses.
const jsWhitespaceMembers = "\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"
const jsDotClass = "[^\n\r\u2028\u2029]"

// Expanding a shorthand inside a class must insert its members, not another
// bracketed class. Escaped brackets do not change the scanner's class state.
// Unrepresentable class complements are checked before emission/construction.
func regexClasses(pattern string, flags ...string) string {
	pattern = regexClassHyphens(pattern)
	dotAll := len(flags) > 0 && strings.Contains(flags[0], "s")
	var b strings.Builder
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' && i+1 < len(pattern) {
			i++
			c = pattern[i]
			members := ""
			switch c {
			case 'w':
				members = "A-Za-z0-9_"
			case 'd':
				members = "0-9"
			case 's':
				members = jsWhitespaceMembers
			case 'S':
				if !inClass {
					members = "^" + jsWhitespaceMembers
				}
			}
			if members != "" {
				if !inClass {
					b.WriteByte('[')
				}
				b.WriteString(members)
				if !inClass {
					b.WriteByte(']')
				}
			} else {
				b.WriteByte('\\')
				b.WriteByte(c)
			}
		} else {
			if c == '.' && !inClass && !dotAll {
				b.WriteString(jsDotClass)
				continue
			}
			if c == '[' {
				inClass = true
			} else if c == ']' {
				inClass = false
			}
			b.WriteByte(c)
		}
	}
	return b.String()
}

// A complemented shorthand cannot be inserted as members of a POSIX class.
// Static emission refuses it; dynamic construction raises rather than guessing.
func regexHasClassComplement(pattern string) bool {
	pattern = strings.ReplaceAll(strings.ReplaceAll(pattern, `[\s\S]`, "[X]"), `[\S\s]`, "[X]")
	return regexHasEscape(pattern, "S", true)
}

func regexHasEscape(pattern, escapes string, inside bool) bool {
	inClass := false
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			i++
			if inClass == inside && strings.ContainsRune(escapes, rune(pattern[i])) {
				return true
			}
		} else if pattern[i] == '[' {
			inClass = true
		} else if pattern[i] == ']' {
			inClass = false
		}
	}
	return false
}
