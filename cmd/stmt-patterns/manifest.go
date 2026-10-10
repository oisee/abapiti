package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/tsfront"
)

func writeManifest(input, output string, files []string, withHIR bool) error {
	h := sha256.New()
	for _, f := range files {
		b, e := os.ReadFile(f)
		if e != nil {
			return e
		}
		h.Write([]byte(filepath.Base(f)))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	n, e := os.ReadFile(filepath.Join(input, "names.json"))
	if e != nil {
		return e
	}
	ns := sha256.Sum256(n)
	record := struct {
		Pin, ClassesSHA256, NamesSHA256 string
		Classes                         int
		HIR                             bool
		InlineMode, SingletonMode       string
	}{tsfront.RegistryUpstreamPin, hex.EncodeToString(h.Sum(nil)), hex.EncodeToString(ns[:]), len(files), withHIR, os.Getenv("ABAPITI_INLINE"), os.Getenv("ABAPITI_SINGLETON")}
	if strings.TrimSpace(record.InlineMode) == "" {
		record.InlineMode = "grace (default)"
	}
	if record.SingletonMode == "" {
		record.SingletonMode = "enabled (default)"
	}
	b, e := json.MarshalIndent(record, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(output, "manifest.json"), append(b, '\n'), 0644)
}
