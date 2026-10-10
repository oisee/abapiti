package main

// Analysis-only export: all emitted method bodies, using the same lexical
// scanner and abstraction as stmt-patterns. No compiler transformations.
import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir/abap"
)

var peepholeFlag = flag.Bool("peephole", false, "export all emitted methods and normalized statements for 2..4-window mining")

type miningStatement struct {
	Text       string `json:"text"`
	Normalized string `json:"normalized"`
	Line       int    `json:"line"`
	Depth      int    `json:"depth"`
}
type miningMethod struct {
	Class      string            `json:"class"`
	ABAP       string            `json:"method"`
	TS         string            `json:"ts"`
	Source     string            `json:"source"`
	Statements []miningStatement `json:"statements"`
}

func exportPeepholes(input, output string) error {
	raw, err := os.ReadFile(filepath.Join(input, "names.json"))
	if err != nil {
		return err
	}
	names := map[string]abap.SourceEntry{}
	if err = json.Unmarshal(raw, &names); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(input, "classes", "*.clas.abap"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	var result []miningMethod
	for _, file := range files {
		class := strings.ToUpper(strings.TrimSuffix(filepath.Base(file), ".clas.abap"))
		entry := names[class]
		if entry.ID == "" {
			entry.ID = strings.ToLower(class)
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		for _, m := range parseMethods(string(raw)) {
			member := names[strings.ToUpper(m.ABAP)]
			name := strings.TrimPrefix(member.ID, "member.")
			if name == "" {
				name = m.ABAP
			}
			mm := miningMethod{Class: class, ABAP: m.ABAP, TS: entry.ID + "." + name}
			for _, d := range member.Declared {
				if strings.HasPrefix(d, entry.ID+"."+name+"@") {
					mm.Source = strings.SplitN(d, "@", 2)[1]
					break
				}
			}
			if name == "constructor" {
				mm.Source = entry.Source
			}
			for _, s := range m.Statements {
				mm.Statements = append(mm.Statements, miningStatement{s.Text, normalize(s.Text), s.Line, s.Depth})
			}
			result = append(result, mm)
		}
	}
	if err = os.MkdirAll(output, 0755); err != nil {
		return err
	}
	raw, err = json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "statements.json"), append(raw, '\n'), 0644)
}
