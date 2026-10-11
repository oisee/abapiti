#!/usr/bin/env node
import assert from 'node:assert/strict';
import {mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join, dirname} from 'node:path';
import {execFileSync} from 'node:child_process';
import {discoverShards, selectShards, assertNoFailures} from './abaplint-tests-all.mjs';
import {ownedResults} from './abaplint-tests-owned.mjs';
const dir = mkdtempSync(join(tmpdir(),'abaplint-all-test-'));
try {
 assertNoFailures('syntax', {results:[{status:'passed'},{status:'trapped',message:'refusal'}]});
 assert.throws(() => assertNoFailures('syntax', {results:[{status:'failed',title:'regression',message:'expected 0 to equal 1'}]}), /STOP: syntax: 1 genuine FAIL.*regression.*expected 0 to equal 1/s);
 const paths = ['root.ts', 'abap/helper.ts', 'abap/syntax/syntax.ts', 'abap/syntax/basic.ts', 'objects/object.ts', 'objects/rename/rename.ts', 'new_directory/future.ts'];
 for (const path of paths) {
  const file = join(dir, 'packages/core/test', path);
  mkdirSync(dirname(file), {recursive:true}); writeFileSync(file, '');
 }
 const borrowed = {total:2, passed:1, failed:0, pending:1, titles:['borrowed','borrowed-pending'], passedTitles:['borrowed'], failedTitles:[], pendingTitles:['borrowed-pending']};
 const original = {total:5, passed:4, failed:0, pending:1, titles:['own','own','borrowed','borrowed-pending','missing'], passedTitles:['own','own','borrowed','missing'], failedTitles:[], pendingTitles:['borrowed-pending'], failures:[]};
 const executed = {results:[{title:'own',status:'passed'},{title:'own',status:'trapped'},{title:'borrowed',status:'passed'}]};
 const owned = ownedResults(original, executed, borrowed, ['test/rules/shared.ts']);
 assert.deepEqual(owned.oracle.titles, ['own','own','missing']);
 assert.deepEqual([owned.oracle.total,owned.oracle.passed,owned.oracle.pending],[3,3,0]);
 assert.equal(owned.run.results.length,2); assert.equal(owned.run.borrowedRegistered,1);
 assert.equal(owned.run.rawRegistered,3);
 assert.throws(() => ownedResults({...original,titles:[...original.titles,'borrowed']},executed,borrowed,[]),/ambiguous/);
 assert.throws(() => ownedResults({...original,titles:['own']},executed,borrowed,[]),/missing/);
 const discovered = discoverShards(dir);
 assert.equal(discovered.files.length, 7);
 assert.deepEqual(discovered.shards.map(s => s.name).sort(), ['abap','abap/syntax','new_directory','objects','objects/rename','root']);
 assert.deepEqual(discovered.shards.flatMap(s => s.roots).sort(), discovered.files);
 assert.equal(new Set(discovered.shards.flatMap(s => s.roots)).size, 7);
 const selected = selectShards(discovered, 'abap/syntax,root');
 assert.equal(selected.partial,true);
 assert.deepEqual(selected.shards.map(s => s.name).sort(),['abap/syntax','root']);
 assert.equal(selected.files.length,3); assert.equal(selected.excluded.length,4);
 assert.equal(selectShards(discovered,undefined),discovered);
 assert.equal(selectShards(discovered,discovered.shards.map(s => s.name).join(',')),discovered);
 assert.throws(() => selectShards(discovered,'syntax'),/unknown test shard/);
 assert.throws(() => selectShards(discovered,'root,root'),/distinct nonempty/);
 assert.throws(() => selectShards(discovered,','),/distinct nonempty/);
 // Non-four-shard summary, duplicate titles, skipped tests, and root/child dirs.
 const shards = ['root','abap','lsp','objects/rename','abap/syntax'].map(name => ({name, id:name.replaceAll('/','-'), roots:[`test/${name}.ts`]}));
 const plan = {files:shards.flatMap(s => s.roots).sort(), shards, excluded:[]};
 writeFileSync(join(dir,'shards.json'), JSON.stringify(plan));
 const node = {total:2213, passed:2212, failed:0, pending:1, titles:Array(2213).fill('duplicate')};
 const run = {results:[{title:'duplicate',status:'passed',message:''},{title:'duplicate',status:'trapped',message:'lowering trap: test/root.ts:1:1: unsupported'}]};
 for (const {id, roots} of shards) {
  writeFileSync(join(dir,`node-${id}.json`), JSON.stringify(node));
  writeFileSync(join(dir,`go-${id}.json`), JSON.stringify(run));
  execFileSync(process.execPath, [join(import.meta.dirname,'abaplint-tests-report.mjs'),join(dir,`node-${id}.json`),join(dir,`go-${id}.json`),join(dir,`report-${id}.json`)]);
  mkdirSync(join(dir,id)); writeFileSync(join(dir,id,'closure.json'),JSON.stringify({upstreamPin:'pin',roots}));
 }
 const all = {...node, total:11065, passed:11060, pending:5, titles:Array(11065).fill('duplicate')};
 const whole = () => writeFileSync(join(dir,'node-all.json'),JSON.stringify(all)); whole();
 const summarize = () => execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-summary.mjs'),dir,join(dir,'summary.json')], {stdio:'pipe'});
 summarize();
 const summary = JSON.parse(readFileSync(join(dir,'summary.json')));
 assert.equal(summary.shards.length,5); assert.equal(summary.uniqueTotal.total,11065);
 assert.equal(summary.uniqueTotal.passed,5); assert.equal(summary.uniqueTotal.trapped,5);
 assert.equal(summary.uniqueTotal.notRun,11055);
 assert.equal(summary.wholeSuite.active,11060); assert.equal(summary.top20Traps[0].count,5);
 assert.equal(summary.wholeSuite.titlesReconciled,true); assert.equal(summary.nodeTotal.pending,5);
 all.titles[0] = 'omitted'; whole(); assert.throws(summarize,/shard titles differ/);
 all.titles[0] = 'duplicate'; all.passed--; whole(); assert.throws(summarize,/whole-suite Node passed mismatch/);
 all.passed++; whole(); plan.files.push('test/missing.ts'); plan.files.sort();
 writeFileSync(join(dir,'shards.json'),JSON.stringify(plan)); assert.throws(summarize,/disjoint exhaustive partition/);
 const partial = {shards:shards.slice(0,1), files:[...shards[0].roots], excluded:shards.slice(1).flatMap(s => s.roots), partial:true, compilerCommit:'fixture-baseline'};
 writeFileSync(join(dir,'shards.json'),JSON.stringify(partial));
 rmSync(join(dir,'node-all.json'));
 summarize();
 const focused = JSON.parse(readFileSync(join(dir,'summary.json')));
 assert.equal(focused.wholeSuite,undefined);
 assert.equal(focused.selectedSuite.active,2212);
 assert.deepEqual(focused.selectedSuite.shards,['root']);
 assert.equal(focused.codeCommit,'fixture-baseline');
 assert.equal(focused.uniqueTotal.passed,1);
 partial.files.push('test/missing.ts');
 writeFileSync(join(dir,'shards.json'),JSON.stringify(partial));
 assert.throws(summarize,/disjoint exhaustive partition/);
 console.log('all-shard discovery, root/child partition, dependency-safe syntax directory, borrowed registrations, whole-suite multiset and totals: ok');
} finally {rmSync(dir, {recursive:true, force:true});}
