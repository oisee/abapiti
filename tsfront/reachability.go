package tsfront

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// Reachability is a workload-specific, source-pinned coverage union. Missing
// entries are always retained. Positions are UTF-8 byte offsets, not UTF-16.
// CoverageInputs names the current workload, independently of the manifest.
// Supply it whenever the manifest records inputs; validation precedes pruning.
type CoverageInputs struct {
	InputDir, DependenciesDir, ConfigPath, NegativesPath string
}

type Reachability struct {
	CurrentInputs *CoverageInputs `json:"-"`

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
	File      string   `json:"file"`
	Start     int      `json:"start"`
	End       int      `json:"end"`
	Kind      string   `json:"kind"`
	Symbol    string   `json:"symbol"`
	Line      int      `json:"line"`
	SHA256    string   `json:"sha256"`
	Executed  bool     `json:"executed"`
	Workloads []string `json:"workloads,omitempty"`
}

// LowerWithReachability validates coverage before lowering. Schema 1 keeps all
// declarations; schema 2 prunes through a conservative declaration graph.
// Excluded bodies trap before defaults or body code. Reflective replacements
// require a separate fingerprinted override.
func (p *Program) LowerWithReachability(files []string, registry *overrides.Registry, coverage *Reachability) (*hir.Program, []LowerDiagnostic, error) {
	return p.lowerWithPolicy(files, registry, coverage)
}

func (l *lowerer) validateReachability(files []string, coverage *Reachability) error {
	if coverage == nil {
		return nil
	}
	if (coverage.Schema != 1 && coverage.Schema != 2) || len(coverage.Workloads) == 0 {
		return fmt.Errorf("invalid reachability manifest")
	}
	if err := coverage.validateInputs(); err != nil {
		return err
	}
	byFile := map[string][]CoverageSpan{}
	seen := map[string]bool{}
	for _, span := range coverage.Spans {
		key := fmt.Sprintf("%s:%d", span.File, span.Start)
		if seen[key] || span.Start < 0 || span.End <= span.Start || len(span.SHA256) != 64 || strings.HasPrefix(span.File, "../") || filepath.IsAbs(span.File) {
			return fmt.Errorf("invalid coverage span %s", key)
		}
		if coverage.Schema == 2 {
			positive := false
			provenance := map[string]bool{}
			for _, workload := range span.Workloads {
				if (workload != "DEPLOYMENT" && workload != "NEGATIVE" && workload != "UPSTREAM" && workload != "OBSERVATION") || provenance[workload] {
					return fmt.Errorf("invalid coverage provenance at %s", key)
				}
				provenance[workload] = true
				positive = true
			}
			if positive != span.Executed {
				return fmt.Errorf("inconsistent coverage provenance at %s", key)
			}
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
			live := span.Executed
			if coverage.Schema == 2 {
				live = false
				for _, workload := range span.Workloads {
					live = live || workload == "DEPLOYMENT" || workload == "NEGATIVE" || workload == "OBSERVATION"
				}
			}
			if !live {
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
	l.diagf(n, "note-reachability", "excluded coverage body traps at %s", location)
	return true
}

func (r *Reachability) validateInputs() error {
	if len(r.Inputs) == 0 {
		if r.UpstreamPin != "" {
			return fmt.Errorf("coverage workload inputs missing")
		}
		return nil
	}
	if r.CurrentInputs == nil {
		return fmt.Errorf("coverage requires current workload inputs before pruning")
	}
	current := map[string]string{}
	add := func(name, path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("coverage input %s: %w", name, err)
		}
		current[name] = overrides.Fingerprint(string(raw))
		return nil
	}
	for _, root := range []struct{ path, prefix string }{{r.CurrentInputs.InputDir, "input/"}, {r.CurrentInputs.DependenciesDir, "dependencies/"}} {
		if root.path == "" {
			return fmt.Errorf("coverage current input directory missing")
		}
		err := filepath.WalkDir(root.path, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("unsupported coverage input %s", path)
			}
			rel, err := filepath.Rel(root.path, path)
			if err != nil {
				return err
			}
			return add(root.prefix+filepath.ToSlash(rel), path)
		})
		if err != nil {
			return fmt.Errorf("coverage workload inputs: %w", err)
		}
	}
	if err := add("config.json", r.CurrentInputs.ConfigPath); err != nil {
		return err
	}
	if err := add("negative-issues.json", r.CurrentInputs.NegativesPath); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, input := range r.Inputs {
		if seen[input.File] || len(input.SHA256) != 64 || current[input.File] != input.SHA256 {
			return fmt.Errorf("coverage workload input is stale or invalid: %s", input.File)
		}
		seen[input.File] = true
	}
	if len(seen) != len(current) {
		return fmt.Errorf("coverage workload input population changed")
	}
	return nil
}
