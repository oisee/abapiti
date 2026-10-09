package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/tsfront"
)

// abaplintSource is a verified abaplint tree: packages/core/src with the
// closure files, and the npm packages the front end resolves.
type abaplintSource struct {
	Root     string            // repository root (holds packages/core)
	Packages map[string]string // npm package name -> its directory
	Embedded bool              // unpacked from inside abapiti
	Files    int               // files unpacked
}

func coreDir(root string) string { return filepath.Join(root, "packages", "core") }

// verifyAbaplintCheckout checks a checkout against the pinned closure and
// npm package manifests and locates each package's directory.
func verifyAbaplintCheckout(root string) (*abaplintSource, error) {
	core := coreDir(root)
	if _, err := os.Stat(filepath.Join(core, "src", "registry.ts")); err != nil {
		return nil, fmt.Errorf("%s is not an abaplint checkout: %s missing", root, filepath.Join("packages", "core", "src", "registry.ts"))
	}
	if err := tsfront.EmbeddedRegistryClosure().Verify(core); err != nil {
		return nil, fmt.Errorf("abaplint checkout %s does not match the pinned commit %s (packages/core 2.120.56)%s: %v", root, tsfront.RegistryUpstreamPin, describeHead(root), err)
	}
	src := &abaplintSource{Root: root, Packages: map[string]string{}}
	for _, pkg := range tsfront.RegistryNodePackages() {
		var verr error
		for _, dir := range []string{filepath.Join(core, "node_modules", pkg.Name), filepath.Join(root, "node_modules", pkg.Name)} {
			if _, err := os.Stat(dir); err != nil {
				continue
			}
			if verr = pkg.Verify(dir); verr == nil {
				src.Packages[pkg.Name] = dir
				break
			}
		}
		if src.Packages[pkg.Name] == "" {
			if verr == nil {
				verr = fmt.Errorf("npm package %s@%s not installed", pkg.Name, pkg.Version)
			}
			return nil, fmt.Errorf("abaplint checkout %s: %v (run \"npm ci\" in packages/core at %s, or omit the path to let abapiti download it)", root, verr, tsfront.RegistryUpstreamPin[:8])
		}
	}
	return src, nil
}

// describeHead names the checkout's git HEAD when it can be read cheaply.
func describeHead(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(raw))
	if ref, ok := strings.CutPrefix(head, "ref: "); ok {
		if raw, err := os.ReadFile(filepath.Join(root, ".git", filepath.FromSlash(ref))); err == nil {
			head = strings.TrimSpace(string(raw))
		} else {
			head = ref
		}
	}
	if head == tsfront.RegistryUpstreamPin {
		return ""
	}
	return "; its HEAD is " + head
}

// embeddedAbaplint unpacks the abaplint sources and npm type declarations
// that ship inside abapiti into dir and verifies them like a checkout.
func embeddedAbaplint(dir string) (*abaplintSource, error) {
	files, err := extractTarGz(bytes.NewReader(tsfront.EmbeddedAbaplintArchive()), dir, func(name string) (string, bool) { return name, true })
	if err != nil {
		return nil, fmt.Errorf("embedded abaplint: %v", err)
	}
	for _, pkg := range tsfront.RegistryNodePackages() {
		if err := pkg.WriteEmbedded(filepath.Join(coreDir(dir), "node_modules", pkg.Name)); err != nil {
			return nil, err
		}
	}
	src, err := verifyAbaplintCheckout(dir)
	if err != nil {
		return nil, fmt.Errorf("embedded abaplint: %v", err)
	}
	src.Embedded, src.Files = true, files
	return src, nil
}

func extractTarGz(r io.Reader, dst string, keep func(string) (string, bool)) (int, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return 0, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return files, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		rel, ok := keep(h.Name)
		if !ok {
			continue
		}
		clean := path.Clean(rel)
		if clean != rel || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return files, fmt.Errorf("unsafe archive path %q", h.Name)
		}
		target := filepath.Join(dst, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return files, err
		}
		f, err := os.Create(target)
		if err != nil {
			return files, err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return files, err
		}
		if err := f.Close(); err != nil {
			return files, err
		}
		files++
	}
}
