package abap

import (
	"regexp"
	"strings"
)

// Peepholes over one emitted method body. The emitter declares every temp
// with an executable initialisation (DATA(tN) = VALUE x( ). or DATA tN TYPE
// REF TO x. + CLEAR tN.), and most temps are assigned right after. On the
// kernel each executed statement costs ~50-100 ns, so an initialisation that
// is always overwritten before any read is dropped: the declaration stays,
// as a plain DATA that executes nothing.

var (
	valueInit = regexp.MustCompile(`^DATA\((t\d+)\) = VALUE ([A-Za-z0-9_/]+)\( \)\.$`)
	refDecl   = regexp.MustCompile(`^DATA (t\d+) TYPE REF TO [A-Za-z0-9_/]+\.$`)
	// Lines that change control flow or may leave the straight line.
	blockWord = regexp.MustCompile(`^(IF|ELSEIF|ELSE|ENDIF|WHILE|ENDWHILE|LOOP|ENDLOOP|DO|ENDDO|CASE|WHEN|ENDCASE|TRY|CATCH|CLEANUP|ENDTRY|RETURN|EXIT|CONTINUE|CHECK|RAISE|METHOD|ENDMETHOD)\b`)
)

// foldTempInits drops initialisations of temps that the next statement
// mentioning them overwrites completely, on a straight line with no
// branch, loop, exit or handler in between. Methods with CATCH or CLEANUP
// are left alone: a handler could read a temp an exception left half-set.
func foldTempInits(code string) string {
	if strings.Contains(code, "\nCATCH ") || strings.HasPrefix(code, "CATCH ") || strings.Contains(code, "CLEANUP") {
		return code
	}
	lines := strings.Split(code, "\n")
	drop := map[int]bool{}
	replace := map[int]string{}
	for i, l := range lines {
		var name, decl string
		clear := -1
		if m := valueInit.FindStringSubmatch(l); m != nil {
			name, decl = m[1], "DATA "+m[1]+" TYPE "+m[2]+"."
		} else if m := refDecl.FindStringSubmatch(l); m != nil && i+1 < len(lines) && lines[i+1] == "CLEAR "+m[1]+"." {
			name, clear = m[1], i+1
		} else {
			continue
		}
		start := i + 1
		if clear >= 0 {
			start = clear + 1
		}
		if overwrittenFirst(lines, start, name) {
			if clear >= 0 {
				drop[clear] = true
			} else {
				replace[i] = decl
			}
		}
	}
	if len(drop) == 0 && len(replace) == 0 {
		return code
	}
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		if drop[i] {
			continue
		}
		if r, ok := replace[i]; ok {
			l = r
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// overwrittenFirst: from line start on, the first statement that mentions
// name assigns it completely (name = / ?= / CREATE OBJECT name) without
// reading it, and no line before that leaves the straight line.
func overwrittenFirst(lines []string, start int, name string) bool {
	word := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + name + `($|[^A-Za-z0-9_])`)
	for j := start; j < len(lines); j++ {
		l := lines[j]
		if blockWord.MatchString(l) {
			return false
		}
		if !word.MatchString(l) {
			// a statement continued on the next line (no final period) must
			// not hide a mention: only single-line statements count
			continue
		}
		for _, op := range []string{" = ", " ?= "} {
			if strings.HasPrefix(l, name+op) && strings.HasSuffix(l, ".") {
				rhs := strings.TrimSuffix(strings.TrimPrefix(l, name+op), ".")
				return !word.MatchString(rhs) && (j == start || strings.HasSuffix(lines[j-1], "."))
			}
		}
		if l == "CREATE OBJECT "+name+"." {
			return true
		}
		return false
	}
	return false
}
