package overrides

import (
	"github.com/oisee/abapiti/hir"
	"os"
	"strings"
	"testing"
)

func checkExcludedArtifacts(t *testing.T, name string) {
	t.Helper()
	raw, err := os.ReadFile("../testdata/stmts/src/abap/artifacts.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "public static "+name)
	end := strings.Index(source[start:], "\n  }") + start + 4
	key := Key{"src/abap/artifacts.ts", "ArtifactsABAP." + name, "KindMethodDeclaration"}
	registry := Abaplint()
	entry, ok, err := registry.Lookup(key, source[start:end], "artifacts.ts")
	if !ok || err != nil {
		t.Fatal(ok, err)
	}
	m := entry.Method()
	if m.Name != name || !m.Static || m.Body.List[0].Kind != hir.Throw {
		t.Fatal("excluded path must throw visibly")
	}
	if _, _, err := registry.Lookup(key, source[start:end]+" ", "artifacts.ts"); err == nil {
		t.Fatal("changed span accepted")
	}
}
func TestAbaplintArtifactsStructures(t *testing.T)  { checkExcludedArtifacts(t, "getStructures") }
func TestAbaplintArtifactsExpressions(t *testing.T) { checkExcludedArtifacts(t, "getExpressions") }

func checkExcludedCombi(t *testing.T, class, name string) {
	t.Helper()
	raw, err := os.ReadFile("../testdata/stmts/src/abap/2_statements/combi.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	base := strings.Index(source, "class "+class+" ")
	start := strings.Index(source[base:], "public "+name+"(") + base
	end := strings.Index(source[start:], "\n  }") + start + 4
	key := Key{"src/abap/2_statements/combi.ts", class + "." + name, "KindMethodDeclaration"}
	entry, ok, err := Abaplint().Lookup(key, source[start:end], "combi.ts")
	if !ok || err != nil {
		t.Fatal(ok, err)
	}
	m := entry.Method()
	if !m.Virtual || m.Static || m.Body.List[0].Kind != hir.Throw {
		t.Fatal("excluded path must trap")
	}
	if _, _, err := Abaplint().Lookup(key, source[start:end]+" ", "combi.ts"); err == nil {
		t.Fatal("changed span accepted")
	}
}
func TestAbaplintRegexToStr(t *testing.T)    { checkExcludedCombi(t, "Regex", "toStr") }
func TestAbaplintTokenRailroad(t *testing.T) { checkExcludedCombi(t, "Token", "railroad") }

func TestAbaplintStarPrioritySentinel(t *testing.T) {
	raw, err := os.ReadFile("../testdata/stmts/src/abap/2_statements/combi.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	base := strings.Index(source, "class StarPriority ")
	start := strings.Index(source[base:], "public run(") + base
	end := strings.Index(source[start:], "\n  }") + start + 4
	key := Key{"src/abap/2_statements/combi.ts", "StarPriority.run", "KindMethodDeclaration"}
	entry, ok, err := Abaplint().Lookup(key, source[start:end], "combi.ts")
	if !ok || err != nil {
		t.Fatal(ok, err)
	}
	value := entry.Expressions["Number.MAX_SAFE_INTEGER"]()
	if value.Type.Kind != hir.I32 || value.Value != 2147483647 {
		t.Fatal(value)
	}
	changed := strings.Replace(source[start:end], "best = p.remainingLength()", "best = p.remainingLength() + 1", 1)
	if _, _, err := Abaplint().Lookup(key, changed, "combi.ts"); err == nil {
		t.Fatal("changed sentinel use accepted")
	}
}

func TestRun3Overrides(t *testing.T) {
	cases := []struct{ file, symbol, kind, start, end string }{
		{"src/abap/2_statements/combi.ts", "mapInput", "KindFunctionDeclaration", "function mapInput(", "\n}"},
		{"src/abap/2_statements/expand_macros.ts", "ExpandMacros.find", "KindMethodDeclaration", "public find(", "\n  }"},
		{"src/_iregistry.ts", "IRegistry", "KindInterfaceDeclaration", "export interface IRegistry", "\n}"},
		{"src/_iregistry.ts", "IRunInput", "KindInterfaceDeclaration", "export interface IRunInput", "\n}"},
		{"src/abap/artifacts.ts", "ArtifactsABAP.getKeywords", "KindMethodDeclaration", "public static getKeywords(", "\n  }"},
		{"src/abap/artifacts.ts", "className", "KindFunctionDeclaration", "function className(", "\n}"},
		{"src/abap/1_lexer/tokens/abstract_token.ts", "AbstractToken", "KindMethodDeclaration", "public [Symbol.for(", "\n  }"},
	}
	for _, c := range cases {
		t.Run(c.symbol, func(t *testing.T) {
			raw, err := os.ReadFile("../testdata/stmts/" + c.file)
			if err != nil {
				t.Fatal(err)
			}
			source := string(raw)
			start := strings.Index(source, c.start)
			end := start + strings.Index(source[start:], c.end) + len(c.end)
			key := Key{c.file, c.symbol, c.kind}
			entry, ok, err := Abaplint().Lookup(key, source[start:end], c.file)
			if !ok || err != nil {
				t.Fatal(ok, err)
			}
			if _, _, err := Abaplint().Lookup(key, source[start:end]+" ", c.file); err == nil {
				t.Fatal("stale override accepted")
			}
			switch {
			case entry.Method != nil:
				if entry.Method().Body.List[0].Kind != hir.Throw {
					t.Fatal("excluded method must trap")
				}
			case entry.Statements != nil:
				for _, build := range entry.Statements {
					if build().List[0].Kind != hir.Throw {
						t.Fatal("external include must trap")
					}
				}
			case entry.Expressions != nil:
				if len(entry.Expressions) != 2 {
					t.Fatal("constructor branch must pin both accesses")
				}
			case entry.Interface != nil:
				if len(entry.Interface().Methods) != 1 {
					t.Fatal("registry scope changed")
				}
			case entry.Types != nil:
				if entry.Types["progress"].Name != hir.RootObject {
					t.Fatal("progress reference lost")
				}
			}
		})
	}
}
