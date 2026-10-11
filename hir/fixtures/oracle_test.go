package fixtures

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestXMLOracleMatchesPinnedNode(t *testing.T) {
	if len(XMLCases()) != 444 {
		t.Fatalf("XML corpus changed: %d cases", len(XMLCases()))
	}
	oracle, err := filepath.Abs("../golang/testdata/xml-validator/validator.mjs")
	if err != nil {
		t.Fatal(err)
	}
	input, err := filepath.Abs("testdata/xml-validation.json")
	if err != nil {
		t.Fatal(err)
	}
	script := `import {validate} from ` + quoteJS(oracle) + `;import fs from 'node:fs';console.log(JSON.stringify(JSON.parse(fs.readFileSync(process.argv[1])).map(c=>({input:c.input,result:validate(c.input)}))));`
	out, err := exec.Command("node", "--input-type=module", "-e", script, input).CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v: %s", err, out)
	}
	var want, got any
	if err := json.Unmarshal(XMLOracle, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatal("committed XML oracle differs from fast-xml-parser 5.10.1")
	}
}
func quoteJS(s string) string { b, _ := json.Marshal(s); return string(b) }
