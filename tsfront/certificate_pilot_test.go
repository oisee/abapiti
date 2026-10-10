package tsfront

import (
	"encoding/json"
	"github.com/oisee/abapiti/tsfront/certificates"
	"github.com/oisee/abapiti/tsfront/overrides"
	"io/fs"
	"testing"
)

func TestCertificateTargetUsesOverrideSpan(t *testing.T) {
	src, spans, err := PilotCertificateSource()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := RegistryOverrides()
	if err != nil {
		t.Fatal(err)
	}
	// This writer is intentionally bound to the exact same declaration as an
	// existing override; whitespace changes cannot broaden the certificate span.
	key := overrides.Key{File: "src/abap/3_structures/structure_parser.ts", Symbol: "StructureParser.runFile", Kind: "KindMethodDeclaration"}
	span, err := src.Span(key)
	if err != nil {
		t.Fatal(err)
	}
	e, ok, err := registry.Lookup(key, span, "pilot")
	if err != nil || !ok {
		t.Fatalf("override key/span drift: %v", err)
	}
	if e.SHA256 != overrides.Fingerprint(spans[key]) {
		t.Fatal("different override fingerprint")
	}
}

func TestCertificatePilotRejectsClaimsOutsideGuards(t *testing.T) {
	for _, mutation := range []string{"none", "cache", "condition", "target", "duplicate", "missing"} {
		t.Run(mutation, func(t *testing.T) {
			f := certificates.Embedded()
			var registry certificates.Registry
			raw, err := fs.ReadFile(f, "registry-certificates.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &registry); err != nil {
				t.Fatal(err)
			}
			loaded := &certificates.Loaded{}
			for _, path := range registry.Certificates {
				var c certificates.Certificate
				raw, err := fs.ReadFile(f, path)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, &c); err != nil {
					t.Fatal(err)
				}
				loaded.Certificates = append(loaded.Certificates, c)
			}
			switch mutation {
			case "cache":
				loaded.Certificates[0].Claim.Cache = "Combi.release"
			case "condition":
				loaded.Certificates[0].Claim.Preconditions = nil
			case "target":
				loaded.Certificates[0].Target.Key.Symbol = "StructureParser.run"
			case "duplicate":
				loaded.Certificates[1] = loaded.Certificates[0]
			case "missing":
				loaded.Certificates = loaded.Certificates[:3]
			}
			err = validatePilotClaims(loaded)
			if mutation == "none" && err != nil {
				t.Fatal(err)
			}
			if mutation != "none" && err == nil {
				t.Fatal("unguarded claim accepted")
			}
		})
	}
}
