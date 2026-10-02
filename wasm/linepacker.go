package wasm

import "strings"

// maxLineLen is the ABAP maximum line length.
// Technical limit is 255; we use 240 to leave margin for indentation.
const maxLineLen = 240

// ABAP nesting is independent of whitespace; cap indentation so deep WASM blocks
// cannot consume the source-line budget.
func sourceIndent(depth int) string {
	if depth > 8 {
		depth = 8
	}
	return strings.Repeat("  ", depth)
}

// linePacker collects statements and packs multiple onto one line.
type linePacker struct {
	sb      *strings.Builder
	indent  int
	pending []string // statements waiting to be flushed
	curLen  int      // current packed line length (including indent)
}

func newLinePacker(sb *strings.Builder, indent int) *linePacker {
	return &linePacker{sb: sb, indent: indent}
}

// add adds a statement. If it fits on the current line, pack it. Otherwise flush and start new line.
func (lp *linePacker) add(stmt string) {
	// Statements that MUST be on their own line (control flow)
	if mustOwnLine(stmt) {
		lp.flush()
		lp.writeLine(stmt)
		return
	}

	// Nothing may follow an end-of-line comment: it would become comment text.
	if commentStart(stmt) >= 0 {
		defer lp.flush()
	}

	indentLen := len(sourceIndent(lp.indent))
	stmtLen := len(stmt)

	if len(lp.pending) == 0 {
		// First statement on new line
		if indentLen+stmtLen > maxLineLen {
			// Single statement exceeds limit — write as-is
			lp.writeLine(stmt)
			return
		}
		lp.pending = append(lp.pending, stmt)
		lp.curLen = indentLen + stmtLen
		return
	}

	// Try to append to current line (add 1 for the space separator)
	newLen := lp.curLen + 1 + stmtLen
	if newLen <= maxLineLen {
		lp.pending = append(lp.pending, stmt)
		lp.curLen = newLen
		return
	}

	// Doesn't fit — flush current, start new line
	lp.flush()
	if indentLen+stmtLen > maxLineLen {
		lp.writeLine(stmt)
		return
	}
	lp.pending = append(lp.pending, stmt)
	lp.curLen = indentLen + stmtLen
}

// flush writes all pending statements as one line.
func (lp *linePacker) flush() {
	if len(lp.pending) == 0 {
		return
	}
	prefix := sourceIndent(lp.indent)
	lp.sb.WriteString(prefix)
	lp.sb.WriteString(strings.Join(lp.pending, " "))
	lp.sb.WriteByte('\n')
	lp.pending = lp.pending[:0]
	lp.curLen = 0
}

func (lp *linePacker) writeLine(stmt string) {
	prefix := sourceIndent(lp.indent)
	lp.sb.WriteString(prefix)
	lp.sb.WriteString(stmt)
	lp.sb.WriteByte('\n')
}

func (lp *linePacker) setIndent(indent int) {
	if indent != lp.indent {
		lp.flush()
		lp.indent = indent
	}
}

// mustOwnLine returns true for statements that TRULY need their own line.
// For machine-generated code, only structural boundaries matter.
// IF/ELSE/ENDIF/DO/ENDDO/CASE etc. can all pack onto the same line.
func mustOwnLine(stmt string) bool {
	for _, kw := range []string{
		// Only top-level structural boundaries
		"FORM ", "ENDFORM.",
		"METHOD ", "ENDMETHOD.",
		"CLASS ", "ENDCLASS.",
		"FUNCTION ", "ENDFUNCTION.",
	} {
		if strings.HasPrefix(stmt, kw) {
			return true
		}
	}
	return false
}

// commentStart returns the byte offset where a comment begins in an ABAP source
// line (a `*` in column 1 or a `"` outside a literal), or -1 if there is none.
func commentStart(line string) int {
	if strings.HasPrefix(line, "*") {
		return 0
	}
	var quote byte // the open literal's delimiter, 0 outside literals
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case quote == '|' && ch == '\\':
			i++ // escaped character in a string template
		case quote != 0:
			// A doubled delimiter is an escaped one and keeps the literal open.
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '`' || ch == '|':
			quote = ch
		case ch == '"':
			return i
		}
	}
	return -1
}

// abapLineLimit is the longest source line ADT accepts.
const abapLineLimit = 255

// wrapLongLines splits every line over abapLineLimit at token boundaries.
// ABAP lets a newline stand between any two tokens, so long statements such
// as parameter lists of METHODS, FORM, calls and PERFORM continue on indented
// lines. Literals are never split and a trailing comment stays last.
func wrapLongLines(src string) string {
	if !hasLongLine(src) {
		return src
	}
	lines := strings.SplitAfter(src, "\n")
	var sb strings.Builder
	sb.Grow(len(src) + len(src)/64)
	for _, line := range lines {
		body := strings.TrimSuffix(line, "\n")
		if len(body) <= abapLineLimit {
			sb.WriteString(line)
			continue
		}
		wrapLine(&sb, body)
		if strings.HasSuffix(line, "\n") {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func hasLongLine(src string) bool {
	for len(src) > 0 {
		n := strings.IndexByte(src, '\n')
		if n < 0 {
			n = len(src)
		}
		if n > abapLineLimit {
			return true
		}
		src = src[min(n+1, len(src)):]
	}
	return false
}

// wrapLine writes one over-long line as several, without its final newline.
// Byte length bounds the UTF-16 length ABAP counts, so a fit here is a fit there.
func wrapLine(sb *strings.Builder, line string) {
	comment := commentStart(line)
	if comment == 0 {
		sb.WriteString(line) // full-line comment: nothing to wrap
		return
	}
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	cont := indent + "    "
	// Break candidates: spaces outside literals, up to the comment start.
	end := len(line)
	if comment > 0 {
		end = comment
	}
	var breaks []int
	var quote byte
	for i := len(indent); i < end; i++ {
		ch := line[i]
		switch {
		case quote == '|' && ch == '\\':
			i++
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '`' || ch == '|':
			quote = ch
		case ch == ' ':
			breaks = append(breaks, i)
		}
	}
	start, prefix := len(indent), indent
	for len(prefix)+len(line)-start > maxLineLen {
		// The furthest break that keeps this segment within the target width.
		best := -1
		for _, b := range breaks {
			if b <= start {
				continue
			}
			if len(prefix)+b-start > maxLineLen {
				break
			}
			best = b
		}
		if best < 0 {
			// No break fits: take the next one, the segment stays long.
			for _, b := range breaks {
				if b > start {
					best = b
					break
				}
			}
			if best < 0 {
				break
			}
		}
		sb.WriteString(prefix)
		sb.WriteString(strings.TrimRight(line[start:best], " "))
		sb.WriteByte('\n')
		start = best
		for start < len(line) && line[start] == ' ' {
			start++
		}
		prefix = cont
	}
	sb.WriteString(prefix)
	sb.WriteString(line[start:])
}
