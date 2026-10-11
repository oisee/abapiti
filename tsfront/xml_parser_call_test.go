package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestXMLParserCallNodeOracle(t *testing.T) {
	dir := t.TempDir()
	for _, pkg := range RegistryNodePackages() {
		if pkg.Name == "fast-xml-parser" {
			if err := pkg.WriteEmbedded(filepath.Join(dir, "node_modules", pkg.Name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	source := `import {XMLParser as Parser} from "fast-xml-parser";
 export class Probe {static parse(xml: string): string {return JSON.stringify(new Parser({parseTagValue:false,ignoreAttributes:true,trimValues:false}).parse(xml));}}`
	for name, text := range map[string]string{"main.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","module":"node16","moduleResolution":"node16"},"files":["main.ts"]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, ds, err := p.LowerWithOptions([]string{"main.ts"}, LowerOptions{TrapOnFailure: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if es := hir.Verify(prog); len(es) != 0 {
		t.Fatal(es)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	core := os.Getenv("TSFRONT_ABAPLINT")
	if core == "" {
		core = filepath.Join(home, "dev/abaplint/packages/core")
	}
	parser := filepath.Join(core, "node_modules/fast-xml-parser")
	if _, err := os.Stat(filepath.Join(parser, "package.json")); err != nil {
		t.Skip("live pinned fast-xml-parser oracle requires installed upstream npm dependencies")
	}
	cases := []string{`<root id="ignored"><item> 001 </item><item>true</item><empty/></root>`, `<?xml version="1.0"?><abapGit><asx:abap><asx:values><DESCRIPTIONS><SEOCOMPOTX><CMPNAME> METH &amp; </CMPNAME></SEOCOMPOTX><SEOCOMPOTX><CMPNAME>OTHER</CMPNAME></SEOCOMPOTX></DESCRIPTIONS></asx:values></asx:abap></abapGit>`, "<root><text>A\r\nB😀</text></root>"}
	// Make the package pin and Node execution explicit; never compare to a shim.
	script := `const p=require(process.argv[1]);const pkg=require(process.argv[1]+"/package.json");if(pkg.version!=="5.10.1")throw Error("wrong parser pin");console.log(JSON.stringify(new p.XMLParser({parseTagValue:false,ignoreAttributes:true,trimValues:false}).parse(process.argv[2])));`
	for _, xml := range cases {
		out, err := exec.Command("node", "-e", script, parser, xml).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		// Input is encoded as a Go quoted literal by the existing entry helper.
		got := runGoHIR(t, prog, "fmt.Print("+goEntry("main.ts.Probe", "parse")+"(str("+strconv.Quote(xml)+")).String())")
		if got != strings.TrimSpace(string(out)) {
			t.Fatalf("XML %q: Go %q Node %q", xml, got, out)
		}
	}
	for _, options := range []string{"parseTagValue:true,ignoreAttributes:true,trimValues:false", "parseTagValue:false,ignoreAttributes:false,trimValues:false", "parseTagValue:false,ignoreAttributes:true,trimValues:true", "parseTagValue:false,ignoreAttributes:true,trimValues:false,extra:true"} {
		text := strings.ReplaceAll(source, "parseTagValue:false,ignoreAttributes:true,trimValues:false", options)
		if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		negative, err := Load(filepath.Join(dir, "tsconfig.json"))
		if err != nil {
			t.Fatal(err)
		}
		_, ds, err := negative.Lower([]string{"main.ts"})
		if err != nil || !hasBlocking(ds) {
			t.Fatal("accepted unsupported parser options", options, err, ds)
		}
	}
}
