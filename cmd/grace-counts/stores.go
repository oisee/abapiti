package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/hirclone"
)

type storeMeasurement struct {
	Seconds       []float64         `json:"seconds"`
	Stats         rewrite.Stats     `json:"stats"`
	Stages        map[string][2]int `json:"stages"`
	Before, After int
	Hot           map[string][2]int `json:"hot_statements"`
}

func measureStores(p *hir.Program, out string) error {
	// ABAP baseline includes the existing singleton/inliner. Go measurements
	// use the untouched lowered input, matching its pre-emission pipeline.
	result := map[string]storeMeasurement{}
	for _, target := range []string{"go", "abap"} {
		baseline := hirclone.Clone(p)
		var vanilla map[string]string
		var names *hir.Names
		var err error
		if target == "abap" {
			if err = os.Setenv("ABAPITI_COPYPROP", "0"); err != nil {
				return err
			}
			vanilla, names, err = abap.EmitNamed(baseline)
			if err != nil {
				return err
			}
		}
		row := storeMeasurement{Stages: map[string][2]int{}, Hot: map[string][2]int{}}
		var optimized *hir.Program
		for trial := 0; trial < 3; trial++ {
			q := hirclone.Clone(baseline)
			runtime.GC()
			var profile *os.File
			if target == "go" && trial == 0 && os.Getenv("GRACE_COUNTS_PROFILE") != "" {
				profile, err = os.Create(os.Getenv("GRACE_COUNTS_PROFILE"))
				if err != nil {
					return err
				}
				if err = pprof.StartCPUProfile(profile); err != nil {
					_ = profile.Close()
					return err
				}
			}
			start := time.Now()
			stats, e := rewrite.CopyProp(q)
			elapsed := time.Since(start).Seconds()
			if profile != nil {
				pprof.StopCPUProfile()
				if err = profile.Close(); err != nil {
					return err
				}
			}
			if e != nil {
				return e
			}
			fmt.Fprintf(os.Stderr, "%s store trial=%d seconds=%.6f %s\n", target, trial+1, elapsed, rewrite.StoreReport(stats))
			row.Seconds = append(row.Seconds, elapsed)
			row.Stats = stats
			optimized = q
		}
		for method, counts := range row.Stats.Methods {
			st := stage(method)
			if strings.HasPrefix(st, "statements/combi.") {
				st = "statements"
			}
			sum := row.Stages[st]
			sum[0] += counts[0]
			sum[1] += counts[1]
			row.Stages[st] = sum
		}
		if target == "abap" {
			if err = os.Setenv("ABAPITI_INLINE", "0"); err != nil {
				return err
			}
			if err = os.Setenv("ABAPITI_SINGLETON", "0"); err != nil {
				return err
			}
			candidate, _, e := abap.EmitNamed(optimized)
			if e != nil {
				return e
			}
			row.Before = statementTotal(vanilla)
			row.After = statementTotal(candidate)
			for _, c := range baseline.Classes {
				methods := append([]*hir.Method{}, c.Methods...)
				if c.Ctor != nil {
					methods = append(methods, c.Ctor)
				}
				for _, m := range methods {
					id := c.Name + "::" + m.Name
					if !(strings.Contains(c.Name, "/result.ts.Result") || strings.Contains(c.Name, "/combi.ts.") && (m.Name == "run" || m.Name == "run_one") || strings.Contains(c.Name, "/lexer.ts.Lexer") && (m.Name == "process" || m.Name == "add")) {
						continue
					}
					file := names.Get(c.Name) + ".clas.abap"
					method := names.Get("member." + m.Name)
					if m == c.Ctor {
						method = "constructor"
					}
					row.Hot[id] = [2]int{methodStatements(vanilla[file], method), methodStatements(candidate[file], method)}
				}
			}
		}
		result[target] = row
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, "stores.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	for _, target := range []string{"go", "abap"} {
		row := result[target]
		fmt.Printf("%s %s seconds=%v statements=%d->%d\n", target, rewrite.StoreReport(row.Stats), row.Seconds, row.Before, row.After)
		var keys []string
		for key := range row.Stages {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Printf("%s %s copy=%d dse=%d\n", target, key, row.Stages[key][0], row.Stages[key][1])
		}
	}
	return nil
}

// Count ABAP terminators outside literals, templates and comments. This is a
// source statement count, including declarations and structural statements.
func abapStatements(src string) int {
	count := 0
	var quote rune
	comment := false
	lineStart := true
	for _, ch := range src {
		if ch == '\n' {
			comment = false
			lineStart = true
			continue
		}
		if comment {
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if lineStart && ch == '*' {
			comment = true
			continue
		}
		lineStart = false
		if ch == '"' {
			comment = true
			continue
		}
		if ch == '\'' || ch == '`' || ch == '|' {
			quote = ch
			continue
		}
		if ch == '.' {
			count++
		}
	}
	return count
}
func statementTotal(files map[string]string) int {
	n := 0
	for _, src := range files {
		n += abapStatements(src)
	}
	return n
}
func methodStatements(src, name string) int {
	start := strings.Index(src, "METHOD "+name+".")
	if start < 0 {
		return 0
	}
	src = src[start:]
	end := strings.Index(src, "ENDMETHOD.")
	if end < 0 {
		return 0
	}
	return abapStatements(src[:end+len("ENDMETHOD.")])
}
