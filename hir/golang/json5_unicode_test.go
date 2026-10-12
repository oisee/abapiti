package golang

import (
	"os/exec"
	"testing"
)

// Exhaustively compare all code points to the installed package's actual
// tables, not the host Unicode categories or our generator.
func TestJSON5UnicodePackageOracle(t *testing.T) {
	pkg := json5PackagePath(t)
	out, err := exec.Command("node", "-e", `const u=require(process.argv[1]+"/lib/unicode.js");for(const re of [u.ID_Start,u.ID_Continue]){let lo;for(let cp=0;cp<=0x110000;cp++){const yes=cp<0x110000&&re.test(String.fromCodePoint(cp));if(yes&&lo===undefined)lo=cp;if(!yes&&lo!==undefined){process.stdout.write(lo+"-"+(cp-1)+",");lo=undefined}}console.log()}`, pkg).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	got := execute(t, configJSONFixture(), `for _,rs:=range [][][2]int32{json5IDStart,json5IDContinue} {for _,r:=range rs {fmt.Printf("%d-%d,",r[0],r[1])};fmt.Println()}`)
	if got != string(out) {
		t.Fatal("JSON5 Unicode range mismatch")
	}
}
