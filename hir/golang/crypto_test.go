package golang

import (
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestSHA1UTF16EncodingNodeOracle(t *testing.T) {
	want, err := exec.Command("node", "-e", `const c=require("node:crypto");for(const s of ["\ud800x\udfff","\ud800\udc00","\udc00\ud800","\ufffd"])console.log(c.createHash("sha1").update(s).digest("hex"))`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	main := `for _,s:=range []jsString{jsString("\x00\xd8x\x00\xff\xdf"),jsString("\x00\xd8\x00\xdc"),jsString("\x00\xdc\x00\xd8"),str("\ufffd")} {fmt.Println(sha1Hex(s).String())}`
	if got := execute(t, &hir.Program{}, main); got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
}
