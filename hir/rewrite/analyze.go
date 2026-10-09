package rewrite

import (
	"embed"
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

//go:embed rules/*.grace
var ruleFiles embed.FS

// Analyze verifies the program and computes the embedded milestone 1 rules.
func Analyze(p *hir.Program) (*DB, error) {
	if errs := hir.Verify(p); len(errs) > 0 {
		return nil, fmt.Errorf("invalid HIR: %v", errs)
	}
	db := Extract(p)
	entries, err := ruleFiles.ReadDir("rules")
	if err != nil {
		return nil, err
	}
	var source strings.Builder
	for _, e := range entries {
		b, err := ruleFiles.ReadFile("rules/" + e.Name())
		if err != nil {
			return nil, err
		}
		source.Write(b)
		source.WriteByte('\n')
	}
	_, rules, err := Parse(source.String())
	if err != nil {
		return nil, err
	}
	if err = Evaluate(db, rules); err != nil {
		return nil, err
	}
	return db, nil
}
