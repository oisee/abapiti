package tsfront

import (
	"os"
	"path/filepath"
	"testing"
)

// The static Registry closure contains callable/indexed interfaces. These
// unnamed members must produce blocking diagnostics rather than panic while
// formatting a diagnostic (Name() is nil for all three signature kinds).
func TestRegistryUnnamedInterfaceMembersFailClosed(t *testing.T) {
	for _, signature := range []string{
		"[key: string]: number;",
		"(): number;",
		"new(): object;",
	} {
		t.Run(signature, func(t *testing.T) {
			requireDiagnostic(t, sourceProbe(t, "export interface Indexed { "+signature+" run(): number; }"), "skipped-interface-member")
		})
	}
}

func TestRegistryRecursiveCollectionTypeFailsClosed(t *testing.T) {
	requireDiagnostic(t, sourceProbe(t, `
type Tree = Tree[];
export class Probe { run(value: Tree): number { return value.length; } }
`), "unsupported-type")
}

func TestRegistryComputedMembersInUnionFailClosed(t *testing.T) {
	requireDiagnostic(t, sourceProbe(t, `
export class A { [Symbol.iterator](): number { return 1; } run(): number { return 1; } }
export class B { run(): number { return 2; } }
export class Probe { run(value: A | B): number { return value.run(); } }
`), "skipped-computed-name")
	for _, source := range []string{
		`export interface Data { value: string; (): number; }`,
		`export interface Data { [Symbol.iterator]: number; }`,
	} {
		_, diags := lowerProbe(t, sourceProbe(t, source))
		if len(diags) == 0 {
			t.Fatal("computed/call signature silently disappeared")
		}
	}
}

func TestRegistryDynamicPropertyAssignmentFailsClosed(t *testing.T) {
	requireDiagnostic(t, sourceProbe(t, `
export class Probe { run(value: any): void { value.config = {}; } }
`), "unsupported-assignment")
}

// Critic (PR #50): delete on a Map must not remove the entry; JS keeps it.
func TestRegistryDeleteOnMapFailsClosed(t *testing.T) {
	dir := sourceProbe(t, `
export class Probe {
  run(): boolean {
    const m = new Map<string, number>();
    m.set("x", 1);
    delete m["x"];
    return m.has("x");
  }
}
`)
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"target":"ES2022","strict":true,"noImplicitAny":false},"files":["input.ts"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	requireDiagnostic(t, dir, "unsupported-expr")
}
