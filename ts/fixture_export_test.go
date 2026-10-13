package ts

import (
	"testing"

	"github.com/oisee/abapiti/internal/abapfixture"
)

func transpileABAP(t *testing.T, astJSON []byte, prefix string) (*TranspileResult, error) {
	t.Helper()
	result, err := Transpile(astJSON, prefix)
	if err == nil {
		files := make(map[string]string, len(result.Classes))
		for name, src := range result.Classes {
			files[name+".clas.abap"] = src
		}
		abapfixture.Track(t, "ts", files)
	}
	return result, err
}
