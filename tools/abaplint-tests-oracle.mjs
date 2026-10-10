#!/usr/bin/env node
// Fresh original upstream build and original mocha/chai; never uses our runtime.
import {execFileSync} from "node:child_process";
import {writeFileSync, existsSync} from "node:fs";
import {join, dirname, resolve} from "node:path";
import {buildUpstream} from "./statements-upstream.mjs";
const [output, upstream = "/home/alice/dev/abaplint", ...files] = process.argv.slice(2);
if (!output || !files.length) throw new Error("usage: abaplint-tests-oracle.mjs OUTPUT.json UPSTREAM test/rules/7bit_ascii.ts [...files]");
for (const file of files) if (!/^test\/.+\.ts$/.test(file) || file.includes("..")) throw new Error(`invalid test path: ${file}`);
const build = buildUpstream(upstream);
try {
 const mocha = [join(upstream,"node_modules/mocha/bin/mocha.js"), join(upstream,"packages/core/node_modules/mocha/bin/mocha.js")].find(existsSync);
 if (!mocha) throw new Error("upstream mocha is not installed");
 let raw;
 try {
  raw = execFileSync(process.execPath, [mocha,"--reporter","json", ...files.map(f => join(dirname(build.core), f.replace(/\.ts$/, ".js")))], {encoding:"utf8"});
 } catch (e) { if (!e.stdout) throw e; raw = e.stdout; }
 const result = JSON.parse(raw);
 const summary = {passed:result.stats.passes,failed:result.stats.failures,failures:result.failures.map(t => ({title:t.fullTitle,message:t.err.message})),
  titles:result.passes.map(t => t.fullTitle)};
 writeFileSync(resolve(output), JSON.stringify(summary,null,2)+"\n");
 console.log(JSON.stringify(summary));
} finally { build.dispose(); }
