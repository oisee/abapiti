package certificates

import (
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/oisee/abapiti/tsfront/overrides"
)

func fixture(t *testing.T) (fstest.MapFS, Source) {
	t.Helper()
	key := overrides.Key{File: "a.ts", Symbol: "A.m", Kind: "KindMethodDeclaration"}
	b := Binding{Key: key, SHA256: overrides.Fingerprint("exact declaration")}
	c := Certificate{Schema: 1, ID: "test", Status: "accepted", Target: b, Dependencies: []Binding{b}, ReceiverSet: []string{"A"}, ReceiverSHA256: DigestReceivers([]string{"A"}), UpstreamSHA256: "pin", Claim: Claim{Predicate: Predicate, Cache: "A.cache", Region: "structure-files", Preconditions: []string{"warm and frozen"}}, Rationale: "narrow", Monitor: "structures-cache-prestore-v1", Hosts: []string{Host}, Optimizations: []string{"warmup-freeze"}}
	f := fstest.MapFS{}
	put := func(path string, v any) Evidence {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		f[path] = &fstest.MapFile{Data: raw}
		return Evidence{Path: path, SHA256: overrides.Fingerprint(string(raw))}
	}
	coverage := map[string]string{}
	for _, k := range []string{"reads", "writes", "aliases", "escapes", "exceptions", "init", "identity", "virtual_targets"} {
		coverage[k] = "covered"
	}
	for _, auditor := range []string{"sol", "glm"} {
		c.Attestations = append(c.Attestations, put(auditor+".json", Attestation{Auditor: auditor, Verdict: "YES", Claim: c.Claim, BindingSHA256: BindingDigest(c), Coverage: coverage, Failure: "late store", Monitor: "pre-store", NegativeTest: "force miss"}))
	}
	c.Validation = put("validation.json", Validation{Schema: 1, UpstreamSHA256: "pin", MonitorSHA256: "host", ABAPDiff: "PASS", Negatives: map[string]string{"StructureParser.singletons": "PASS", "Alternative.map": "PASS", "SubStructure.matcher": "PASS", "sub.singletons": "PASS"}, Differentials: map[string]string{"clean": "PASS", "seeded": "PASS", "abapgit-src": "PASS"}})
	put("test.json", c)
	put("registry-certificates.json", Registry{Schema: 1, Certificates: []string{"test.json"}})
	return f, Source{Span: func(overrides.Key) (string, error) { return "exact declaration", nil }, Receivers: func() ([]string, error) { return []string{"A"}, nil }, UpstreamSHA256: "pin", MonitorSHA256: "host"}
}
func TestLoaderRefusalsAndProvenance(t *testing.T) {
	for _, mode := range []string{"accepted", "span", "receivers", "upstream", "revoked", "unknown", "claim", "duplicate-auditor", "evidence", "dependency", "schema", "missing-target", "monitor", "validation"} {
		t.Run(mode, func(t *testing.T) {
			f, s := fixture(t)
			var c Certificate
			if err := json.Unmarshal(f["test.json"].Data, &c); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "monitor":
				s.MonitorSHA256 = "changed"
			case "validation":
				f["validation.json"].Data = []byte(`{"schema":1}`)
				c.Validation.SHA256 = overrides.Fingerprint(string(f["validation.json"].Data))
			case "span":
				s.Span = func(overrides.Key) (string, error) { return "exact declaration ", nil }
			case "missing-target":
				s.Span = func(overrides.Key) (string, error) { return "", nil }
			case "receivers":
				s.Receivers = func() ([]string, error) { return []string{"A", "B"}, nil }
			case "upstream":
				s.UpstreamSHA256 = "new pin"
			case "revoked":
				f["registry-certificates.json"].Data = []byte(`{"schema":1,"certificates":["test.json"],"tombstones":["test"]}`)
			case "schema":
				c.Schema = 2
			case "dependency":
				c.Dependencies[0].SHA256 = overrides.Fingerprint("changed")
			case "evidence":
				f["sol.json"].Data = append(f["sol.json"].Data, ' ')
			case "unknown", "claim", "duplicate-auditor":
				var a Attestation
				if err := json.Unmarshal(f["glm.json"].Data, &a); err != nil {
					t.Fatal(err)
				}
				if mode == "unknown" {
					a.Verdict = "UNKNOWN"
				}
				if mode == "claim" {
					a.Claim.Cache = "whole-loop"
				}
				if mode == "duplicate-auditor" {
					a.Auditor = "sol"
				}
				raw, _ := json.Marshal(a)
				f["glm.json"].Data = raw
				c.Attestations[1].SHA256 = overrides.Fingerprint(string(raw))
			}
			raw, _ := json.Marshal(c)
			f["test.json"].Data = raw
			l, err := Load(f, s)
			if mode != "accepted" {
				if err == nil {
					t.Fatal("unsafe evidence accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			db, err := l.Axioms()
			if err != nil {
				t.Fatal(err)
			}
			if !db.Has("cert_cache_stable", "structure-files", "A.cache", "test", "a.ts", "A.m", c.Target.SHA256) {
				t.Fatal("missing source provenance")
			}
		})
	}
}
