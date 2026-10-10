// grace-parallel evaluates a source-audited parallel proof certificate for the
// pinned abaplint closure. It does not lower, rewrite, or execute that program.
package main

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"

	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/hir/rewrite/parallel"
	"github.com/oisee/abapiti/tsfront"
)

//go:embed audit.grace
var audit string

const archiveSHA = "5ea3cd76e2058563ca5311dcc50707e78df5fd04bcb0ce84255ccbd3d0c8be4b"

type candidate struct{ id, loop, conditions, chain, kind string }

var candidates = []candidate{
	{"registry-objects", "Registry.parse / parsePrivate: per object", "none sufficient", "parsePrivate → object.parse → StatementParser.match → Combi.run → writes/reads Combi.release, langVer", "real dependency"},
	{"lexer-files", "ABAPParser.parse: files.map lexer", "closed IFile targets; immutable raw/sets; warm lexer closure; source-order slots; lowest-index exception", "none (receiver graph gap discharged by audited fresh graph)", "—"},
	{"statement-discovery", "StatementParser.run: discovery per file (one object)", "macro pre-pass alone insufficient", "categorize → match → Combi.run → settings stores; macros.find → Macros.addMacro → shared first-definition-wins map", "real dependency"},
	{"statement-expansion", "StatementParser.run: expansion per file (one object)", "frozen macros alone insufficient", "handleMacros → expandContents → StatementParser.run → Combi.run; MacroReferences.addReference → shared appends/readers", "real dependency"},
	{"structure-files", "ABAPParser.parse: structures + file information per file", "ordered join output/issues; recursive matcher warm-up needed", "StructureParser.runFile → singletons[constructor.name] = getMatcher(); matcher.run → Alternative.setupMap / SubStructure.setupMatcher → shared lazy writes", "analysis gap (cache stability, complete warm-up and receiver effects unproved)"},
	{"syntax-objects", "RulesRunner.runRules: syntax per object", "progress/performance need buffering; no sufficient warm-up proved", "SyntaxLogic.run → traverseObject → CurrentScope.findTypePoolType → SyntaxLogic(otherObject).run → otherObject.syntaxResult", "real dependency"},
	{"rule-objects", "RulesRunner.runRules: rules per object", "ordered join issues; rule receivers/timing need separate proof", "rule.run(obj) → UnusedVariables.run → this.workarea = new WorkArea(); rulePerformance[key] = old + runtime", "real dependency"},
}

// The displayed first blocker is required to exist in the evaluated fact set.
// Missing completeness/ownership also blocks these candidates, but the source
// witness is more informative than those conservative fallback diagnostics.
var firstBlocks = map[string]rewrite.Tuple{
	"registry-objects":    {"registry-objects", "Combi::run", "Combi.release"},
	"statement-discovery": {"statement-discovery", "Combi::run", "Combi.release"},
	"statement-expansion": {"statement-expansion", "Combi::run", "Combi.release"},
	"structure-files":     {"structure-files", "StructureParser::runFile", "StructureParser.singletons"},
	"syntax-objects":      {"syntax-objects", "CurrentScope::findTypePoolType", "object.syntaxResult"},
	"rule-objects":        {"rule-objects", "UnusedVariables::run", "UnusedVariables.workarea"},
}

func evidence() (*rewrite.DB, error) {
	if got := fmt.Sprintf("%x", sha256.Sum256(tsfront.EmbeddedAbaplintArchive())); got != archiveSHA {
		return nil, fmt.Errorf("audit source hash changed: %s, want %s; review certificate again", got, archiveSHA)
	}
	db, _, err := rewrite.Parse(audit)
	return db, err
}

// baseline retains every source witness but withholds the new ownership/effect
// certificate for the lexer receiver graph. This is an audit-local comparison,
// not a remeasurement of the older-base HIR classifier.
func baseline(db *rewrite.DB) (*rewrite.DB, error) {
	out := rewrite.NewDB()
	for _, pred := range db.Predicates() {
		for _, row := range db.Facts(pred) {
			if pred == "p_complete" && row[0] == "Lexer::*" {
				continue
			}
			if err := out.Add(pred, row...); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func report() (string, error) {
	base, err := evidence()
	if err != nil {
		return "", err
	}
	old, err := baseline(base)
	if err != nil {
		return "", err
	}
	before, err := parallel.Analyze(old)
	if err != nil {
		return "", err
	}
	after, err := parallel.Analyze(base)
	if err != nil {
		return "", err
	}
	s := fmt.Sprintf("Pinned source archive SHA-256: %s\nSource-audit certificate; conditional proofs, no HIR/runtime changes.\n\n", archiveSHA)
	s += "| Loop | Proven | Preconditions | First blocking chain | Real/gap |\n|---|---|---|---|---|\n"
	for _, c := range candidates {
		if witness, ok := firstBlocks[c.id]; ok && !after.Has("p_block", witness...) {
			return "", fmt.Errorf("displayed blocker lacks a fact witness: %v", witness)
		}
		proven := "no"
		for _, row := range after.Facts("mark_parallel") {
			if row[0] == c.id {
				proven = "yes (conditional)"
			}
		}
		s += fmt.Sprintf("| %s | %s | %s | %s | %s |\n", c.loop, proven, c.conditions, c.chain, c.kind)
	}
	s += "\nClassifier counts (audit-local; legacy HIR classifiers unchanged):\n\n| Relation | Before | After |\n|---|---:|---:|\n"
	for _, pred := range []string{"p_loop", "p_complete", "p_reach", "p_memo_write", "p_counter_write", "p_allowed_write", "p_blocked", "mark_parallel", "parallel_precondition"} {
		s += fmt.Sprintf("| %s | %d | %d |\n", pred, before.Count(pred), after.Count(pred))
	}
	s += "\nDerived obligations (include partial plans on blocked candidates):\n"
	for _, row := range after.Facts("parallel_precondition") {
		s += fmt.Sprintf("- %s: %s %s\n", row[0], row[1], row[2])
	}
	return s, nil
}

func main() {
	s, err := report()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(s)
}
