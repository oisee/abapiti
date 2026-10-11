import assert from 'node:assert/strict';
import {fixRefusalStatus} from './abaplint-tests-refusals.mjs';
const source = 'status:="failed";if _,ok:=x.(trap);ok{status="trapped"}\nr.Status="failed";if _,ok:=x.(trap);ok {r.Status="trapped"}\nif strings.Contains(r.Message,"unsupported-async") {r.Status="not-run"}';
const fixed = fixRefusalStatus(source);
assert.equal((fixed.match(/case trap, rangeFault:/g) ?? []).length, 2);
assert.ok(fixed.includes('status:="failed";switch'));
assert.ok(fixed.includes('r.Status="failed";switch'));
assert.ok(fixed.includes('"unsupported-async") {r.Status="not-run"}'));
assert.throws(() => fixRefusalStatus(source.split('\n')[0]), /expected two/);
assert.throws(() => fixRefusalStatus(fixed), /expected two/);
console.log('refusal status recovery boundaries: ok');
// Execute both host boundaries: refusals must trap; assertion payloads and
// unexpected Go panics must remain failures. No message-based classification.
const {mkdtempSync, writeFileSync, rmSync} = await import('node:fs');
const {tmpdir} = await import('node:os');
const {join} = await import('node:path');
const {execFileSync} = await import('node:child_process');
const dir = mkdtempSync(join(tmpdir(), 'triage-refusal-test-'));
try {
 const [initBoundary, testBoundary] = fixed.split('\n');
 writeFileSync(join(dir, 'main.go'), `package main
import "fmt"
type trap struct{}
type rangeFault struct{}
type payload struct{}
func main(){for _,x:=range []any{trap{},rangeFault{},payload{},"unexpected Go panic"}{
 func(){defer func(){x:=recover();${initBoundary};fmt.Print(status," ")}();panic(x)}()
 func(){defer func(){x:=recover();r:=struct{Status string}{};${testBoundary};fmt.Println(r.Status)}();panic(x)}()
}}`);
 assert.equal(execFileSync('go', ['run', join(dir, 'main.go')], {encoding:'utf8', env:{...process.env, GOFLAGS:'-p=2', GOMAXPROCS:'1'}}), 'trapped trapped\ntrapped trapped\nfailed failed\nfailed failed\n');
} finally {rmSync(dir, {recursive:true, force:true});}
console.log('refusals versus assertions: Go execution ok');
