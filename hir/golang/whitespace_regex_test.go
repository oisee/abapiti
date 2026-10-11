package golang

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestWhitespaceRegexNodeOracle(t *testing.T) {
	// Every BMP code unit distinguishes ECMAScript WhiteSpace/LineTerminator
	// from Go's ASCII \\s and Unicode White_Space (notably U+0085/U+FEFF).
	inputs := []string{"😀 \tfoo\n\u00a0bar\ufeff", "", "a", " \t\r\n"}
	for r := rune(0); r <= 0xffff; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		} // JS lone surrogates tested by runtime string tests.
		inputs = append(inputs, "x"+string(r)+" "+string(r)+"y")
	}
	data, _ := json.Marshal(inputs)
	// stdin avoids argv-size limits.
	cmd := exec.Command("node", "-e", `let input='';const units=s=>Array.from({length:s.length},(_,i)=>s.charCodeAt(i)+',').join('');process.stdin.on('data',c=>input+=c);process.stdin.on('end',()=>{for(const s of JSON.parse(input)){const r=/\s+/g;r.lastIndex=1;const m=r.exec(s);console.log(units(s.replace(/\s+/g,'_'))+':'+(m?units(m[0])+':'+m.index:'null:null')+':'+r.lastIndex);}})`)
	cmd.Stdin = strings.NewReader(string(data))
	want, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	var main strings.Builder
	main.WriteString("inputs:=[]string{")
	for _, s := range inputs {
		fmt.Fprintf(&main, "%q,", s)
	}
	main.WriteString(`};units:=func(s jsString){for i:=int32(0);i<s.length();i++{fmt.Printf("%d,",s.charCodeAt(i))}};for _,s:=range inputs {r:=newRegExp(str("\\s+"),str("g"));r.LastIndex=1;m:=r.exec(str(s));units(str(s).replaceRegex(newRegExp(str("\\s+"),str("g")),str("_")));fmt.Print(":");if m!=nil {units(m.Items[0]);fmt.Printf(":%d",m.MatchIndex)}else{fmt.Print("null:null")};fmt.Printf(":%d\n",r.LastIndex)}`)
	got := execute(t, &hir.Program{}, main.String())
	if got != string(want) {
		t.Fatal("whitespace replace/exec differs from Node")
	}
}
