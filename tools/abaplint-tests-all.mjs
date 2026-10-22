#!/usr/bin/env node
// Go: one binary per directory. OSG: one binary per union group, runtime-selected shards.
// Keep syntax together: helpers import
// syntax.ts, which registers tests; splitting its roots would count them twice.
// No source editing, test extraction, or oracle-synthesized Go registrations.
import {readdirSync, readFileSync, writeFileSync, mkdirSync, mkdtempSync, rmSync, copyFileSync, existsSync} from 'node:fs';
import {resolve, join, dirname} from 'node:path';
import {execFile, execFileSync} from 'node:child_process';
import {promisify} from 'node:util';
import {homedir, tmpdir} from 'node:os';
import {pathToFileURL} from 'node:url';
import {materializeRegistryClosure} from './registry-closure.mjs';
import {upstreamPin, buildUpstream} from './statements-upstream.mjs';
import {executeOracle} from './abaplint-tests-oracle.mjs';
import {ownedResults} from './abaplint-tests-owned.mjs';
import {waitForResources, heavyCommand} from './osg-harness-resources.mjs';
import {materializeShardedClosure, buildGroups} from './abaplint-tests-sharded-closure.mjs';

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

// File-focused runs keep dependency closure materialization and the independent
// Node oracle unchanged. In particular, do not extract or synthesize test bodies.
export function selectFiles(plan, selection) {
 if (!selection) return plan;
 const files = selection.split(',').map(file => file.trim());
 if (files.some(file => !file) || new Set(files).size !== files.length) throw new Error('file selection must contain distinct nonempty paths');
 for (const file of files) if (!plan.files.includes(file)) throw new Error(`unknown test file: ${file}`);
 if (files.length === plan.files.length) return plan;
 const selected = new Set(files);
 const shards = plan.shards.map(shard => ({...shard, roots:shard.roots.filter(file => selected.has(file))})).filter(shard => shard.roots.length);
 return {...plan, shards, files:files.sort(), excluded:[...plan.excluded, ...plan.files.filter(file => !selected.has(file))].sort(), partial:true};
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

export function assertRegistrationParity(summary) {
 if (!summary.registrationParity || summary.uniqueTotal.unregistered !== 0) {
  throw new Error(`registration parity failed: ${summary.uniqueTotal.unregistered} unregistered test(s)`);
 }
}

// Drain in-flight work before cleanup, and do not schedule past a genuine FAIL.
export async function runShards(shards, workers, runShard, admit = async () => {}) {
 if (!Number.isInteger(workers) || workers < 1 || workers > 17) throw new Error('ABAPITI_TESTS_WORKERS must be an integer from 1 to 17');
  let next = 0, failure;
  const settled = await Promise.allSettled(Array.from({length:Math.min(workers, shards.length)}, async () => {
   while (!failure && next < shards.length) {
    try {
     await admit();
     if (failure || next >= shards.length) break;
     const shard = shards[next++];
     await runShard(shard);
    } catch (error) { failure = error; throw error; }
   }
  }));
  for (const result of settled) if (result.status === 'rejected') throw result.reason;
}

// Preserve host refusals and timings even when no runner JSON was produced.
export function readOSGEvidence(build) {
 const read = file => existsSync(join(build,file)) ? JSON.parse(readFileSync(join(build,file))) : null;
 return {gap:read('evidence/abap-gap.json'),gate:read('evidence/test-gate.json'),host:read('osg/timing.json'),
  ...(existsSync(join(build,'osg/osg-refusals.json')) ? {refusals:read('osg/osg-refusals.json')} : {})};
}

// Build-only failures are retryable regardless of their diagnostic or exit
// mechanism. Runtime execution is deliberately outside this boundary.
export function createSharedBuilder(groups, build, dispose, log = console.error) {
 let shared;
 async function get(name) {
  const group = groups.find(group => group.some(shard => shard.name === name));
  if (!group) throw new Error(`unknown build shard: ${name}`);
  if (shared?.group === group) return shared;
  if (shared) dispose(shared);
  const index = groups.indexOf(group);
  shared = {group, index, evidence:{gap:null, gate:null, host:null}};
  try { await build(shared); }
  catch (error) { shared.error = error; }
  const rss = shared.evidence?.host?.buildPeakRSSKiB;
  if (group.length > 1 && (shared.error || rss > 40 * 1048576)) {
   // execFile's message starts with the command; prefer the first stderr line
   // when present, retaining messages/signals for failures without stderr.
   const error = shared.error;
   const reason = error
    ? (String(error.stderr ?? '').trim() || error.message || String(error)).split(/\r?\n/)[0]
    : `build exceeded memory budget (${rss} KiB > 40 GiB)`;
   const halves = buildGroups(group, Math.ceil(group.length / 2));
   log(`OSG union build: splitting [${group.map(s => s.name).join(', ')}] into ${halves.map(g => g.length).join('+')}: ${reason}`);
   groups.splice(index, 1, ...halves);
   return get(name);
  }
  return shared;
 }
 return get;
}

async function main() {
 const [upstreamArg = join(homedir(), 'dev/abaplint'), outputArg = 'docs/abaplint-tests-latest.json'] = process.argv.slice(2);
 const upstream = resolve(upstreamArg), output = resolve(outputArg);
 const tools = import.meta.dirname, repository = resolve(tools, '..');
 const scratch = process.env.ABAPITI_TESTS_SCRATCH ?? join(tmpdir(), 'abapiti-test-shards');
 mkdirSync(scratch, {recursive:true});
 const run = mkdtempSync(join(scratch, 'all-'));
 const plan = prioritizeShards(selectFiles(selectShards(discoverShards(upstream), process.env.ABAPITI_TESTS_SHARDS), process.env.ABAPITI_TESTS_FILES), process.env.ABAPITI_TESTS_FIRST);
 const target = process.env.ABAPITI_TESTS_TARGET ?? 'go';
 if (!['go','osg'].includes(target)) throw new Error('ABAPITI_TESTS_TARGET must be go or osg');
 if (target === 'osg' && !process.env.ABAPITI_OSG_HOME) throw new Error('ABAPITI_OSG_HOME is required for osg');
 if (target === 'osg') plan.target = target;
 const targetArgs = target === 'osg' ? ['--target','osg','--osg-home',process.env.ABAPITI_OSG_HOME,'--osg-runner',join(tools,'abaplint-tests-osg.mjs')] : [];
 const oneBuild = target === 'osg' && process.env.ABAPITI_OSG_ONE_BUILD !== '0';
 const separateRules = oneBuild && process.env.ABAPITI_TESTS_RULES_MEMORY_MAX;
 const sharedShards = separateRules ? plan.shards.filter(shard => shard.name !== 'rules') : plan.shards;
 const groups = oneBuild ? buildGroups(sharedShards, process.env.ABAPITI_OSG_SHARDS_PER_BUILD === undefined ? undefined : Number(process.env.ABAPITI_OSG_SHARDS_PER_BUILD)) : [];
 if (separateRules && plan.shards.some(shard => shard.name === 'rules')) groups.unshift(plan.shards.filter(shard => shard.name === 'rules'));
 const env = {...process.env, GOCACHE:process.env.GOCACHE ?? join(process.env.HOME, '.cache/go-build'), GOTOOLCHAIN:'go1.26.0', GOFLAGS:`${process.env.GOFLAGS ?? ''} -p=2 -buildvcs=false`};
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
  let shared;
  const sharedBuild = createSharedBuilder(groups, async built => {
   const {group, index} = built;
   built.closure = join(run, `shared-${index}`);
   built.build = join(run, `shared-${index}-out`);
   try {
    const {manifests} = materializeShardedClosure(built.closure, upstream, group, join(repository,'tsfront/testdata/abaplint-tests/runtime'));
    built.manifests = manifests;
    await waitForResources();
    const buildEnv = {...env, ABAPITI_OSG_BUILD_ONLY:'1', ...(group.some(shard => shard.name === 'rules') && env.ABAPITI_TESTS_RULES_MEMORY_MAX
     ? {ABAPITI_OSG_MEMORY_MAX:env.ABAPITI_TESTS_RULES_MEMORY_MAX,
        ...(env.ABAPITI_TESTS_RULES_NODE_OPTIONS ? {NODE_OPTIONS:env.ABAPITI_TESTS_RULES_NODE_OPTIONS} : {})} : {})};
    const step = heavyCommand(join(run,'abapiti'), ['abaplint-tests', built.closure, '-o', built.build, ...targetArgs], buildEnv);
    await promisify(execFile)(step.command, step.args, {cwd:repository, env:buildEnv, encoding:'utf8', maxBuffer:128*1024*1024});
   } catch (error) { built.error = error; }
   built.evidence = readOSGEvidence(built.build);
  }, built => {
   rmSync(built.closure, {recursive:true, force:true});
   rmSync(built.build, {recursive:true, force:true});
  });
  async function runShard({name, id, roots}) {
   const shardEnv = name === 'rules' && env.ABAPITI_TESTS_RULES_MEMORY_MAX
    ? {...env, ABAPITI_OSG_MEMORY_MAX:env.ABAPITI_TESTS_RULES_MEMORY_MAX,
       ...(env.ABAPITI_TESTS_RULES_NODE_OPTIONS ? {NODE_OPTIONS:env.ABAPITI_TESTS_RULES_NODE_OPTIONS} : {})}
    : env;
   console.error(`=== ${name}: ${roots.length} files ===`);
   const closure = join(run, id), build = join(run, `${id}-out`);
   let manifest, borrowed, foreign;
   let node = oracle(id, roots), result;
   try {
    if (oneBuild) {
     const built = shared = await sharedBuild(name);
     manifest = built.manifests?.get(name);
     if (built.error) throw built.error;
    } else manifest = materializeRegistryClosure(closure, upstream, {roots, runtime:join(repository,'tsfront/testdata/abaplint-tests/runtime')});
    foreign = manifest.sources.map(s => s.file).filter(f => f.startsWith('test/') && !roots.includes(f));
    if (foreign.length) borrowed = executeOracle(oracleBuild, upstream, foreign);
    let execution;
    try {
     const step = oneBuild
      ? heavyCommand(process.execPath, [join(tools,'osg-runner-execute.mjs'), join(shared.build,'osg/abaplint-tests-osg'), join(shared.build,'abap/names.json'), join(build,'osg'), name], shardEnv)
      : heavyCommand(join(run,'abapiti'), ['abaplint-tests', closure, '-o', build, ...targetArgs], shardEnv);
     execution = {...await promisify(execFile)(step.command, step.args, {cwd:repository, env:shardEnv, encoding:'utf8', maxBuffer:128*1024*1024}), status:0};
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
    if (target === 'osg') {
     result.osgEvidence = readOSGEvidence(build);
     if (oneBuild && shared) {
      const host = result.osgEvidence.host;
      result.osgEvidence = {...shared.evidence, host:host ? {...shared.evidence.host, ...host,
       timingMs:{...shared.evidence.host?.timingMs, ...host.timingMs}} : shared.evidence.host};
     }
    }
    if (process.env.ABAPITI_TESTS_KEEP_ARTIFACTS !== '1') {
     rmSync(build, {recursive:true, force:true});
     rmSync(closure, {recursive:true, force:true});
    }
    mkdirSync(closure, {recursive:true});
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
  // One heavy OSG shard at a time, even if Go-target workers were requested.
  // A single generated package can consume many GiB; -p does not bound that.
  if (target === 'osg' && workers > 1) console.error('OSG: serializing heavy shards (ABAPITI_TESTS_WORKERS applies only to the Go target)');
  await runShards(plan.shards, target === 'osg' ? 1 : workers, runShard,
   target === 'osg' || env.ABAPITI_OSG_MEMORY_MAX ? () => waitForResources() : undefined);
  command(process.execPath, [join(tools,'abaplint-tests-summary.mjs'), run, output], {stdio:'inherit'});
  const summary = JSON.parse(readFileSync(output));
  console.log('| Shard | Pass | Fail | Trap | Async-pending | Not-run | Registered | Node |');
  console.log('| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |');
  for (const row of [...summary.shards, {name:'TOTAL', ...summary.uniqueTotal, node:summary.nodeTotal}]) console.log(`| ${row.name} | ${row.passed} | ${row.failed} | ${row.trapped} | ${row.asyncPending} | ${row.notRun} | ${row.registered} | ${row.node.passed + row.node.failed}+${row.node.pending} pending |`);
  console.log(`${summary.nodeActive?.passed ?? summary.uniqueTotal.passed}/${summary.wholeSuite?.active ?? summary.nodeTotal.passed + summary.nodeTotal.failed} (Node-active only); Node total including pending: ${summary.nodeTotal.total}`);
  if (summary.nodeActive) console.log(`Node-active not passing: ${summary.nodeActive.notPassing} = ${summary.nodeActive.trapped} traps + ${summary.nodeActive.unregistered} unregistered + ${summary.nodeActive.skipped} skipped + ${summary.nodeActive.failed} FAIL + ${summary.nodeActive.asyncPending} async-pending; Node-pending outside denominator: ${summary.nodePending.total}`);
  if (summary.shards.some(s => s.infrastructureFailure || s.unmatched)) process.exitCode = 1;
  finished = true;
  assertRegistrationParity(summary);
 } finally {
  oracleBuild?.dispose();
  if (finished && process.env.ABAPITI_TESTS_KEEP_ARTIFACTS !== '1') rmSync(run, {recursive:true, force:true});
  else console.error(`all-shard evidence retained: ${run}`);
 }
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) main().catch(error => { console.error(error); process.exitCode = 1; });
