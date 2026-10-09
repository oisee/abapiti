package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/oisee/abapiti/tsfront"
)

// abaplintArchiveURL is the source archive of the pinned commit.
const abaplintArchiveURL = "https://codeload.github.com/abaplint/abaplint/tar.gz/" + tsfront.RegistryUpstreamPin

// abaplintSource is a verified abaplint tree: packages/core/src with the
// closure files, and the npm packages the front end resolves.
type abaplintSource struct {
	Root     string            // repository root (holds packages/core)
	Packages map[string]string // npm package name -> its directory
	Fetched  bool              // downloaded in this run
	Cached   bool              // reused from the cache
	Bytes    int64             // bytes downloaded
	Files    int               // files extracted from the archive
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

// fetchAbaplint returns the verified cached tree at cacheDir, downloading
// the archive and the npm packages when it is missing or does not verify.
func fetchAbaplint(cacheDir string, offline bool, say func(string, ...any)) (*abaplintSource, error) {
	if _, err := os.Stat(cacheDir); err == nil {
		src, verr := verifyAbaplintCheckout(cacheDir)
		if verr == nil {
			src.Cached = true
			return src, nil
		}
		if offline {
			return nil, fmt.Errorf("cached abaplint at %s does not verify: %v", cacheDir, verr)
		}
		say("cache %s does not verify (%v); downloading again", cacheDir, verr)
	} else if offline {
		return nil, fmt.Errorf("--offline: no abaplint checkout given and none cached at %s", cacheDir)
	}
	if err := os.MkdirAll(filepath.Dir(cacheDir), 0755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(cacheDir), filepath.Base(cacheDir)+".partial-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	client := &http.Client{Timeout: 10 * time.Minute}
	var total int64
	start := time.Now()
	n, files, err := download(client, abaplintArchiveURL, func(r io.Reader) (int, error) {
		return extractTarGz(r, stage, func(name string) (string, bool) {
			// <repo>-<sha>/packages/core/{src/**,package.json,package-lock.json}
			_, rest, ok := strings.Cut(name, "/")
			if !ok {
				return "", false
			}
			if strings.HasPrefix(rest, "packages/core/src/") || rest == "packages/core/package.json" || rest == "packages/core/package-lock.json" {
				return rest, true
			}
			return "", false
		})
	})
	if err != nil {
		return nil, err
	}
	total += n
	say("downloaded abaplint %s: %s, %d files of packages/core (%.1fs)", tsfront.RegistryUpstreamPin[:8], mib(n), files, time.Since(start).Seconds())
	lock, err := readLock(filepath.Join(coreDir(stage), "package-lock.json"))
	if err != nil {
		return nil, err
	}
	start = time.Now()
	for _, pkg := range tsfront.RegistryNodePackages() {
		entry, ok := lock.Packages["node_modules/"+pkg.Name]
		if !ok || entry.Version != pkg.Version || entry.Resolved != pkg.Resolved || entry.Integrity != pkg.Integrity {
			return nil, fmt.Errorf("packages/core/package-lock.json of %s does not pin %s@%s (%s)", tsfront.RegistryUpstreamPin[:8], pkg.Name, pkg.Version, pkg.Integrity)
		}
		dir := filepath.Join(coreDir(stage), "node_modules", pkg.Name)
		var data []byte
		n, _, err := download(client, entry.Resolved, func(r io.Reader) (int, error) {
			var err error
			data, err = io.ReadAll(r)
			return 0, err
		})
		if err != nil {
			return nil, err
		}
		total += n
		if err := checkIntegrity(data, entry.Integrity); err != nil {
			return nil, fmt.Errorf("%s: %v", entry.Resolved, err)
		}
		if _, err := extractTarGz(bytes.NewReader(data), dir, func(name string) (string, bool) {
			rest, ok := strings.CutPrefix(name, "package/")
			return rest, ok && rest != ""
		}); err != nil {
			return nil, fmt.Errorf("%s: %v", entry.Resolved, err)
		}
	}
	say("downloaded %d npm packages from package-lock.json, sha512 integrity verified (%.1fs)", len(tsfront.RegistryNodePackages()), time.Since(start).Seconds())
	if _, err := verifyAbaplintCheckout(stage); err != nil {
		return nil, fmt.Errorf("downloaded abaplint: %v", err)
	}
	if err := os.RemoveAll(cacheDir); err != nil {
		return nil, err
	}
	if err := os.Rename(stage, cacheDir); err != nil {
		return nil, err
	}
	src, err := verifyAbaplintCheckout(cacheDir)
	if err != nil {
		return nil, err
	}
	src.Fetched, src.Bytes, src.Files = true, total, files
	return src, nil
}

// download GETs url and hands the body to consume; errors name the URL.
func download(client *http.Client, url string, consume func(io.Reader) (int, error)) (int64, int, error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, 0, fmt.Errorf("download %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("download %s: HTTP %s", url, resp.Status)
	}
	counter := &countingReader{r: resp.Body}
	files, err := consume(counter)
	if err != nil {
		return counter.n, files, fmt.Errorf("download %s: %v", url, err)
	}
	return counter.n, files, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// extractTarGz writes the regular files that keep maps to a relative path.
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

type packageLock struct {
	Packages map[string]struct {
		Version   string `json:"version"`
		Resolved  string `json:"resolved"`
		Integrity string `json:"integrity"`
	} `json:"packages"`
}

func readLock(file string) (*packageLock, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var lock packageLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	return &lock, nil
}

// checkIntegrity verifies a "sha512-<base64>" subresource integrity value.
func checkIntegrity(data []byte, sri string) error {
	want, ok := strings.CutPrefix(sri, "sha512-")
	if !ok {
		return fmt.Errorf("unsupported integrity %q", sri)
	}
	sum := sha512.Sum512(data)
	if got := base64.StdEncoding.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("integrity mismatch: got sha512-%s, lockfile %s", got, sri)
	}
	return nil
}

func mib(n int64) string { return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20)) }
