#!/usr/bin/env node
// Independent Node contract tests for the runtime draft, not TS-HG acceptance.
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {createRequire} from "node:module";
import {resolve} from "node:path";
import vm from "node:vm";
import {verifyUpstream} from "./statements-upstream.mjs";
const upstream = process.argv[2] || resolve(process.env.HOME, "dev/abaplint");
const require = createRequire(import.meta.url);
const ts = require(verifyUpstream(upstream));
const originalChai = require(resolve(upstream, "packages/core/node_modules/chai"));
// The transpiled runtime deliberately leaves target-only intrinsic stubs.
// This independent Node contract must install the real package semantics,
// not execute that stub or silently return true for array inclusion.
function nodeArrayInclude(actual, expected) {
 try { originalChai.assert.include(actual, expected); return true; }
 catch (error) { if (error instanceof originalChai.AssertionError) return false; throw error; }
}
const runtime = resolve(import.meta.dirname, "../tsfront/testdata/abaplint-tests/runtime");
async function run(source) {
 const context = vm.createContext({exports:{}, nodeArrayInclude});
 for (const name of ["mocha.ts", "chai.ts"]) {
  const raw = readFileSync(resolve(runtime, name), "utf8");
  const output = ts.transpileModule(raw, {fileName:name,compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
  vm.runInContext(output, context, {filename:name});
 }
 vm.runInContext("chaiArrayInclude = nodeArrayInclude; const expect = exports.expect; " + source, context);
 return JSON.parse(JSON.stringify(await vm.runInContext("TestRuntime.run()", context)));
}
assert.deepEqual(await run(`
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
assert.deepEqual(await run(`
 describe("outer", function() {
  it("ignored", () => { throw new Error("not focused"); });
  describe("inner", function() { this.timeout(0); it.only("focused", () => expect(true).to.equal(true)); });
 });
 it.only("second", () => expect(undefined).to.not.equals(1));
 it.skip("skipped focus", () => { throw new Error("skip"); });
`), {passed:2,failed:0,failures:[]});
assert.deepEqual(await run(`
 describe.only("focused suite", () => it("focused", () => {}));
 describe("other", () => it("ignored", () => { throw new Error("not focused"); }));
`), {passed:1,failed:0,failures:[]});
assert.deepEqual(await run(`
 it("expect message", () => expect(1, "expect message").to.not.equals(1));
 it("explicit failure", () => expect.fail("explicit"));
`), {passed:0,failed:2,failures:[
 {title:"expect message",message:"Error: expect message: expected 1 not to equal 1"},
 {title:"explicit failure",message:"Error: explicit"},
]});
assert.deepEqual(await run(`
 describe.only("focused but skipped", () => it.skip("pending", () => {}));
 it("outside", () => { throw new Error("not focused"); });
`), {passed:0,failed:0,failures:[]});
assert.deepEqual(await run(`
 describe.only("empty focused suite", () => {});
 it("outside", () => { throw new Error("not focused"); });
`), {passed:0,failed:0,failures:[]});
assert.deepEqual(await run(`
 class State { static value = ""; }
 before(async function() { this.timeout(1); State.value += "root"; });
 describe("async", () => {
  before(async () => { State.value += ":before"; });
  beforeEach(async () => { State.value += ":each"; });
  afterEach(() => Promise.resolve().then(() => { State.value += ":afterEach"; }));
  after(async () => { if (State.value !== "root:before:each:afterEach:each:afterEach") { throw new Error(State.value); } });
  it("fulfillment", async () => { await Promise.resolve(); });
  it("rejection", () => Promise.reject(new Error("rejected")));
 });
`), {passed:1,failed:1,failures:[{title:"async rejection",message:"Error: rejected"}]});
assert.deepEqual(await run(`
 describe("setup failure", () => {
  before(async () => { throw new Error("setup rejected"); });
  it("first", () => { throw new Error("body must not run"); });
  it("second", async () => {});
 });
 describe("teardown failure", () => {
  afterEach(async () => { throw new Error("cleanup rejected"); });
  it("test", async () => {});
 });
 describe("after failure", () => {
  after(() => Promise.reject(new Error("after rejected")));
  it("test", async () => {});
 });
`), {passed:0,failed:4,failures:[
 {title:"setup failure first",message:"Error: setup rejected"},
 {title:"setup failure second",message:"Error: setup rejected"},
 {title:"teardown failure test",message:"Error: cleanup rejected"},
 {title:"after failure test",message:"Error: after rejected"},
]});
assert.deepEqual(await run(`
 const object = {};
 it("strict array identity", () => {expect([object]).to.include(object);expect([object]).to.not.include({});expect([1,2]).to.not.include(3);});
 it("real absent inclusion failure", () => expect([1,2]).to.include(3));
`), {passed:1,failed:1,failures:[{title:"real absent inclusion failure",message:"Error: assertion failed: contain check"}]});
console.log("runtime Node contract: 9 scenarios passed against original Chai (not TS-HG@Go)");
