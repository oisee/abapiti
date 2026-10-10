// Package certificates supplies source-bound axioms to the pilot only.
// Neither lowering nor hir/abap imports this package. Attribution is not proof.
package certificates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/tsfront/overrides"
)

const Schema = 1
const Predicate = "cache-stable-after-warmup"
const Host = "TS-HG@Go"

type Binding struct {
	Key    overrides.Key `json:"key"`
	SHA256 string        `json:"sha256"`
}
type Claim struct {
	Predicate     string   `json:"predicate"`
	Cache         string   `json:"cache"`
	Region        string   `json:"region"`
	Preconditions []string `json:"preconditions"`
}
type Attestation struct {
	Auditor       string            `json:"auditor"`
	Verdict       string            `json:"verdict"`
	Claim         Claim             `json:"claim"`
	BindingSHA256 string            `json:"binding_sha256"`
	Coverage      map[string]string `json:"coverage"`
	Failure       string            `json:"failure_scenario"`
	Monitor       string            `json:"monitor"`
	NegativeTest  string            `json:"negative_test"`
}
type Evidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Certificate struct {
	Schema         int        `json:"schema"`
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	Target         Binding    `json:"target"`
	Dependencies   []Binding  `json:"dependencies"`
	ReceiverSet    []string   `json:"receiver_set"`
	ReceiverSHA256 string     `json:"receiver_sha256"`
	UpstreamSHA256 string     `json:"upstream_sha256"`
	Claim          Claim      `json:"claim"`
	Rationale      string     `json:"rationale"`
	Attestations   []Evidence `json:"attestations"`
	Validation     Evidence   `json:"dynamic_validation"`
	Monitor        string     `json:"monitor"`
	Hosts          []string   `json:"hosts"`
	Optimizations  []string   `json:"permitted_optimizations"`
}
type Registry struct {
	Schema       int      `json:"schema"`
	Certificates []string `json:"certificates"`
	Tombstones   []string `json:"tombstones"`
}

// Source resolves exact declaration spans and independently enumerates the
// receiver envelope. The upstream archive binds the entire transitive closure.
type Source struct {
	Span           func(overrides.Key) (string, error)
	Receivers      func() ([]string, error)
	UpstreamSHA256 string
	MonitorSHA256  string
}
type Loaded struct{ Certificates []Certificate }

func decode(f fs.FS, path string, v any) error {
	raw, err := fs.ReadFile(f, path)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s: trailing JSON", path)
	}
	return nil
}
func DigestReceivers(receivers []string) string {
	a := append([]string(nil), receivers...)
	sort.Strings(a)
	return overrides.Fingerprint(strings.Join(a, "\n"))
}
func BindingDigest(c Certificate) string {
	b, _ := json.Marshal(struct {
		Target       Binding
		Dependencies []Binding
		Receivers    string
		Upstream     string
	}{c.Target, c.Dependencies, c.ReceiverSHA256, c.UpstreamSHA256})
	return overrides.Fingerprint(string(b))
}
func sameClaim(a, b Claim) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func evidence(f fs.FS, e Evidence) ([]byte, error) {
	if !fs.ValidPath(e.Path) || len(e.SHA256) != 64 {
		return nil, fmt.Errorf("missing evidence")
	}
	b, err := fs.ReadFile(f, e.Path)
	if err != nil {
		return nil, err
	}
	if overrides.Fingerprint(string(b)) != e.SHA256 {
		return nil, fmt.Errorf("evidence changed: %s", e.Path)
	}
	return b, nil
}

// Load fails closed, including missing/deleted/ambiguous declarations, receiver
// changes, expired upstream pins, revoked records and incomplete audit coverage.
func Load(f fs.FS, src Source) (*Loaded, error) {
	var r Registry
	if err := decode(f, "registry-certificates.json", &r); err != nil {
		return nil, err
	}
	if r.Schema != Schema {
		return nil, fmt.Errorf("unsupported certificate registry schema")
	}
	tomb := map[string]bool{}
	for _, id := range r.Tombstones {
		tomb[id] = true
	}
	out := &Loaded{}
	seen := map[string]bool{}
	for _, path := range r.Certificates {
		var c Certificate
		if err := decode(f, path, &c); err != nil {
			return nil, err
		}
		if seen[c.ID] || c.ID == "" {
			return nil, fmt.Errorf("duplicate/empty certificate: %s", c.ID)
		}
		seen[c.ID] = true
		if tomb[c.ID] || c.Status == "revoked" {
			return nil, fmt.Errorf("certificate revoked: %s", c.ID)
		}
		if c.Schema != Schema || c.Status != "accepted" || c.Claim.Predicate != Predicate || c.Claim.Region != "structure-files" || c.Claim.Cache == "" || len(c.Claim.Preconditions) == 0 || c.Rationale == "" || c.Monitor != "structures-cache-prestore-v1" || len(c.Hosts) != 1 || c.Hosts[0] != Host || len(c.Optimizations) != 1 || c.Optimizations[0] != "warmup-freeze" {
			return nil, fmt.Errorf("certificate not accepted or unsupported: %s", c.ID)
		}
		if c.UpstreamSHA256 != src.UpstreamSHA256 || c.UpstreamSHA256 == "" {
			return nil, fmt.Errorf("certificate upstream pin stale: %s", c.ID)
		}
		if len(c.Dependencies) == 0 || src.Span == nil || src.Receivers == nil {
			return nil, fmt.Errorf("missing dependency closure: %s", c.ID)
		}
		for _, b := range append([]Binding{c.Target}, c.Dependencies...) {
			span, err := src.Span(b.Key)
			if err != nil {
				return nil, err
			}
			if span == "" || len(b.SHA256) != 64 || overrides.Fingerprint(span) != b.SHA256 {
				return nil, fmt.Errorf("certificate %s stale: %v", c.ID, b.Key)
			}
		}
		receivers, err := src.Receivers()
		if err != nil {
			return nil, err
		}
		if len(receivers) == 0 || DigestReceivers(receivers) != c.ReceiverSHA256 || DigestReceivers(c.ReceiverSet) != c.ReceiverSHA256 {
			return nil, fmt.Errorf("certificate receiver envelope stale: %s", c.ID)
		}
		if len(c.Attestations) != 2 {
			return nil, fmt.Errorf("two independent audits required: %s", c.ID)
		}
		auditors := map[string]bool{}
		for _, e := range c.Attestations {
			if _, err := evidence(f, e); err != nil {
				return nil, err
			}
			var a Attestation
			if err := decode(f, e.Path, &a); err != nil {
				return nil, err
			}
			if a.Verdict != "YES" || a.Auditor == "" || auditors[a.Auditor] || !sameClaim(a.Claim, c.Claim) || a.BindingSHA256 != BindingDigest(c) || a.Failure == "" || a.Monitor == "" || a.NegativeTest == "" {
				return nil, fmt.Errorf("audit disagreement/incomplete: %s", c.ID)
			}
			for _, k := range strings.Fields("reads writes aliases escapes exceptions init identity virtual_targets") {
				if a.Coverage[k] == "" {
					return nil, fmt.Errorf("audit missing %s: %s", k, c.ID)
				}
			}
			auditors[a.Auditor] = true
		}
		if _, err := evidence(f, c.Validation); err != nil {
			return nil, err
		}
		var validation Validation
		if err := decode(f, c.Validation.Path, &validation); err != nil {
			return nil, err
		}
		if validation.Schema != Schema || validation.UpstreamSHA256 != src.UpstreamSHA256 || validation.MonitorSHA256 == "" || validation.MonitorSHA256 != src.MonitorSHA256 || validation.ABAPDiff != "PASS" {
			return nil, fmt.Errorf("pilot dynamic validation stale/incomplete: %s", c.ID)
		}
		for _, key := range []string{"StructureParser.singletons", "Alternative.map", "SubStructure.matcher", "sub.singletons"} {
			if validation.Negatives[key] != "PASS" {
				return nil, fmt.Errorf("missing caught pre-store negative: %s", key)
			}
		}
		for _, key := range []string{"clean", "seeded", "abapgit-src"} {
			if validation.Differentials[key] != "PASS" {
				return nil, fmt.Errorf("pilot differential not green: %s", key)
			}
		}
		out.Certificates = append(out.Certificates, c)
	}
	return out, nil
}

// Axioms are namespaced, provenance-bearing base facts. They never claim a
// complete receiver/effect summary or an ownership proof for the whole loop.
func (l *Loaded) Axioms() (*rewrite.DB, error) {
	db := rewrite.NewDB()
	for _, c := range l.Certificates {
		if err := db.Add("cert_cache_stable", c.Claim.Region, c.Claim.Cache, c.ID, c.Target.Key.File, c.Target.Key.Symbol, c.Target.SHA256); err != nil {
			return nil, err
		}
	}
	return db, nil
}

// Validation is a source-and-monitor-bound dynamic gate, not an arbitrary blob.
type Validation struct {
	Schema         int               `json:"schema"`
	UpstreamSHA256 string            `json:"upstream_sha256"`
	MonitorSHA256  string            `json:"monitor_sha256"`
	Negatives      map[string]string `json:"prestore_negatives"`
	Differentials  map[string]string `json:"differentials"`
	ABAPDiff       string            `json:"abap_diff"`
}
