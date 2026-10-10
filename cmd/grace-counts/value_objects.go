package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
)

type valueRow struct {
	Class, Source, ABAP, Fields, Bytes, Refs string
	Conditions                               [5]string
	First                                    [5]string
	Definite                                 [5]string
	Optional, Qualifies                      bool
	Total, Loop, Hot, Removed                int
	Stages                                   map[string][2]int
	FieldCount, DynamicNew                   int
	ObjectFields                             string
}

func valueRows(p *hir.Program, db *rewrite.DB, dir string) []valueRow {
	rows := map[string]*valueRow{}
	rel := func(s string) string { return strings.TrimPrefix(s, dir+string(filepath.Separator)) }
	names := hir.NewNames()
	for _, c := range p.Classes {
		if strings.HasSuffix(c.Name, ".module") {
			continue
		}
		r := &valueRow{Class: c.Name, Source: rel(c.Source), ABAP: strings.ToUpper(names.Get(c.Name)), Qualifies: db.Has("value_object", c.Name), Optional: db.Has("vo_absent_flag", c.Name), Stages: map[string][2]int{}}
		for i := range r.Conditions {
			if db.Has("vo_pass", c.Name, strconv.Itoa(i+1)) {
				r.Conditions[i] = "pass"
			} else {
				r.Conditions[i] = "fail"
			}
		}
		rows[c.Name] = r
	}
	for _, f := range db.Facts("vo_field") {
		if r := rows[f[0]]; r != nil {
			if r.Fields != "" {
				r.Fields += "; "
			}
			r.Fields += f[1] + ":" + f[2]
			r.FieldCount++
			if strings.Contains(f[2], "classref") || strings.Contains(f[2], "interfaceref") || strings.Contains(f[2], "array") || strings.Contains(f[2], "map") || strings.Contains(f[2], "set") {
				if r.ObjectFields != "" {
					r.ObjectFields += "; "
				}
				r.ObjectFields += f[1] + ":" + f[2]
			}
		}
	}
	for _, f := range db.Facts("vo_size") {
		if r := rows[f[0]]; r != nil {
			r.Bytes = f[1]
			r.Refs = f[2]
		}
	}
	violations := db.Facts("vo_violation")
	sort.Slice(violations, func(i, j int) bool {
		a, b := violations[i], violations[j]
		ka, kb := sourceOrder(rel(a[3])), sourceOrder(rel(b[3]))
		if ka != kb {
			return ka < kb
		}
		return strings.Join(a, "\x00") < strings.Join(b, "\x00")
	})
	for _, f := range violations {
		if r := rows[f[0]]; r != nil {
			n, _ := strconv.Atoi(f[1])
			if r.First[n-1] == "" {
				r.First[n-1] = rel(f[3]) + " " + f[4] + " [" + f[2] + "]"
			}
			if !uncertainValueReason(f[4]) && r.Definite[n-1] == "" {
				r.Definite[n-1] = rel(f[3]) + " " + f[4] + " [" + f[2] + "]"
			}
		}
	}
	for _, f := range db.Facts("vo_optional") {
		if r := rows[f[0]]; r != nil && r.First[3] == "" {
			r.First[3] = rel(f[2]) + " " + f[3] + " (explicit absent flag required)"
		}
	}
	for _, f := range db.Facts("vo_new") {
		if r := rows[f[0]]; r != nil {
			depth, _ := strconv.Atoi(f[4])
			r.Total++
			if depth > 0 {
				r.Loop++
			}
			st := stage(f[1])
			counts := r.Stages[st]
			if depth > 0 {
				counts[0]++
			} else {
				counts[1]++
			}
			r.Stages[st] = counts
			// This is a static allocation opportunity in parser stages. It is not an
			// execution count, and includes out-of-loop factories called by hot loops.
			if st == "lexer" || st == "statements" || strings.HasPrefix(st, "statements/combi.") || st == "structures" {
				r.Hot++
			}
		}
	}
	for _, f := range db.Facts("vo_dynamic_new") {
		if r := rows[f[0]]; r != nil {
			r.DynamicNew++
		}
	}
	out := []valueRow{}
	for _, r := range rows {
		for i := range r.Conditions {
			if r.Conditions[i] == "fail" && r.Definite[i] == "" {
				r.Conditions[i] = "fail:unproven"
			}
		}
		if r.Qualifies {
			r.Removed = r.Hot
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Removed != out[j].Removed {
			return out[i].Removed > out[j].Removed
		}
		if out[i].Hot != out[j].Hot {
			return out[i].Hot > out[j].Hot
		}
		return out[i].Class < out[j].Class
	})
	return out
}
func sourceOrder(s string) string {
	parts := strings.Split(s, ":")
	if len(parts) >= 3 {
		line, _ := strconv.Atoi(parts[len(parts)-2])
		col, _ := strconv.Atoi(parts[len(parts)-1])
		return fmt.Sprintf("%s:%09d:%09d", strings.Join(parts[:len(parts)-2], ":"), line, col)
	}
	return s
}
func selectedValue(c string) bool {
	for _, suffix := range []string{".Result", ".Position", ".VirtualPosition", ".TokenNode", ".ExpressionNode", ".StatementNode", ".Identifier"} {
		if strings.HasSuffix(c, suffix) {
			return true
		}
	}
	return strings.Contains(c, "src/abap/1_lexer/tokens/") || strings.HasPrefix(c, "src/abap/2_statements/combi.ts.")
}
func writeValueOutputs(out, dir string, p *hir.Program, flow *rewrite.DB, check func(*rewrite.DB) error) error {
	db := rewrite.ExtractValueObjects(p, flow)
	if check != nil {
		if err := check(db); err != nil {
			return err
		}
	}
	_, rules, err := rewrite.Parse(rewrite.ValueObjectRules())
	if err != nil {
		return err
	}
	if err = rewrite.Evaluate(db, rules); err != nil {
		return err
	}
	// Save first witnesses per blocker category and optional type; all allocation sites.
	evidence := map[string][]rewrite.Tuple{}
	for _, pred := range db.Predicates() {
		evidence[pred] = db.Facts(pred)
		for _, r := range evidence[pred] {
			for i, s := range r {
				r[i] = strings.TrimPrefix(s, dir+string(filepath.Separator))
			}
		}
	}
	b, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, "value-facts.json"), b, 0644); err != nil {
		return err
	}
	rows := valueRows(p, db, dir)
	f, err := os.Create(filepath.Join(out, "value-objects.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	header := []string{"class", "abap_name", "source", "c1_immutable", "c1_first", "c2_identity_free", "c2_first", "c3_closed", "c3_first", "c4_nullability", "c4_first", "c5_small", "c5_first", "fields", "inline_bytes_estimate", "reference_slots", "absent_flag", "qualifies", "new_sites", "in_loop", "outside_loop", "parser_hot_sites", "qualifying_hot_sites_removed", "stage_counts_in_out", "field_count", "object_fields_remaining", "dynamic_new_potential", "c1_first_definite", "c2_first_definite", "c3_first_definite", "c5_first_definite"}
	if err = w.Write(header); err != nil {
		f.Close()
		return err
	}
	var report strings.Builder
	report.WriteString("Value-object screen: 1538-file closure + RegistryRun; analysis only\nStatic parser-hot = lexer + statements (combi by caller class) + structures. Counts are sites, not executions.\n\nClass | C1 C2 C3 C4 C5 | bytes/ref | new/loop/hot | eligible hot sites removed | first blocker\n")
	for _, r := range rows {
		stages := []string{}
		for s, n := range r.Stages {
			stages = append(stages, fmt.Sprintf("%s=%d/%d", s, n[0], n[1]))
		}
		sort.Strings(stages)
		data := []string{r.Class, r.ABAP, r.Source}
		for i := range r.Conditions {
			data = append(data, r.Conditions[i], r.First[i])
		}
		data = append(data, r.Fields, r.Bytes, r.Refs, strconv.FormatBool(r.Optional), strconv.FormatBool(r.Qualifies), strconv.Itoa(r.Total), strconv.Itoa(r.Loop), strconv.Itoa(r.Total-r.Loop), strconv.Itoa(r.Hot), strconv.Itoa(r.Removed), strings.Join(stages, "; "))
		data = append(data, strconv.Itoa(r.FieldCount), r.ObjectFields, strconv.Itoa(r.DynamicNew), r.Definite[0], r.Definite[1], r.Definite[2], r.Definite[4])
		if err = w.Write(data); err != nil {
			f.Close()
			return err
		}
		if selectedValue(r.Class) || r.Removed > 0 {
			first := ""
			for _, s := range r.Definite {
				if s != "" {
					first = s
					break
				}
			}
			if first == "" {
				for _, s := range r.First {
					if s != "" {
						first = s
						break
					}
				}
			}
			fmt.Fprintf(&report, "%s | %s | %s/%s | %d/%d/%d | %d | %s\n", r.Class, strings.Join(r.Conditions[:], " "), r.Bytes, r.Refs, r.Total, r.Loop, r.Hot, r.Removed, first)
		}
		if r.Class == "src/abap/2_statements/result.ts.Result" {
			fmt.Fprintf(&report, "RESULT VERDICT: qualifies=%t; C1=%s; %s\n", r.Qualifies, r.Conditions[0], r.Definite[0])
		}
	}
	w.Flush()
	err = w.Error()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	a, err := os.Create(filepath.Join(out, "value-allocations.csv"))
	if err != nil {
		return err
	}
	aw := csv.NewWriter(a)
	aw.Write([]string{"class", "caller", "stage", "site", "source", "loop_depth", "hot_stage"})
	for _, r := range db.Facts("vo_new") {
		st := stage(r[1])
		isHot := st == "lexer" || st == "statements" || strings.HasPrefix(st, "statements/combi.") || st == "structures"
		aw.Write([]string{r[0], r[1], st, r[2], strings.TrimPrefix(r[3], dir+string(filepath.Separator)), r[4], strconv.FormatBool(isHot)})
	}
	aw.Flush()
	err = aw.Error()
	closeErr = a.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.WriteFile(filepath.Join(out, "value-tables.txt"), []byte(report.String()), 0644); err != nil {
		return err
	}
	// Same stable identity algorithm used by abapiti's names.json, without emission.
	mapping := map[string]map[string]string{}
	for _, r := range rows {
		mapping[r.ABAP] = map[string]string{"id": r.Class, "kind": "class", "source": r.Source}
	}
	b, err = json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, "value-names.json"), b, 0644); err != nil {
		return err
	}
	fmt.Print(report.String())
	return nil
}

func uncertainValueReason(reason string) bool {
	return strings.HasPrefix(reason, "potential ") || strings.HasPrefix(reason, "unproven ") || strings.Contains(reason, "may leak/be observed")
}
