package hostio

import (
	"os"
	"os/exec"
	"path"
	"strings"
	"unicode/utf8"
)

func ReadText(file string) (string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return decodeUTF8(b), nil
}
func WriteText(file, text string) error { return os.WriteFile(file, []byte(text), 0666) }
func ListDir(dir string) ([]string, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(es))
	for i, e := range es {
		names[i] = e.Name()
	}
	return names, nil
}
func ListEntries(dir string) ([]struct {
	F0 string
	F1 bool
}, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]struct {
		F0 string
		F1 bool
	}, len(es))
	for i, e := range es {
		out[i].F0 = e.Name()
		out[i].F1 = e.IsDir()
	}
	return out, nil
}
func Basename(file string) string {
	file = strings.TrimRight(file, "/")
	if i := strings.LastIndexByte(file, '/'); i >= 0 {
		return file[i+1:]
	}
	return file
}
func Dirname(file string) string {
	if file == "" {
		return "."
	}
	end := -1
	matched := true
	for i := len(file) - 1; i >= 1; i-- {
		if file[i] == '/' {
			if !matched {
				end = i
				break
			}
		} else {
			matched = false
		}
	}
	if end == -1 {
		if file[0] == '/' {
			return "/"
		}
		return "."
	}
	if file[0] == '/' && end == 1 {
		return "//"
	}
	return file[:end]
}
func Join(parts ...string) string {
	s := []string{}
	for _, p := range parts {
		if p != "" {
			s = append(s, p)
		}
	}
	if len(s) == 0 {
		return "."
	}
	joined := strings.Join(s, "/")
	clean := path.Clean(joined)
	if strings.HasSuffix(joined, "/") && clean != "/" {
		clean += "/"
	}
	return clean
}
func GitHead(cwd string) (string, bool) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = cwd
	b, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}
func Env(name string) (string, bool) { return os.LookupEnv(name) }

func decodeUTF8(b []byte) string {
	var out strings.Builder
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r != utf8.RuneError || n > 1 {
			out.WriteRune(r)
			b = b[n:]
			continue
		}
		n = 1
		want := 1
		switch {
		case b[0] >= 0xc2 && b[0] <= 0xdf:
			want = 2
		case b[0] >= 0xe0 && b[0] <= 0xef:
			want = 3
		case b[0] >= 0xf0 && b[0] <= 0xf4:
			want = 4
		}
		for n < want && n < len(b) {
			v := b[n]
			if v < 0x80 || v > 0xbf {
				break
			}
			if n == 1 && (b[0] == 0xe0 && v < 0xa0 || b[0] == 0xed && v >= 0xa0 || b[0] == 0xf0 && v < 0x90 || b[0] == 0xf4 && v >= 0x90) {
				break
			}
			n++
		}
		out.WriteRune(utf8.RuneError)
		b = b[n:]
	}
	return out.String()
}
