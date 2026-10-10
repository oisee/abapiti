package tsfront

import (
	"path/filepath"
	"strings"
)

// siteSource changes metadata only; diagnostic Source retains its legacy path.
func (p *Program) siteSource(source string) string {
	source = strings.ReplaceAll(source, "\\", "/")
	root := filepath.ToSlash(p.configDir) + "/"
	if strings.HasPrefix(source, root) {
		return strings.TrimPrefix(source, root)
	}
	if i := strings.Index(source, "/node_modules/"); i >= 0 {
		return source[i+1:]
	}
	return source
}
