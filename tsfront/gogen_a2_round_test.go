package tsfront

import (
	"strings"
	"testing"
)

// Run Node before lowering, then require exact execution or the specific,
// located blocking refusal. Other diagnostics cannot mask a regression.
func gogenA2ExactOrRefused(t *testing.T, source, want, category, message string) {
	t.Helper()
	source = `"use strict";` + source
	if got := conversionOracle(t, source); got != want+"\n" {
		t.Fatalf("Node %q want %q", got, want)
	}
	p, diags := lowerProbe(t, sourceProbe(t, source))
	refused := false
	var blocking []LowerDiagnostic
	for _, d := range diags {
		if d.Category == category && strings.Contains(d.Message, message) && strings.Contains(d.Loc, "input.ts:") {
			refused = true
			continue
		}
		if !strings.HasPrefix(d.Category, "note-") {
			blocking = append(blocking, d)
		}
	}
	if !refused {
		if len(blocking) != 0 {
			t.Fatalf("missing specific refusal; diagnostics: %v", blocking)
		}
		gogenCheck(t, p, "input.ts", source, want)
	}
}

func TestGogenA2NestedFinallyOverride(t *testing.T) {
	for name, body := range map[string]string{
		"exception":    `try{try{throw new Error("original")}finally{return 2}}finally{}return 0;`,
		"return":       `try{return 1}finally{try{return 3}finally{return 2}}`,
		"outer_return": `try{try{return 1}finally{return 2}}finally{}return 0;`,
	} {
		t.Run(name, func(t *testing.T) {
			source := `function f():number {` + body + `} class Probe {static run():string {try{return ""+f()}catch(e){return (e as Error).message}}}`
			gogenA2ExactOrRefused(t, source, "2", "unsupported-statement", "return in finally")
		})
	}
}

func TestGogenA2ModuleCallbackThis(t *testing.T) {
	for _, callback := range []string{"cb", "alias"} {
		t.Run(callback, func(t *testing.T) {
			source := `function cb(v:number):boolean{return this===undefined;} const alias=cb;
class Probe {static run():string{return [1].map(` + callback + `).join();}}`
			gogenA2ExactOrRefused(t, source, "true", "unsupported-callback", "undefined this")
		})
	}
}

func TestGogenA2NamedArrowLexicalThis(t *testing.T) {
	gogenOracle(t, `class Probe {value=7;runInner():string {const cb=(v:number):number=>this.value+v;return [1].map(cb).join();}static run():string{return new Probe().runInner();}}`, "8")
}

func TestGogenA2NamedCallbackThis(t *testing.T) {
	for name, declaration := range map[string]string{
		"expression":   `const cb=function(v:number):boolean{return this===undefined;};`,
		"declaration":  `function cb(v:number):boolean{return this===undefined;}`,
		"alias":        `const original=function(v:number):boolean{return this===undefined;};const cb=original;`,
		"nested_arrow": `function cb(v:number):boolean{const read=()=>this===undefined;return read();}`,
	} {
		t.Run(name, func(t *testing.T) {
			source := `class Probe {runInner():string {` + declaration + `return [1].map(cb).join();}static run():string{return new Probe().runInner();}}`
			gogenA2ExactOrRefused(t, source, "true", "unsupported-callback", "undefined this")
		})
	}
}
