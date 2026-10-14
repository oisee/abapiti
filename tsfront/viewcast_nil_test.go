package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestViewCastCallWithoutLoweredSignature(t *testing.T) {
	dir := t.TempDir()
	for name, source := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"strict":true},"files":["probe.ts","vendor.d.ts"]}`,
		"vendor.d.ts":   `export declare class Vendor { run: any; }`,
		"probe.ts":      `import {Vendor} from "./vendor"; export class Probe { static call(value: Vendor): void { value.run(); } }`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The vendored class is checked but not lowered. Its non-method member
	// creates a view without installing a method signature in methodsBy.
	_, diags, err := p.Lower([]string{"probe.ts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Category == "unsupported-call" && strings.Contains(d.Message, "method run on view view.Vendor has no lowered signature") {
			if !strings.HasPrefix(d.Loc, "probe.ts:1:") || d.Cascade {
				t.Fatalf("expected a located root diagnostic: %+v", d)
			}
			return
		}
	}
	t.Fatalf("missing unsupported-call diagnostic: %+v", diags)
}
