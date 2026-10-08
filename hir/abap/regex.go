package abap

import (
	"regexp"
	"strings"
)

var excludedWords = regexp.MustCompile(`^\^\(\?!\(\?:([A-Za-z0-9_|]+)\)\$\)`)

// regexPattern preserves the matching language while expressing two bounded
// negative assertions without lookahead. Whole-input exclusions remain a
// separate match, so replacing an excluded whole input returns it unchanged.
func regexPattern(pattern string) (string, string) {
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
	return regexClasses(bounded.String()), regexClasses(reject)
}

// Expanding a shorthand inside a class must insert its members, not another
// bracketed class. Escaped brackets do not change the scanner's class state.
func regexClasses(pattern string) string {
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
