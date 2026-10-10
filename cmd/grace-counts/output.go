package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func writeOutputs(out string, sites []site, methods []methodUse, report string) error {
	rows := [][]string{{"caller", "site", "source", "stage", "hot_profile_5", "loop_depth", "argument_index", "form", "verdict", "eligible_receivers", "receiver_count", "receivers", "targets"}}
	for _, s := range sites {
		rows = append(rows, []string{s.Caller, s.Path, s.Source, s.Stage, strconv.FormatBool(s.Hot), strconv.Itoa(s.Loop), strconv.Itoa(s.Arg), s.Form, s.Verdict, strconv.Itoa(s.Yes), strconv.Itoa(len(s.Targets)), strings.Join(s.Receivers, ";"), strings.Join(s.Targets, ";")})
	}
	if err := writeCSV(filepath.Join(out, "sites.csv"), rows); err != nil {
		return err
	}
	rows = [][]string{{"method", "parameter_index", "parameter", "only_iterated", "foreach_uses", "first_blocking_use", "grace_escapes", "grace_alias_edges", "grace_flow_edges"}}
	for _, m := range methods {
		rows = append(rows, []string{m.Method, strconv.Itoa(m.Arg), m.Param, strconv.FormatBool(m.Eligible), strconv.Itoa(m.Loops), m.Blocker, strconv.FormatBool(m.Escapes), strconv.Itoa(m.Aliases), strconv.Itoa(m.Flows)})
	}
	if err := writeCSV(filepath.Join(out, "methods.csv"), rows); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "tables.txt"), []byte(report), 0644)
}
func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	err = w.WriteAll(rows)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func tables(sites []site, methods []methodUse) string {
	type tally struct{ in, out, all, some, none, chain int }
	counts := map[string]tally{}
	total, hotSites, hotAll, hotSome := 0, 0, 0, 0
	for _, stage := range []string{"statements", "lexer", "structures", "syntax", "rules", "other"} {
		counts[stage] = tally{}
	}
	for _, s := range sites {
		t := counts[s.Stage]
		if s.Form == "chained" {
			t.chain++
			counts[s.Stage] = t
			continue
		}
		total++
		if s.Loop > 0 {
			t.in++
		} else {
			t.out++
		}
		switch s.Verdict {
		case "qualifies":
			t.all++
		case "partially":
			t.some++
		default:
			t.none++
		}
		counts[s.Stage] = t
		if s.Hot {
			hotSites++
			if s.Verdict == "qualifies" {
				hotAll++
			}
			if s.Verdict == "partially" {
				hotSome++
			}
		}
	}
	var b strings.Builder
	fmt.Fprintln(&b, "Singleton argument sites (one row per call/parameter; chained iterations excluded from S1)")
	fmt.Fprintln(&b, "Caller stage/class | in-loop | outside | all | some | none | chained")
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t := counts[k]
		marker := ""
		if strings.HasPrefix(k, "statements/combi.") && hot("src/abap/2_statements/combi.ts."+strings.TrimPrefix(k, "statements/combi.")+"::run") {
			marker = "*"
		}
		fmt.Fprintf(&b, "%s%s | %d | %d | %d | %d | %d | %d\n", k, marker, t.in, t.out, t.all, t.some, t.none, t.chain)
	}
	fmt.Fprintf(&b, "Total S1 sites: %d\n", total)
	fmt.Fprintln(&b, "Profile #5 receiver methods (* hot class; all details in methods.csv)")
	fmt.Fprintln(&b, "Receiver method | only iterated | first blocker")
	for _, m := range methods {
		if !strings.HasPrefix(m.Method, "src/abap/2_statements/combi.ts.") || !strings.HasSuffix(m.Method, "::run") {
			continue
		}
		marker := ""
		if hot(m.Method) {
			marker = "*"
		}
		blocker := m.Blocker
		if i := strings.Index(blocker, " at "); i >= 0 {
			blocker = blocker[:i]
		}
		if blocker == "" {
			blocker = fmt.Sprintf("%d foreach uses", m.Loops)
		}
		fmt.Fprintf(&b, "%s%s | %t | %s\n", strings.TrimPrefix(m.Method, "src/abap/2_statements/combi.ts."), marker, m.Eligible, blocker)
	}
	percent := 0.0
	if hotSites > 0 {
		percent = 100 * float64(hotAll) / float64(hotSites)
	}
	fmt.Fprintf(&b, "Hot-class S1 coverage: %d/%d (%.1f%%) all receivers; %d/%d partially covered.\n", hotAll, hotSites, percent, hotSome, hotSites)
	fmt.Fprintln(&b, "Partial sites can use m_one(e)=m([e]) for uncovered receivers. Static site counts are not execution-weighted savings; chained calls have no singleton guarantee.")
	return b.String()
}
