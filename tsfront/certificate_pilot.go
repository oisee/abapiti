package tsfront

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/oisee/abapiti/hir"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strings"

	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/tsfront/certificates"
	"github.com/oisee/abapiti/tsfront/overrides"
)

//go:embed golang_cert_pilot.go golang_cert_driver.go testdata/certpilot/prestore_test.go.txt
var pilotMonitorSources embed.FS

// PilotMonitorSHA256 expires validation when host guards, quarantine or tests change.
func PilotMonitorSHA256() string {
	var b strings.Builder
	for _, path := range []string{"golang_cert_pilot.go", "golang_cert_driver.go", "testdata/certpilot/prestore_test.go.txt"} {
		raw, err := pilotMonitorSources.ReadFile(path)
		if err != nil {
			panic(err)
		}
		b.WriteString(path)
		b.WriteByte(0)
		b.Write(raw)
		b.WriteByte(0)
	}
	return overrides.Fingerprint(b.String())
}

// LoadPilotCertificates is invoked only by the analysis command and Go pilot.
func LoadPilotCertificates() (*certificates.Loaded, error) {
	src, _, err := PilotCertificateSource()
	if err != nil {
		return nil, err
	}
	src.MonitorSHA256 = PilotMonitorSHA256()
	loaded, err := certificates.Load(certificates.Embedded(), src)
	if err != nil {
		return nil, err
	}
	if err := validatePilotClaims(loaded); err != nil {
		return nil, err
	}
	return loaded, nil
}

func validatePilotClaims(loaded *certificates.Loaded) error {
	targets := map[string]struct{ file, symbol, kind, cache string }{
		"structure-parser-singletons": {"src/abap/3_structures/structure_parser.ts", "StructureParser.runFile", "KindMethodDeclaration", "StructureParser.singletons"},
		"alternative-setup-map":       {"src/abap/3_structures/structures/_combi.ts", "Alternative.setupMap", "KindMethodDeclaration", "Alternative.map"},
		"substructure-setup-matcher":  {"src/abap/3_structures/structures/_combi.ts", "SubStructure.setupMatcher", "KindMethodDeclaration", "SubStructure.matcher"},
		"module-sub-singletons":       {"src/abap/3_structures/structures/_combi.ts", "sub", "KindFunctionDeclaration", "sub.singletons"},
	}
	conditions := []string{"original pinned upstream and closed structures matcher graph", "successful cycle-aware identity walk from Any/ClassGlobal/InterfaceGlobal/DynproLogic before the structures region", "every reachable Alternative.setupMap and SubStructure.setupMatcher completed; module sub and root singletons populated", "no external cache aliases, invalidation, monkey patching or new receiver implementations", "pre-store guard active; failed private attempt quarantined and vanilla rerun from retained raw inputs in fresh process"}
	if loaded == nil || len(loaded.Certificates) != len(targets) {
		return fmt.Errorf("pilot requires exactly four cache certificates")
	}
	seen := map[string]bool{}
	for _, c := range loaded.Certificates {
		want, ok := targets[c.ID]
		if !ok || seen[c.ID] || c.Target.Key.File != want.file || c.Target.Key.Symbol != want.symbol || c.Target.Key.Kind != want.kind || c.Claim.Cache != want.cache || len(c.Claim.Preconditions) != len(conditions) {
			return fmt.Errorf("unsupported pilot claim: %s", c.ID)
		}
		for i, condition := range conditions {
			if c.Claim.Preconditions[i] != condition {
				return fmt.Errorf("pilot precondition changed: %s", c.ID)
			}
		}
		seen[c.ID] = true
	}
	return nil
}

const pilotCertificateRules = `
(rule certified-cache-write 0
 (head (cert_discharge ?loop cache-write ?cache ?id ?file ?symbol ?sha))
 (base (cert_cache_stable ?loop ?cache ?id ?file ?symbol ?sha)))
`

// PilotCertificateReport derives narrow conclusions with certificate IDs carried
// as arguments through Grace. It never turns an axiom into mark_parallel.
func PilotCertificateReport() (string, error) {
	loaded, err := LoadPilotCertificates()
	if err != nil {
		return "", err
	}
	db, err := loaded.Axioms()
	if err != nil {
		return "", err
	}
	_, rules, err := rewrite.Parse(pilotCertificateRules)
	if err != nil {
		return "", err
	}
	if err := rewrite.Evaluate(db, rules); err != nil {
		return "", err
	}
	s := "Certified facts pilot: TS-HG@Go only; conditional on successful guarded warm-up; no parallel execution.\n\n"
	for _, row := range db.Facts("cert_discharge") {
		s += fmt.Sprintf("%s: %s %s rests on %s (%s %s SHA-256 %s)\n", row[0], row[1], row[2], row[3], row[4], row[5], row[6])
	}
	s += "\nStructures map provable: NO. Open: receiver completeness; matcher run effects; input ownership/aliases; file-information effects; initialization timing/identity and exception order; ordered results/issues and lowest-index exception join.\n"
	return s, nil
}

// PilotCertificateUses is the Go names.json provenance channel. Each record
// identifies its declaring member and exact source declaration SHA, never the
// merged member.* identity used by historical names maps.
func PilotCertificateUses(loaded *certificates.Loaded, files map[string]string) (string, error) {
	type use struct {
		CertificateID, Conclusion, Host, File, Symbol, Kind, SHA256, Monitor string
		SourceStart, SourceEnd                                               int
		EmittedFile, EmittedFunction                                         string
		EmittedStart, EmittedEnd                                             int
	}
	src, _, err := PilotCertificateSource()
	if err != nil {
		return "", err
	}
	originals := map[string]string{}
	z, err := gzip.NewReader(bytes.NewReader(EmbeddedAbaplintArchive()))
	if err != nil {
		return "", err
	}
	defer z.Close()
	archive := tar.NewReader(z)
	for {
		h, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(h.Name, "packages/core/src/abap/3_structures/") {
			continue
		}
		b, err := io.ReadAll(archive)
		if err != nil {
			return "", err
		}
		originals[strings.TrimPrefix(h.Name, "packages/core/")] = string(b)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "hir.go", files["hir.go"], 0)
	if err != nil {
		return "", err
	}
	functions := map[string]*ast.FuncDecl{}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			functions[fn.Name.Name] = fn
		}
	}
	n := hir.NewNames()
	var records []use
	for _, c := range loaded.Certificates {
		span, err := src.Span(c.Target.Key)
		if err != nil {
			return "", err
		}
		source := originals[c.Target.Key.File]
		start := strings.Index(source, span)
		if start < 0 || strings.Count(source, span) != 1 {
			return "", fmt.Errorf("ambiguous certificate provenance: %s", c.ID)
		}
		symbol := c.Target.Key.Symbol
		if symbol == "sub" {
			symbol = "module.sub"
		}
		name := n.Get("body." + c.Target.Key.File + "." + symbol)
		fn := functions[name]
		if fn == nil {
			return "", fmt.Errorf("missing emitted certificate span: %s", c.ID)
		}
		records = append(records, use{CertificateID: c.ID, Conclusion: "cache-write discharged after successful guarded warm-up", Host: certificates.Host, File: c.Target.Key.File, Symbol: c.Target.Key.Symbol, Kind: c.Target.Key.Kind, SHA256: c.Target.SHA256, Monitor: c.Monitor, SourceStart: start, SourceEnd: start + len(span), EmittedFile: "hir.go", EmittedFunction: name, EmittedStart: fset.Position(fn.Pos()).Offset, EmittedEnd: fset.Position(fn.End()).Offset})
	}
	b, err := json.MarshalIndent(struct {
		CertificateUses []use `json:"certificate_uses"`
	}{records}, "", "  ")
	return string(b) + "\n", err
}
