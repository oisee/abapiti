package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Hash the actual serialized object (metadata as well as ABAP), not only its
// implementation. Length framing makes multi-file object hashes unambiguous.
func writeAbapGitManifest(path, pkg string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	type object struct {
		SHA256 string   `json:"sha256"`
		Files  []string `json:"files"`
	}
	groups := map[string][]*zip.File{}
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, "src/") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(f.Name, "src/"), ".")
		if len(parts) < 3 {
			return fmt.Errorf("invalid abapGit object path %s", f.Name)
		}
		key := strings.ToUpper(parts[0] + "." + parts[1])
		groups[key] = append(groups[key], f)
	}
	objects := map[string]object{}
	for key, files := range groups {
		sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
		h := sha256.New()
		names := []string{}
		for _, f := range files {
			stream, err := f.Open()
			if err != nil {
				return err
			}
			raw, err := io.ReadAll(stream)
			closeErr := stream.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			_, _ = fmt.Fprintf(h, "%d:%s%d:", len(f.Name), f.Name, len(raw))
			_, _ = h.Write(raw)
			names = append(names, f.Name)
		}
		objects[key] = object{hex.EncodeToString(h.Sum(nil)), names}
	}
	raw, err := json.MarshalIndent(struct {
		Package   string            `json:"package"`
		Algorithm string            `json:"algorithm"`
		Objects   map[string]object `json:"objects"`
	}{pkg, "sha256-length-framed-path-and-content-v1", objects}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(strings.TrimSuffix(path, ".zip")+".manifest.json", append(raw, '\n'), 0644)
}
