package sourcemap

import "github.com/oisee/abapiti/internal/tsgo/core"

type Source interface {
	Text() string
	FileName() string
	ECMALineMap() []core.TextPos
}
