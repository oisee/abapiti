package main

import (
	"regexp"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront"
)

type gapMessage struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
	Example string `json:"example"`
}
type gapSummary struct {
	Blocking        int            `json:"blocking"`
	RootBlocking    int            `json:"rootBlocking"`
	CascadeBlocking int            `json:"cascadeBlocking"`
	HIRErrors       int            `json:"hirErrors"`
	ByCategory      map[string]int `json:"byCategory"`
	ByArea          map[string]int `json:"byArea"`
	ByRootMessage   []gapMessage   `json:"byRootMessage"`
	Files           int            `json:"files"`
	// PartialFieldReads counts note-partial-field-read: JavaScript reads of a
	// union field that only some members declare (undefined for the others).
	PartialFieldReads int `json:"partialFieldReads,omitempty"`
}

var locationSuffix = regexp.MustCompile(`:[0-9]+:[0-9]+$`)
var quotedGapName = regexp.MustCompile("\"[^\"]*\"|'[^']*'|`[^`]*`")
var namedGapValue = regexp.MustCompile(`\b(identifier|class|interface|method|property|member|symbol|operator|callee) ([A-Za-z_$][A-Za-z0-9_.$/<>-]*)`)
var assignedGapValue = regexp.MustCompile(`\b(assignment to|new|checker type) ([A-Za-z_$][A-Za-z0-9_.$/<>-]*)`)
var referencedGapType = regexp.MustCompile(`(classref|interfaceref|classvalue)<[^>]+>`)
var gapFingerprint = regexp.MustCompile(`sha256=[a-f0-9]+`)

func normalizeGapMessage(s string) string {
	s = quotedGapName.ReplaceAllString(s, "X")
	s = namedGapValue.ReplaceAllString(s, "${1} X")
	s = assignedGapValue.ReplaceAllString(s, "${1} X")
	s = referencedGapType.ReplaceAllString(s, "${1}<X>")
	return gapFingerprint.ReplaceAllString(s, "sha256=X")
}

func gapSourceFile(loc string) string {
	file := locationSuffix.ReplaceAllString(loc, "")
	for _, prefix := range []string{"node_modules/", "src/", "harness/"} {
		if i := strings.Index(file, "/"+prefix); i >= 0 {
			return file[i+1:]
		}
	}
	return file
}

func gapArea(file string) string {
	parts := strings.Split(strings.TrimPrefix(file, "src/"), "/")
	if parts[0] == "node_modules" && len(parts) > 1 {
		if strings.HasPrefix(parts[1], "@") && len(parts) > 2 {
			return strings.Join(parts[:3], "/")
		}
		return strings.Join(parts[:2], "/")
	}
	if parts[0] == "abap" && len(parts) > 1 {
		return strings.Join(parts[:2], "/")
	}
	if len(parts) == 1 {
		return "src"
	}
	return parts[0]
}

// Category/area histograms count all blockers; root messages exclude cascades.
// Files counts distinct source files with blocking or HIR diagnostics.
func summarizeGaps(r *tsfront.RegistryLowering) gapSummary {
	s := gapSummary{Blocking: len(r.Blocking), HIRErrors: len(r.Verification), ByCategory: map[string]int{}, ByArea: map[string]int{}, ByRootMessage: []gapMessage{}}
	files, messages, examples := map[string]bool{}, map[string]int{}, map[string]string{}
	for _, d := range r.Blocking {
		file := gapSourceFile(d.Loc)
		files[file] = true
		s.ByCategory[d.Category]++
		s.ByArea[gapArea(file)]++
		if d.Cascade {
			s.CascadeBlocking++
		} else {
			message := normalizeGapMessage(d.Message)
			messages[message]++
			if examples[message] == "" || d.Loc < examples[message] {
				examples[message] = d.Loc
			}
		}
	}
	for _, err := range r.Verification {
		if e, ok := err.(hir.Error); ok && locationSuffix.MatchString(e.Source) {
			files[gapSourceFile(e.Source)] = true
		}
	}
	for _, d := range r.Diagnostics {
		if d.Category == "note-partial-field-read" {
			s.PartialFieldReads++
		}
	}
	s.Files = len(files)
	s.RootBlocking = s.Blocking - s.CascadeBlocking
	for message, count := range messages {
		s.ByRootMessage = append(s.ByRootMessage, gapMessage{Message: message, Count: count, Example: examples[message]})
	}
	sort.Slice(s.ByRootMessage, func(i, j int) bool {
		a, b := s.ByRootMessage[i], s.ByRootMessage[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Message < b.Message
	})
	if len(s.ByRootMessage) > 30 {
		s.ByRootMessage = s.ByRootMessage[:30]
	}
	return s
}
