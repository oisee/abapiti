package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/inlineoracle"
)

// TestLexerFactsReport shares the exact lowering closure used by the lexer
// differential. GRACE_FACTS_OUT exports a report; goldens remain read-only.
func TestLexerFactsReport(t *testing.T) {
	p, diags := lowerClosure(t)
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") && d.Category != "skipped-computed-name" {
			t.Fatalf("blocking lowering diagnostic: %s", d)
		}
	}
	corpus, err := LoadLexerCorpus(filepath.Join("testdata", "lexercorpus"))
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus) != 44 {
		t.Fatalf("corpus cardinality: %d", len(corpus))
	}
	inlineoracle.Check(t, p)
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	report := rewrite.Report(db)
	if out := os.Getenv("GRACE_FACTS_OUT"); out != "" {
		if err = os.WriteFile(out, []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "lexer.facts.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if report != string(golden) {
		t.Fatal("lexer facts differ from testdata/lexer.facts.golden; export with GRACE_FACTS_OUT and inspect")
	}
	// Spot-check against the vendored TypeScript: accessors read fields, advance
	// writes position state, charCodeAt can raise on ABAP, debugDescription traps.
	checks := []struct {
		pred, method string
		want         bool
	}{
		{"pure", "src/position.ts.Position::getRow", true},
		{"pure", "src/abap/1_lexer/lexer_stream.ts.LexerStream::advance", false},
		{"may_throw", "src/abap/1_lexer/lexer_stream.ts.LexerStream::currentChar", true},
		{"may_throw", "src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken::debugDescription", true},
	}
	for _, c := range checks {
		if db.Has(c.pred, c.method) != c.want {
			t.Errorf("%s(%s) != %v", c.pred, c.method, c.want)
		}
	}
	t.Log("\n" + report[strings.Index(report, "summary\n"):])
}
