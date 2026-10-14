import assert from 'node:assert/strict';
import {test} from 'node:test';
import {mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {createSharedBuilder, runShards, assertNoFailures} from './abaplint-tests-all.mjs';
import {buildGroups} from './abaplint-tests-sharded-closure.mjs';

const shards = ['a','b','c','d'].map(name => ({name, id:name, roots:[`test/${name}.ts`]}));
const verdict = name => ({results:[{title:name,status:'passed'}, {title:name,status:'trapped',message:'refusal'}], initialization:[]});
async function simulate(failure = () => {}, evidence = () => undefined) {
 const calls = [], disposed = [], logs = [], runs = [], results = new Map();
 const get = createSharedBuilder(buildGroups(shards,4), async built => {
  const names = built.group.map(s => s.name).join('');
  calls.push(names);
  failure(names);
  built.evidence = evidence(names);
  built.execute = name => { runs.push(name); return verdict(name); };
 }, built => disposed.push(built.group.map(s => s.name).join('')), line => logs.push(line));
 await runShards(shards,1,async ({name}) => {
  const built = await get(name);
  // Same build-error/report boundary as the real per-shard harness.
  let result;
  try { if (built.error) throw built.error; result = built.execute(name); }
  catch (error) { result = {results:[], initialization:[], infrastructureFailure:String(error)}; }
  assertNoFailures(name,result);
  results.set(name,result);
 });
 return {calls,disposed,logs,runs,results};
}

for (const [stage, error] of [
 ['lowering', Object.assign(new Error('lowering exit 2\ndetail'), {code:2})],
 ['frontend/emit', new RangeError('Maximum call stack size exceeded\nstack')],
 ['go build', Object.assign(new Error('Command failed: go build'), {code:1,stderr:'compiler failed\nsecond line'})],
 ['signal', Object.assign(new Error('build terminated by SIGKILL'), {code:null,signal:'SIGKILL'})],
 ['OOM', Object.assign(new Error('Command failed'), {stderr:'out of memory\nstack'})],
]) {
 test(`${stage}: 4 -> 2+2 -> 1+1 preserves every per-shard verdict`, async () => {
  const baseline = await simulate();
  const split = await simulate(names => { if (['abcd','ab'].includes(names)) throw error; });
  assert.deepEqual(split.calls,['abcd','ab','a','b','cd']);
  assert.deepEqual(split.disposed,['abcd','ab','a','b']);
  assert.deepEqual(split.runs,['a','b','c','d']);
  assert.deepEqual(split.results,baseline.results);
  assert.equal(split.logs.length,2);
  assert.match(split.logs[0],/splitting \[a, b, c, d\] into 2\+2:/);
  assert.match(split.logs[1],/splitting \[a, b\] into 1\+1:/);
  const reason = (error.stderr || error.message).split('\n')[0];
  assert.ok(split.logs.every(line => line.endsWith(reason)));
  assert.ok(split.logs.every(line => !line.includes('\n')));
 });
}

test('persistent singleton build failure stays infrastructure failure, not passes or traps', async () => {
 const error = new Error('persistent lowering failure');
 const split = await simulate(names => { if (names.includes('b')) throw error; });
 assert.deepEqual(split.calls,['abcd','ab','a','b','cd']);
 assert.deepEqual(split.runs,['a','c','d']);
 assert.deepEqual(split.results.get('b'),{results:[],initialization:[],infrastructureFailure:String(error)});
 const baseline = await simulate();
 for (const name of ['a','c','d']) assert.deepEqual(split.results.get(name),baseline.results.get(name));
 assert.equal(split.logs.length,2); // No pointless retry/split of a singleton.
 const dir = mkdtempSync(join(tmpdir(),'autosplit-report-'));
 try {
  const oracle = {total:2,passed:2,failed:0,pending:0,titles:['b','b']};
  writeFileSync(join(dir,'node.json'),JSON.stringify(oracle));
  writeFileSync(join(dir,'run.json'),JSON.stringify(split.results.get('b')));
  execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-report.mjs'),join(dir,'node.json'),join(dir,'run.json'),join(dir,'report.json')]);
  const report = JSON.parse(readFileSync(join(dir,'report.json')));
  assert.equal(report.passed,0);
  assert.equal(report.registered,0);
  assert.equal(report.notRun,2);
  writeFileSync(join(dir,'shards.json'),JSON.stringify({shards,files:shards.flatMap(s => s.roots),excluded:[],partial:true,target:'osg'}));
  for (const shard of shards) {
   writeFileSync(join(dir,`node-${shard.id}.json`),JSON.stringify({...oracle,titles:[shard.name,shard.name]}));
   writeFileSync(join(dir,`go-${shard.id}.json`),JSON.stringify(split.results.get(shard.name)));
   mkdirSync(join(dir,shard.id));
   writeFileSync(join(dir,shard.id,'closure.json'),JSON.stringify({roots:shard.roots,upstreamPin:'fixture'}));
   execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-report.mjs'),join(dir,`node-${shard.id}.json`),join(dir,`go-${shard.id}.json`),join(dir,`report-${shard.id}.json`)]);
  }
  execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-summary.mjs'),dir,join(dir,'summary.json')]);
  const summary = JSON.parse(readFileSync(join(dir,'summary.json')));
  assert.deepEqual(summary.shards.filter(s => s.infrastructureFailure).map(s => [s.name,s.infrastructureFailure]),[['b',String(error)]]);
  assert.equal(summary.uniqueTotal.notRun,2);
  assert.equal(summary.uniqueTotal.passed,3);
  assert.equal(summary.uniqueTotal.trapped,3);
  assert.equal(summary.shards.length,4);
 } finally { rmSync(dir,{recursive:true,force:true}); }
});

test('measured memory budget still splits, and odd groups halve contiguously', async () => {
 const split = await simulate(undefined,names => ({host:{buildPeakRSSKiB:names === 'abcd' ? 41 * 1048576 : 1}}));
 assert.deepEqual(split.calls,['abcd','ab','cd']);
 assert.match(split.logs[0],/build exceeded memory budget/);
 const calls = [];
 const get = createSharedBuilder(buildGroups(shards.slice(0,3),3), async built => {
  calls.push(built.group.map(s => s.name).join(''));
  if (built.group.length > 1) throw new Error('too big');
 }, () => {}, () => {});
 for (const {name} of shards.slice(0,3)) assert.equal((await get(name)).error,undefined);
 assert.deepEqual(calls,['abc','ab','a','b','c']);
});

test('default group size is three; explicit size still allows the full union', () => {
 const plan = ['abap','abap/expressions','abap/lexer','abap/nodes','abap/statements','abap/structures','abap/syntax','abap/types','root','cds','ddl','lsp','objects','objects/rename','pretty_printer','rules','utils'].map(name => ({name}));
 const groups = buildGroups(plan);
 assert.deepEqual(groups.map(g => g.length),[3,3,3,3,3,2]);
 assert.deepEqual(groups.flat(),plan);
 const large = new Set(['abap/statements','abap/syntax','objects','rules']);
 assert.ok(groups.every(g => g.filter(s => large.has(s.name)).length <= 1));
 assert.deepEqual(buildGroups(plan,17),[plan]);
});

test('runtime genuine FAIL propagates without rebuilding or splitting', async () => {
 const calls = [], logs = [];
 const get = createSharedBuilder(buildGroups(shards,4), async built => { calls.push(built.group); }, () => {}, line => logs.push(line));
 await assert.rejects(runShards(shards,1,async ({name}) => {
  await get(name);
  assertNoFailures(name,{results:[{title:name,status:'failed',message:'assertion mismatch'}]});
 }),/STOP: a/);
 assert.equal(calls.length,1);
 assert.deepEqual(logs,[]);
});
