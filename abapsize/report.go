// Package abapsize measures generated ABAP sources before deployment.
package abapsize

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
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
			length := utf8.RuneCountInString(strings.TrimSuffix(line, "\r"))
			if length > f.MaxLineLength {
				f.MaxLineLength = length
			}
			if length > MaxLineLength {
				r.LongLines = append(r.LongLines, LineError{name, i + 1, length})
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "METHOD ") || strings.HasPrefix(trimmed, "FORM ") {
				f.Routines++
				routine = 1
			} else if routine > 0 {
				routine++
			}
			if routine > f.LongestRoutine {
				f.LongestRoutine = routine
			}
			if trimmed == "ENDMETHOD." || trimmed == "ENDFORM." {
				routine = 0
			}
		}
		r.Files[name] = f
	}
	return r
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
