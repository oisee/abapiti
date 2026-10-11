// gogen-spike measures full, trap-on-failure lowering; it is not a compiler driver.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/hir/golang"
	"github.com/oisee/abapiti/tsfront"
)

type result struct {
	Files                                       []string
	Checker                                     []tsfront.Diagnostic
	Lower                                       []tsfront.LowerDiagnostic
	LowerError                                  string
	RawLower                                    []tsfront.LowerDiagnostic
	RawError                                    string
	RawVerify                                   []string
	Verify                                      []string
	Classes, Interfaces                         int
	GoFiles, ABAPFiles                          int
	GoError, ABAPError, BuildError, BuildOutput string
}

func attempt(f func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return f()
}
func message(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func save(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
func emit(dir string, f func() (map[string]string, error)) (int, string) {
	count := 0
	err := attempt(func() error {
		files, err := f()
		if err != nil {
			return err
		}
		count = len(files)
		for name, text := range files {
			if err := save(filepath.Join(dir, name), []byte(text)); err != nil {
				return err
			}
		}
		return nil
	})
	return count, message(err)
}
func main() {
	config := flag.String("config", "", "scratch tsconfig")
	out := flag.String("out", "", "fresh output directory")
	js := flag.Bool("js", false, "also lower the resolved project JS closure (not node_modules)")
	flag.Parse()
	if *config == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if _, err := os.Stat(*out); !os.IsNotExist(err) {
		panic("output must not exist")
	}
	p, err := tsfront.Load(*config)
	if err != nil {
		panic(err)
	}
	root, _ := filepath.Abs(filepath.Dir(*config))
	r := result{}
	for _, f := range p.SourceFiles() {
		if !strings.HasPrefix(f, root+string(filepath.Separator)) || strings.Contains(f, "/node_modules/") || strings.HasSuffix(f, ".d.ts") || strings.HasSuffix(f, ".d.mts") {
			continue
		}
		if strings.HasSuffix(f, ".ts") || (*js && (strings.HasSuffix(f, ".mjs") || strings.HasSuffix(f, ".js"))) {
			r.Files = append(r.Files, f)
		}
	}
	r.Checker = p.CheckerDiagnostics(context.Background())
	// Trap mode rewrites blockers to notes and repairs invalid signatures.
	// A second, untrapped run preserves original categories and verifier evidence.
	err = attempt(func() error {
		raw, ds, e := p.LowerWithOptions(r.Files, tsfront.LowerOptions{})
		r.RawLower = ds
		if raw != nil {
			for _, v := range hir.Verify(raw) {
				r.RawVerify = append(r.RawVerify, v.Error())
			}
		}
		return e
	})
	r.RawError = message(err)
	var h *hir.Program
	err = attempt(func() error {
		var e error
		h, r.Lower, e = p.LowerWithOptions(r.Files, tsfront.LowerOptions{TrapOnFailure: true})
		return e
	})
	r.LowerError = message(err)
	if h != nil {
		r.Classes = len(h.Classes)
		r.Interfaces = len(h.Interfaces)
		err = attempt(func() error {
			for _, e := range hir.Verify(h) {
				r.Verify = append(r.Verify, e.Error())
			}
			return nil
		})
		if err != nil {
			r.Verify = append(r.Verify, err.Error())
		}
		if e := save(filepath.Join(*out, "hir.txt"), []byte(hir.Dump(h))); e != nil {
			panic(e)
		}
		r.GoFiles, r.GoError = emit(filepath.Join(*out, "go"), func() (map[string]string, error) { return golang.EmitPackage(h, "spike") })
		r.ABAPFiles, r.ABAPError = emit(filepath.Join(*out, "abap"), func() (map[string]string, error) { return abap.Emit(h) })
		if r.GoError != "" {
			r.BuildError = "not attempted: Go emission failed"
		} else {
			if e := save(filepath.Join(*out, "go", "go.mod"), []byte("module gogen-spike-output\n\ngo 1.26.0\n")); e != nil {
				panic(e)
			}
			cmd := exec.Command("go", "build", "./...")
			cmd.Dir = filepath.Join(*out, "go")
			data, e := cmd.CombinedOutput()
			r.BuildError = message(e)
			r.BuildOutput = string(data)
		}
	}
	data, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = save(filepath.Join(*out, "result.json"), append(data, '\n')); e != nil {
		panic(e)
	}
	fmt.Printf("%s: files=%d checker=%d lower=%d verify=%d Go=%d (%s) build=%s ABAP=%d (%s) lowerError=%s\n", *out, len(r.Files), len(r.Checker), len(r.Lower), len(r.Verify), r.GoFiles, r.GoError, r.BuildError, r.ABAPFiles, r.ABAPError, r.LowerError)
}
