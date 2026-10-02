// Package abapsize measures generated ABAP sources before deployment.
package abapsize

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
)

const MaxLineLength = 255

// UNMEASURED: initial warning thresholds until A4H limits are measured.
const (
	DefaultMaxLines        = 70000
	DefaultMaxRoutineLines = 10000
	DefaultMaxRoutines     = 1000
	DefaultMaxBytes        = 8 * 1024 * 1024
)

type Thresholds struct{ Lines, RoutineLines, Routines, Bytes int }

var DefaultThresholds = Thresholds{DefaultMaxLines, DefaultMaxRoutineLines, DefaultMaxRoutines, DefaultMaxBytes}

type File struct {
	Lines, MaxLineLength, LongestRoutine, Routines, Bytes int
}
type LineError struct {
	File         string
	Line, Length int
}

func (e LineError) String() string {
	return fmt.Sprintf("%s:%d:%d: line exceeds %d characters", e.File, e.Line, e.Length, MaxLineLength)
}

type ReportResult struct {
	Files     map[string]File
	LongLines []LineError
}

func Report(files map[string]string) ReportResult {
	r := ReportResult{Files: make(map[string]File, len(files))}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		src := files[name]
		f := File{Bytes: len(src)}
		if src == "" {
			r.Files[name] = f
			continue
		}
		lines := strings.Split(strings.TrimSuffix(src, "\n"), "\n")
		f.Lines = len(lines)
		routine := 0
		for i, line := range lines {
			length := LineLength(strings.TrimSuffix(line, "\r"))
			if length > f.MaxLineLength {
				f.MaxLineLength = length
			}
			if length > MaxLineLength {
				r.LongLines = append(r.LongLines, LineError{name, i + 1, length})
			}
			starts, ends := routineBounds(line)
			if starts {
				f.Routines++
				routine = 1
			} else if routine > 0 {
				routine++
			}
			if routine > f.LongestRoutine {
				f.LongestRoutine = routine
			}
			if ends {
				routine = 0
			}
		}
		r.Files[name] = f
	}
	return r
}

// routineBounds reports whether a line opens a METHOD/FORM and whether it
// closes one, ignoring case and comments; both can hold for a one-line routine.
func routineBounds(line string) (starts, ends bool) {
	if strings.HasPrefix(line, "*") {
		return false, false
	}
	if i := strings.IndexByte(line, '"'); i >= 0 {
		line = line[:i] // approximate: generated code has no '"' inside literals here
	}
	fields := strings.Fields(strings.ToUpper(line))
	if len(fields) == 0 {
		return false, false
	}
	starts = fields[0] == "METHOD" || fields[0] == "FORM"
	for _, f := range fields {
		if f == "ENDMETHOD." || f == "ENDFORM." {
			ends = true
		}
	}
	return starts, ends
}

// LineLength is a line's length as ABAP counts it: UTF-16 code units, so a
// character outside the BMP (such as an emoji) counts twice.
func LineLength(line string) int {
	n := 0
	for _, r := range line {
		n += utf16.RuneLen(r)
	}
	return n
}

func (r ReportResult) Errors() []string {
	out := make([]string, 0, len(r.LongLines))
	for _, e := range r.LongLines {
		out = append(out, e.String())
	}
	return out
}
func (r ReportResult) Warnings(t Thresholds) []string {
	names := make([]string, 0, len(r.Files))
	for name := range r.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []string
	for _, name := range names {
		f := r.Files[name]
		for _, v := range []struct {
			got, limit int
			label      string
		}{{f.Lines, t.Lines, "lines"}, {f.LongestRoutine, t.RoutineLines, "longest METHOD/FORM lines"}, {f.Routines, t.Routines, "METHODs/FORMs"}, {f.Bytes, t.Bytes, "bytes"}} {
			if v.limit > 0 && v.got > v.limit {
				out = append(out, fmt.Sprintf("%s: %s %d exceeds warning threshold %d (UNMEASURED)", name, v.label, v.got, v.limit))
			}
		}
	}
	return out
}
