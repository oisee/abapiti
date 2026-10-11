#!/usr/bin/env node
// Diagnostic-only original Node run; use with the locally combined harness.
import {buildUpstream} from './statements-upstream.mjs';
import {createRequire} from 'node:module';
import {writeFileSync} from 'node:fs';
import {join, dirname} from 'node:path';
const [out, upstream, ...files] = process.argv.slice(2);
if (!out || !upstream || !files.length) {
 throw new Error('usage: abaplint-tests-triage-node.mjs OUTPUT.json PINNED_UPSTREAM TEST_FILES...');
}
const build = buildUpstream(upstream);
try {
 const require = createRequire(join(upstream, 'packages/core/package.json'));
 const Mocha = require('mocha');
 const rules = require(join(build.core, 'rules/index.js'));
 const Registry = require(join(build.core, 'registry.js')).Registry;
 const mocha = new Mocha({timeout:10000, reporter:class {constructor() {}}});
 let title = 'initialization';
 let occurrence = 0;
 const occurrences = new Map();
 const traces = [];
 const record = (site, issues) => traces.push({title, occurrence, site, issues:issues.map(i => ({
  key:i.getKey(), message:i.getMessage(), row:i.getStart().getRow(), col:i.getStart().getCol(),
 }))});
 const findIssues = Registry.prototype.findIssues;
 Registry.prototype.findIssues = function(...args) {
  const result = findIssues.apply(this, args);
  record('Registry.findIssues', result);
  return result;
 };
 const seen = new Set();
 for (const ctor of Object.values(rules)) if (typeof ctor === 'function') {
  for (let p = ctor.prototype; p && p !== Object.prototype; p = Object.getPrototypeOf(p)) {
   if (seen.has(p)) continue;
   seen.add(p);
   for (const method of ['run', 'runParsed']) {
    if (!Object.hasOwn(p, method) || typeof p[method] !== 'function') continue;
    const original = p[method];
    p[method] = function(...args) {
     const result = original.apply(this, args);
     if (Array.isArray(result)) record(ctor.name+'.'+method, result);
     return result;
    };
   }
  }
 }
 for (const file of files) mocha.addFile(join(dirname(build.core), file.replace(/\.ts$/, '.js')));
 const failures = await new Promise(resolve => {
  const runner = mocha.run(resolve);
  runner.on('test', test => {
   title = test.fullTitle();
   occurrence = occurrences.get(title) ?? 0;
   occurrences.set(title, occurrence+1);
  });
  runner.on('end', () => console.error(JSON.stringify(runner.stats)));
 });
 if (failures) throw new Error(`instrumented Node oracle has ${failures} failures`);
 writeFileSync(out, JSON.stringify(traces, null, 2)+'\n');
} finally {build.dispose();}
