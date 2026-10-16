package jsregex

// IteratorLabelPattern is the Go binding scanner's name-and-colon language.
// The final assertion observes one following unit without consuming it.
const IteratorLabelPattern = `\b([A-Za-z_][A-Za-z_0-9]*)\s*:(?!=)`

// TranslateIteratorSubset extends the existing certified grammar. Unicode
// mode changes the input alphabet to code points; it does not enable Unicode
// case folding or escapes. Named captures are certified as ordinary captures
// first, then their names are restored in the translated expression. Keeping
// the legacy translator separate preserves frozen deployment runtime bytes.
func TranslateIteratorSubset(pattern, flags string) (string, bool) {
	plainFlags := ""
	unicodeMode, insensitive := false, false
	for i := 0; i < len(flags); i++ {
		if flags[i] == 'u' {
			if unicodeMode {
				return "", false
			}
			unicodeMode = true
		} else {
			plainFlags += flags[i : i+1]
		}
		insensitive = insensitive || flags[i] == 'i'
	}
	if unicodeMode && insensitive {
		return "", false
	}
	if _, ok := TranslateSubset("a", plainFlags); !ok {
		return "", false
	}
	if pattern == IteratorLabelPattern {
		prefix, ok := TranslateSubset(pattern[:len(pattern)-5], plainFlags)
		if !ok {
			return "", false
		}
		// The name and whitespace cannot consume a colon. There is one
		// possible endpoint per candidate, so this finite guard preserves
		// JS backtracking. exec hides the wrapper capture and trailing unit.
		return "(" + prefix + ")(?:[^=]|\\z)", true
	}
	if pattern == "" || pattern == "(?:)" {
		return "(?:)", true
	}
	clean := ""
	captures := 0
	names := map[int]string{}
	seen := map[string]bool{}
	inClass, escaped := false, false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if escaped {
			clean += pattern[i : i+1]
			escaped = false
			continue
		}
		if c == '\\' {
			clean += pattern[i : i+1]
			escaped = true
			continue
		}
		if c == '[' {
			inClass = true
		}
		if c == ']' {
			inClass = false
		}
		if c == '(' && !inClass {
			if i+2 < len(pattern) && pattern[i+1:i+3] == "?<" {
				start := i + 3
				end := start
				for end < len(pattern) && (pattern[end] >= 'a' && pattern[end] <= 'z' || pattern[end] >= 'A' && pattern[end] <= 'Z' || pattern[end] == '_' || end > start && pattern[end] >= '0' && pattern[end] <= '9') {
					end++
				}
				if end == start || end >= len(pattern) || pattern[end] != '>' {
					return "", false
				}
				name := pattern[start:end]
				if seen[name] {
					return "", false
				}
				seen[name] = true
				captures++
				names[captures] = name
				clean += "("
				i = end
				continue
			}
			if i+1 >= len(pattern) || pattern[i+1] != '?' {
				captures++
			}
		}
		clean += pattern[i : i+1]
	}
	translated, ok := TranslateSubset(clean, plainFlags)
	if !ok {
		return "", false
	}
	out := ""
	captures = 0
	inClass = false
	escaped = false
	for i := 0; i < len(translated); i++ {
		c := translated[i]
		if escaped {
			out += translated[i : i+1]
			escaped = false
			continue
		}
		if c == '\\' {
			out += translated[i : i+1]
			escaped = true
			continue
		}
		if c == '[' {
			inClass = true
		}
		if c == ']' {
			inClass = false
		}
		if c == '(' && !inClass && (i+1 >= len(translated) || translated[i+1] != '?') {
			captures++
			if name := names[captures]; name != "" {
				out += "(?P<" + name + ">"
				continue
			}
		}
		out += translated[i : i+1]
	}
	return out, true
}
