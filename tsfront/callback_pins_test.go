package tsfront

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"github.com/oisee/abapiti/tsfront/overrides"
	"io"
	"strings"
	"testing"
)

func TestBatchTwoCallbackSourcePins(t *testing.T) {
	specs := []struct{ file, symbol, kind, start, end string }{
		{"src/rules/identical_conditions.ts", "Conditions", "KindClassDeclaration", "class Conditions {", "\n}"},
		{"src/rules/contains_tab.ts", "ContainsTab.runParsed", "KindMethodDeclaration", "public runParsed(", "\n  }"},
		{"src/abap/5_syntax/statements/catch.ts", "Catch.findCommonSuperclass", "KindMethodDeclaration", "private findCommonSuperclass(", "\n  }"},
	}
	zr, err := gzip.NewReader(bytes.NewReader(EmbeddedAbaplintArchive()))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	sources := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range specs {
			if h.Name == "packages/core/"+s.file {
				raw, err := io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}
				sources[s.file] = string(raw)
			}
		}
	}
	registry, err := RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range specs {
		t.Run(s.symbol, func(t *testing.T) {
			text := sources[s.file]
			start := strings.Index(text, s.start)
			if start < 0 {
				t.Fatal("missing source", s.file)
			}
			end := strings.Index(text[start:], s.end)
			if end < 0 {
				t.Fatal("missing end")
			}
			text = text[start : start+end+len(s.end)]
			key := overrides.Key{File: s.file, Symbol: s.symbol, Kind: s.kind}
			entry, ok, err := registry.Lookup(key, text, s.file)
			if !ok || err != nil {
				t.Fatal(ok, err)
			}
			if entry.Patterns == nil || len(entry.Patterns.DenseCallbacks) == 0 {
				t.Fatal("missing proof")
			}
			for call := range entry.Patterns.DenseCallbacks {
				if !strings.Contains(text, call) {
					t.Fatalf("call not in pinned source: %s", call)
				}
			}
			if _, _, err := registry.Lookup(key, text+" ", s.file); err == nil {
				t.Fatal("stale pin accepted")
			}
		})
	}
}
