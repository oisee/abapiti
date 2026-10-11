#!/usr/bin/env node
// One binary per direct test directory. Keep syntax together: helpers import
// syntax.ts, which registers tests; splitting its roots would count them twice.
// No source editing, test extraction, or oracle-synthesized Go registrations.
import {readdirSync, readFileSync, writeFileSync, mkdirSync, mkdtempSync, rmSync, copyFileSync} from 'node:fs';
import {resolve, join, dirname} from 'node:path';
import {execFile, execFileSync} from 'node:child_process';
import {promisify} from 'node:util';
import {homedir, tmpdir} from 'node:os';
import {pathToFileURL} from 'node:url';
import {materializeRegistryClosure} from './registry-closure.mjs';
import {upstreamPin, buildUpstream} from './statements-upstream.mjs';
import {executeOracle} from './abaplint-tests-oracle.mjs';
import {ownedResults} from './abaplint-tests-owned.mjs';

export function discoverShards(upstream) {
 const root = join(upstream, 'packages/core');
 const files = [];
 function walk(dir) {
  for (const entry of readdirSync(join(root, dir), {withFileTypes:true}).sort((a,b) => a.name.localeCompare(b.name))) {
   const path = `${dir}/${entry.name}`;
   if (entry.isDirectory()) walk(path);
   else if (entry.isFile() && entry.name.endsWith('.ts')) files.push(path);
  }
 }
 walk('test'); files.sort();
 const groups = new Map();
 for (const file of files) {
  const name = dirname(file).replace(/^test\/?/, '') || 'root';
  const roots = groups.get(name) ?? []; roots.push(file); groups.set(name, roots);
 }
 return {upstreamPin, files, shards:[...groups].map(([name, roots]) => ({name, id:name.replaceAll('/', '-'), roots})), excluded:[]};
}

// Focused measurements retain the same ownership and FAIL checks as full runs.
export function selectShards(plan, selection) {
 if (!selection) return plan;
 const names = selection.split(',').map(name => name.trim());
 if (names.some(name => !name) || new Set(names).size !== names.length) throw new Error('shard selection must contain distinct nonempty names');
 for (const name of names) if (!plan.shards.some(shard => shard.name === name)) throw new Error(`unknown test shard: ${name}`);
 const shards = plan.shards.filter(shard => names.includes(shard.name));
 if (shards.length === plan.shards.length) return plan;
 const files = shards.flatMap(shard => shard.roots).sort();
 const selected = new Set(files);
 return {...plan, shards, files, excluded:plan.files.filter(file => !selected.has(file)), partial:true};
}

// Execute selected high-risk shards first without excluding roots or changing
// whole-suite accounting. Useful when owned repairs must fail fast before gates.
export function prioritizeShards(plan, selection) {
 if (!selection) return plan;
 selectShards(plan, selection); // shared validation, including duplicate/unknown names
 const names = selection.split(',').map(name => name.trim());
 const first = names.map(name => plan.shards.find(shard => shard.name === name));
 return {...plan, shards:[...first, ...plan.shards.filter(shard => !names.includes(shard.name))]};
}

// Never continue trap work past an assertion mismatch (or other genuine
// failed throw). Runtime/lowering refusals already have typed trap statuses.
export function assertNoFailures(name, run) {
 const failures = [...run.results, ...(run.initialization ?? [])].filter(r => r.status === 'failed');
 if (failures.length) throw new Error(`STOP: ${name}: ${failures.length} genuine FAIL(s)\n${JSON.stringify(failures, null, 2)}`);
}

// Drain in-flight work before cleanup, and do not schedule past a genuine FAIL.
export async function runShards(shards, workers, runShard) {
 if (!Number.isInteger(workers) || workers < 1 || workers > 17) throw new Error('ABAPITI_TESTS_WORKERS must be an integer from 1 to 17');
  let next = 0, failure;
  const settled = await Promise.allSettled(Array.from({length:Math.min(workers, shards.length)}, async () => {
   while (!failure && next < shards.length) {
    const shard = shards[next++];
    try { await runShard(shard); } catch (error) { failure = error; throw error; }
   }
  }));
  for (const result of settled) if (result.status === 'rejected') throw result.reason;
}

async function main() {
 const [upstreamArg = join(homedir(), 'dev/abaplint'), outputArg = 'docs/abaplint-tests-latest.json'] = process.argv.slice(2);
 const upstream = resolve(upstreamArg), output = resolve(outputArg);
 const tools = import.meta.dirname, repository = resolve(tools, '..');
 const scratch = process.env.ABAPITI_TESTS_SCRATCH ?? join(tmpdir(), 'abapiti-test-shards');
 mkdirSync(scratch, {recursive:true});
 const run = mkdtempSync(join(scratch, 'all-'));
 const plan = prioritizeShards(selectShards(discoverShards(upstream), process.env.ABAPITI_TESTS_SHARDS), process.env.ABAPITI_TESTS_FIRST);
 const env = {...process.env, GOCACHE:join(process.env.HOME, '.cache/go-build'), GOTOOLCHAIN:'go1.26.0', GOFLAGS:`${process.env.GOFLAGS ?? ''} -p=2 -buildvcs=false`};
 function command(command, args, options = {}) {
  return execFileSync(command, args, {cwd:repository, env, maxBuffer:128*1024*1024, ...options});
 }
 const compiler = process.env.ABAPITI_TESTS_COMPILER;
 if (compiler) {
  const revision = process.env.ABAPITI_TESTS_COMPILER_COMMIT;
  if (!revision) throw new Error('ABAPITI_TESTS_COMPILER requires ABAPITI_TESTS_COMPILER_COMMIT');
  plan.compilerCommit = command('git', ['rev-parse', '--verify', `${revision}^{commit}`], {encoding:'utf8'}).trim();
 }
 let oracleBuild, finished = false;
 function oracle(id, roots) {
  const result = executeOracle(oracleBuild, upstream, roots);
  writeFileSync(join(run,`node-${id}.json`), JSON.stringify(result));
  return result;
 }
 try {
  oracleBuild = buildUpstream(upstream);
  writeFileSync(join(run,'shards.json'), JSON.stringify(plan));
  // Independent whole-suite run detects cross-shard omissions and duplicate roots.
  if (!plan.partial) oracle('all', plan.files);
  if (compiler) copyFileSync(resolve(compiler), join(run,'abapiti'));
  else command('go', ['build', '-o', join(run,'abapiti'), './cmd/abapiti'], {stdio:'inherit'});
  async function runShard({name, id, roots}) {
   console.error(`=== ${name}: ${roots.length} files ===`);
   const closure = join(run, id), build = join(run, `${id}-out`);
   let manifest, borrowed, foreign;
   let node = oracle(id, roots), result;
   try {
    manifest = materializeRegistryClosure(closure, upstream, {roots, runtime:join(repository,'tsfront/testdata/abaplint-tests/runtime')});
    foreign = manifest.sources.map(s => s.file).filter(f => f.startsWith('test/') && !roots.includes(f));
    if (foreign.length) borrowed = executeOracle(oracleBuild, upstream, foreign);
    let execution;
    try {
     execution = {...await promisify(execFile)(join(run,'abapiti'), ['abaplint-tests', closure, '-o', build], {cwd:repository, env, encoding:'utf8', maxBuffer:128*1024*1024}), status:0};
    } catch (error) {
     execution = {stdout:error.stdout ?? '', stderr:error.stderr ?? '', status:error.code, error};
    }
    // Never manufacture registrations if materialization, emission, or Go fails.
    try { result = JSON.parse(execution.stdout); if (!Array.isArray(result.results)) throw new Error('missing results'); }
    catch {
     throw new Error(`harness exit ${execution.status}: ${execution.error?.message ?? ''}\n${execution.stderr?.slice(-6000) ?? ''}`);
    }
    if (![0,1].includes(execution.status)) throw new Error(`unexpected harness exit ${execution.status}`);
   } catch (error) {
    // Infrastructure failure is explicit and all oracle cases remain not-run.
    result = {results:[], initialization:[], infrastructureFailure:String(error)};
    console.error(String(error));
   } finally {
    rmSync(build, {recursive:true, force:true});
    rmSync(closure, {recursive:true, force:true});
    mkdirSync(closure);
    writeFileSync(join(closure,'closure.json'), JSON.stringify({roots, upstreamPin:manifest?.upstreamPin ?? upstreamPin}));
   }
   // Borrowed test bodies are real executions too: never hide their FAILs
   // while restricting the accounting to this shard's owned titles.
   assertNoFailures(name, result);
   if (borrowed) ({oracle:node, run:result} = ownedResults(node, result, borrowed, foreign));
   writeFileSync(join(run,`node-${id}.json`), JSON.stringify(node));
   writeFileSync(join(run,`go-${id}.json`), JSON.stringify(result));
   command(process.execPath, [join(tools,'abaplint-tests-report.mjs'), join(run,`node-${id}.json`), join(run,`go-${id}.json`), join(run,`report-${id}.json`)], {stdio:['ignore','ignore','inherit']});
   assertNoFailures(name, result);
  }
  const workers = Number(process.env.ABAPITI_TESTS_WORKERS ?? 1);
  if (!Number.isInteger(workers) || workers < 1 || workers > 17) throw new Error('ABAPITI_TESTS_WORKERS must be an integer from 1 to 17');
  await runShards(plan.shards, workers, runShard);
  command(process.execPath, [join(tools,'abaplint-tests-summary.mjs'), run, output], {stdio:'inherit'});
  const summary = JSON.parse(readFileSync(output));
  console.log('| Shard | Pass | Fail | Trap | Async-pending | Not-run | Registered | Node |');
  console.log('| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |');
  for (const row of [...summary.shards, {name:'TOTAL', ...summary.uniqueTotal, node:summary.nodeTotal}]) console.log(`| ${row.name} | ${row.passed} | ${row.failed} | ${row.trapped} | ${row.asyncPending} | ${row.notRun} | ${row.registered} | ${row.node.passed + row.node.failed}+${row.node.pending} pending |`);
  console.log(`${summary.uniqueTotal.passed}/${summary.wholeSuite?.active ?? summary.nodeTotal.passed + summary.nodeTotal.failed}; Node total including pending: ${summary.nodeTotal.total}`);
  if (summary.shards.some(s => s.infrastructureFailure || s.unmatched)) process.exitCode = 1;
  finished = true;
 } finally {
  oracleBuild?.dispose();
  if (finished) rmSync(run, {recursive:true, force:true});
  else console.error(`all-shard evidence retained after incomplete run: ${run}`);
 }
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) main().catch(error => { console.error(error); process.exitCode = 1; });
