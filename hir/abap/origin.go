package abap

import "strings"

// originComment is the first line of a class or interface translated from
// TypeScript: its source position and TS name, so a reader on the system
// sees where it comes from without names.json. A full-line comment, so it
// cannot swallow code; short, so the line wrapper never splits it.
func originComment(id, source string) string {
	src := relSource(source)
	if src == "" {
		return ""
	}
	name := id
	if i := strings.Index(id, ".ts."); i >= 0 {
		name = id[i+4:]
	}
	line := "* TS: " + src + " " + name
	if len(line) > 200 {
		line = line[:200]
	}
	return line + "\n"
}
