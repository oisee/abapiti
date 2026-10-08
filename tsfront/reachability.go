package tsfront

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// Reachability is a workload-specific, source-pinned coverage union. Missing
// entries are always retained. Positions are UTF-8 byte offsets, not UTF-16.
type Reachability struct {
	Schema      int      `json:"schema"`
	UpstreamPin string   `json:"upstreamPin"`
	Workloads   []string `json:"workloads"`
	Inputs      []struct {
		File   string `json:"file"`
		SHA256 string `json:"sha256"`
	} `json:"inputs"`
	Spans []CoverageSpan `json:"spans"`
}
type CoverageSpan struct {
	File     string `json:"file"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Kind     string `json:"kind"`
	Symbol   string `json:"symbol"`
	Line     int    `json:"line"`
	SHA256   string `json:"sha256"`
	Executed bool   `json:"executed"`
}

// LowerWithReachability preserves all declarations and their ABI. Unexecuted
// bodies raise a dedicated exception before evaluating defaults or body code.
// Coverage does not authorize dropping imports or reflective factories.
func (p *Program) LowerWithReachability(files []string, registry *overrides.Registry, coverage *Reachability) (*hir.Program, []LowerDiagnostic, error) {
	return p.lowerWithPolicy(files, registry, coverage)
}

func (l *lowerer) validateReachability(files []string, coverage *Reachability) error {
	if coverage == nil {
		return nil
	}
	if coverage.Schema != 1 || len(coverage.Workloads) == 0 {
		return fmt.Errorf("invalid reachability manifest")
	}
	byFile := map[string][]CoverageSpan{}
	seen := map[string]bool{}
	for _, span := range coverage.Spans {
		key := fmt.Sprintf("%s:%d", span.File, span.Start)
		if seen[key] || span.Start < 0 || span.End <= span.Start || len(span.SHA256) != 64 || strings.HasPrefix(span.File, "../") || filepath.IsAbs(span.File) {
			return fmt.Errorf("invalid coverage span %s", key)
		}
		seen[key] = true
		byFile[span.File] = append(byFile[span.File], span)
	}
	for _, name := range files {
		f, ok := l.prog.File(name)
		if !ok {
			return fmt.Errorf("coverage file %s missing", name)
		}
		rel, err := filepath.Rel(l.prog.configDir, f.FileName())
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		spans := byFile[rel]
		delete(byFile, rel)
		if len(spans) == 0 {
			continue
		}
		nodes := map[int]*ast.Node{}
		var visit func(*ast.Node)
		visit = func(n *ast.Node) {
			if n == nil {
				return
			}
			switch n.Kind {
			case ast.KindMethodDeclaration, ast.KindFunctionDeclaration, ast.KindConstructor, ast.KindGetAccessor, ast.KindSetAccessor:
				nodes[scanner.GetTokenPosOfNode(n, f, false)] = n
			}
			n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
		}
		visit(f.AsNode())
		for _, span := range spans {
			if span.End > len(f.Text()) {
				return fmt.Errorf("coverage is stale at %s:%d", rel, span.Line)
			}
			n := nodes[span.Start]
			line, _ := lineCol(f, span.Start)
			if n == nil || span.Line != line || n.Body() == nil || n.End() != span.End || n.Kind.String() != "Kind"+span.Kind || span.End > len(f.Text()) || overrides.Fingerprint(f.Text()[span.Start:span.End]) != span.SHA256 {
				return fmt.Errorf("coverage is stale at %s:%d", rel, span.Line)
			}
			if !span.Executed {
				l.unexecuted[n] = fmt.Sprintf("%s:%d", rel, span.Line)
			}
		}
	}
	if len(byFile) > 0 {
		return fmt.Errorf("coverage contains files outside selected closure")
	}
	return nil
}

func (l *lowerer) trapUnexecuted(n *ast.Node, hm *hir.Method) bool {
	location, ok := l.unexecuted[n]
	if !ok {
		return false
	}
	hm.Body = hir.B(&hir.Stmt{Node: l.node(n), Kind: hir.Trap, Name: location})
	l.diagf(n, "note-reachability", "unexecuted coverage body traps at %s", location)
	return true
}
