package tsfront

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"github.com/oisee/abapiti/tsfront/certificates"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// PilotCertificateSource indexes the original embedded upstream, never HIR.
// The whole archive digest expires the pilot on any transitive pin change.
func PilotCertificateSource() (certificates.Source, map[overrides.Key]string, error) {
	z, err := gzip.NewReader(bytes.NewReader(EmbeddedAbaplintArchive()))
	if err != nil {
		return certificates.Source{}, nil, err
	}
	defer z.Close()
	spans := map[overrides.Key]string{}
	var receivers []string
	t := tar.NewReader(z)
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return certificates.Source{}, nil, err
		}
		file := strings.TrimPrefix(h.Name, "packages/core/")
		if !strings.HasPrefix(file, "src/abap/3_structures/") && file != "src/abap/2_statements/combi.ts" {
			continue
		}
		b, err := io.ReadAll(t)
		if err != nil {
			return certificates.Source{}, nil, err
		}
		index, err := CertificateSpans(file, string(b))
		if err != nil {
			return certificates.Source{}, nil, err
		}
		for k, v := range index {
			spans[k] = v
			if strings.HasPrefix(file, "src/abap/3_structures/") && k.Kind == "KindClassDeclaration" {
				receivers = append(receivers, file+"."+k.Symbol)
			}
		}
	}
	src := certificates.Source{UpstreamSHA256: overrides.Fingerprint(string(EmbeddedAbaplintArchive())), Span: func(k overrides.Key) (string, error) {
		v, ok := spans[k]
		if !ok || v == "" {
			return "", fmt.Errorf("missing/ambiguous certificate target: %v", k)
		}
		return v, nil
	}, Receivers: func() ([]string, error) { return append([]string(nil), receivers...), nil }}
	return src, spans, nil
}
