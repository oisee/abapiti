package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireDiagnostic(t *testing.T, dir, category string) {
	t.Helper()
	_, diags := lowerProbe(t, dir)
	for _, d := range diags {
		if d.Category == category {
			if !strings.Contains(d.Loc, "input.ts:") {
				t.Fatalf("missing location: %v", d)
			}
			return
		}
	}
	t.Fatalf("missing %s: %v", category, diags)
}

// Unchanged critic sources: unused initialization and reverse access order
// cannot be implemented by independent lazy ABAP class constructors.
func TestCriticR2Diagnostics(t *testing.T) {
	for name, category := range map[string]string{
		"unused_static": "unsupported-static-init", "static_order": "unsupported-static-init",
		"static_block": "unsupported-member", "surrogate": "unsupported-lone-surrogate",
	} {
		t.Run(name, func(t *testing.T) { requireDiagnostic(t, filepath.Join("testdata", "critic-r2", name), category) })
	}
}

func sourceProbe(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{
		"input.ts":      source,
		"tsconfig.json": `{"compilerOptions":{"target":"ES2022","strict":true},"files":["input.ts"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCriticR2MemberPassesFailClosed(t *testing.T) {
	for _, source := range []string{
		`export class Probe { static {} run(): number { return 0; } }`,
		`export class Probe { get value(): number { return 7; } }`,
	} {
		_, diags := lowerProbe(t, sourceProbe(t, source))
		count := 0
		for _, d := range diags {
			if d.Category == "unsupported-member" {
				count++
			}
		}
		if count != 2 {
			t.Fatalf("both member passes must reject unknown syntax, got %v", diags)
		}
	}
}

func TestCriticR2SurrogateUnits(t *testing.T) {
	for name, literal := range map[string]string{
		"high": `"\uD800"`, "low": `"\uDC00"`,
		"high_braced": `"\u{D800}"`, "low_braced": `"\u{DC00}"`,
		"raw_high": "\"\xed\xa0\x80\"", "raw_low": "\"\xed\xb0\x80\"",
		"template": "`\\uD800`",
	} {
		t.Run(name, func(t *testing.T) {
			dir := sourceProbe(t, "export class Probe { run(): number { const s = "+literal+"; return s.charCodeAt(0); } }")
			requireDiagnostic(t, dir, "unsupported-lone-surrogate")
		})
	}
	for _, literal := range []string{`"\uD83D\uDE00"`, `"\u{1F600}"`, `"\uFFFD"`} {
		t.Run("valid_"+literal, func(t *testing.T) {
			_, diags := lowerProbe(t, sourceProbe(t, "export class Probe { run(): string { return "+literal+"; } }"))
			for _, d := range diags {
				if !strings.HasPrefix(d.Category, "note-") {
					t.Fatal(d)
				}
			}
		})
	}
}

func TestCriticR2StaticPurity(t *testing.T) {
	for name, source := range map[string]string{
		"module_call":       `class Counter { static n: number = 0; static bump(): number { Counter.n = Counter.n + 1; return Counter.n; } } const x = Counter.bump(); export class Probe { run(): number { return Counter.n; } }`,
		"module_class_read": `class Counter { static n: number = 0; } const x = Counter.n;`,
		"cross_class_read":  `class Counter { static n: number = 0; } class Probe { static n: number = Counter.n; }`,
		"forward_read":      `class Probe { static x: number = Probe.y; static y: number = 7; }`,
		"assignment":        `class Probe { static x: number = 0; static y: number = (Probe.x = 7); }`,
		"user_new":          `class Other { static n = 0; constructor() { Other.n++; } } class Probe { static x: Other = new Other(); }`,
		"user_collection":   `class Set<T> { static n = 0; constructor() { Set.n++; } } class Probe { static x: Set<number> = new Set<number>(); }`,
		"user_new_call":     `function f(): number { return 1; } class Other { x: number = f(); } class Probe { static x: Other = new Other(); }`,
	} {
		t.Run(name, func(t *testing.T) { requireDiagnostic(t, sourceProbe(t, source), "unsupported-static-init") })
	}
	for name, source := range map[string]string{
		"constants":   `const a = 3; const b = a + 4; export class Probe { static n: number = b * 2; run(): number { return Probe.n; } }`,
		"same_class":  `export class Probe { static x: number = 7; static y: number = Probe.x + 1; run(): number { return Probe.y; } }`,
		"collection":  `const xs = new Set<number>([1, 2, 3]); export class Probe { run(): boolean { return xs.has(2); } }`,
		"trivial_new": `class Other { readonly x: number; constructor(x: number) { this.x = x; } } export class Probe { static o: Other = new Other(7); static s: Set<string> = new Set<string>(); static u: string | undefined = undefined; run(): number { return Probe.o.x; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			_, diags := lowerProbe(t, sourceProbe(t, source))
			for _, d := range diags {
				if !strings.HasPrefix(d.Category, "note-") {
					t.Fatal(d)
				}
			}
		})
	}
}
