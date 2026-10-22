package tsfront

import "testing"

func TestStep4EmitMatchIndexSliceNodeOracle(t *testing.T) {
	gogenTSOracle(t, `class Probe {static run(): string {const out:string[]=[];for(const line of ["func one(){", "😀func two(){"]){for(const match of line.matchAll(/func/g)){out.push(line.slice(match.index,line.indexOf("{",match.index)));}}return out.join("|");}}`, "regexp.matchAllNext")
}
