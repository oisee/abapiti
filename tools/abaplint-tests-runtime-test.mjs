#!/usr/bin/env node
// Independent Node contract tests for the runtime draft, not TS-HG acceptance.
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {createRequire} from "node:module";
import {resolve} from "node:path";
import vm from "node:vm";
import {verifyUpstream} from "./statements-upstream.mjs";
const upstream = process.argv[2] || "/home/alice/dev/abaplint";
const ts = createRequire(import.meta.url)(verifyUpstream(upstream));
const runtime = resolve(import.meta.dirname, "../tsfront/testdata/abaplint-tests/runtime");
function run(source) {
 const context = vm.createContext({exports:{}});
 for (const name of ["mocha.ts", "chai.ts"]) {
  const raw = readFileSync(resolve(runtime, name), "utf8");
  const output = ts.transpileModule(raw, {fileName:name,compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
  vm.runInContext(output, context, {filename:name});
 }
 vm.runInContext("const expect = exports.expect; " + source, context);
 return JSON.parse(vm.runInContext("JSON.stringify(TestRuntime.run())", context));
}
assert.deepEqual(run(`
 class Item {}
 describe("suite", function() {
  this.timeout(1);
  it("equal", () => { expect(3).to.equal(3); expect(3).to.equals(3); expect(3).to.not.equal(4); expect(3).to.not.equals(4); });
  it("instanceof", () => { expect(new Item()).to.be.instanceof(Item); expect({}).to.not.be.instanceof(Item); });
  it("contain", () => { expect("abc").to.contain("b"); expect([1,2]).to.include(2); expect("abc").to.not.contain("z"); });
  it("length", () => { expect("abc").to.have.length(3); expect([1,2]).to.have.length(2); });
  it("undefined", () => { expect(undefined).to.be.undefined; expect(1).to.not.be.undefined; });
  it("failure", () => expect(1).to.equal(2, "custom message"));
  it.skip("skip", () => { throw new Error("must not run"); });
 });
 describe.skip("skipped", () => it("skip suite", () => { throw new Error("must not run"); }));
`), {passed:5,failed:1,failures:[{title:"suite failure",message:"Error: custom message: expected 1 to equal 2"}]});
assert.deepEqual(run(`
 describe("outer", function() {
  it("ignored", () => { throw new Error("not focused"); });
  describe("inner", function() { this.timeout(0); it.only("focused", () => expect(true).to.equal(true)); });
 });
 it.only("second", () => expect(undefined).to.not.equals(1));
 it.skip("skipped focus", () => { throw new Error("skip"); });
`), {passed:2,failed:0,failures:[]});
assert.deepEqual(run(`
 describe.only("focused suite", () => it("focused", () => {}));
 describe("other", () => it("ignored", () => { throw new Error("not focused"); }));
`), {passed:1,failed:0,failures:[]});
assert.deepEqual(run(`
 it("expect message", () => expect(1, "expect message").to.not.equals(1));
 it("explicit failure", () => expect.fail("explicit"));
`), {passed:0,failed:2,failures:[
 {title:"expect message",message:"Error: expect message: expected 1 not to equal 1"},
 {title:"explicit failure",message:"Error: explicit"},
]});
assert.deepEqual(run(`
 describe.only("focused but skipped", () => it.skip("pending", () => {}));
 it("outside", () => { throw new Error("not focused"); });
`), {passed:0,failed:0,failures:[]});
assert.deepEqual(run(`
 describe.only("empty focused suite", () => {});
 it("outside", () => { throw new Error("not focused"); });
`), {passed:0,failed:0,failures:[]});
console.log("runtime Node contract: 6 scenarios passed (not TS-HG@Go)");
