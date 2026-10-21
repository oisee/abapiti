// The OSG input assembly every bridge shares: unit folders over a pinned OSG
// checkout, the dependency closure including native exception bases, tolerant
// refusals, and the Go build with parity flags. Single-sourced for
// abaplint-tests-osg.mjs and selfhost-step4b-osg.mjs so the two cannot drift:
// a rules shard built fine in the harness but failed `go build` in the bridge
// because the bridge's closure missed superclasses, which left runtime
// exception constructors without their inherited TEXT/PREVIOUS parameters
// (2026-10-21).
import {cpSync, existsSync, readFileSync, readdirSync} from 'node:fs';
import {join} from 'node:path';
import {execFileSync, spawnSync} from 'node:child_process';

// The TS-HA driver always starts the runner with these flags (cmd/abapiti/
// abaplint_tests_osg.go); a bridge started as `node tools/...mjs` re-execs
// itself once to match. --expose-gc: without it global.gc() is undefined, and
// the IR stays resident through the memory-heavy Go build (a 16G cgroup then
// OOMs, kernel log 2026-10-21). --max-old-space-size: the frontend peaks near
// 11 GiB on a 4k-object shard, above Node's ~4 GB default. Both change
// capacity, never the output; NODE_OPTIONS counts as already set.
export function osgNodeHeap(ceiling = 12288) {
  const has = flag => process.execArgv.some(arg => arg.includes(flag)) || (process.env.NODE_OPTIONS ?? '').includes(flag);
  if (has('expose-gc') && has('max-old-space-size')) return;
  const [entry, ...args] = process.argv;
  const flags = [];
  if (!has('expose-gc')) flags.push('--expose-gc');
  if (!has('max-old-space-size')) flags.push(`--max-old-space-size=${ceiling}`);
  process.exit(spawnSync(entry, [...flags, ...args], {stdio: 'inherit'}).status ?? 1);
}

// Read-only use of a caller-supplied clean OSG checkout at .github/ci/osgo.ref.
export function osgPinnedCheckout(osg) {
  const pin = readFileSync(new URL('../.github/ci/osgo.ref', import.meta.url), 'utf8').trim();
  const git = (...args) => execFileSync('git', ['-C', osg, ...args], {encoding: 'utf8'}).trim();
  if (git('rev-parse', 'HEAD') !== pin || git('status', '--porcelain')) throw new Error('OSG must be clean at ' + pin);
  return pin;
}

export async function osgInputFolders({tool, home, fixture}) {
  const {unitInputs} = await tool('unit-inputs.mjs');
  const config = JSON.parse(readFileSync(join(home, 'abap_transpile.json'), 'utf8'));
  return unitInputs({home, config, fixture});
}

export function osgFixtureObjects(input) {
  const abapFiles = readdirSync(input).filter(f => /\.(clas|intf)\.abap$/.test(f));
  return {abapFiles, objects: abapFiles.map(f => f.split('.')[0].toUpperCase())};
}

// Match unit.mjs's dependency closure, including native exception bases: the
// text-level class references of every selected object, then the frontend
// closure (referenced classes, missing objects - native exception bases arrive
// there when an exception widens to a ref - and superclasses, whose compiled
// definitions carry inherited constructor parameters), settled in 12 rounds.
export async function osgCompileClosure({tool, folders, skip, objects, compileOptions = {}}) {
  const {compileProgram} = await tool('frontend.mjs');
  const {referencedClasses} = await tool('emit-go.mjs');
  const walk = dir => readdirSync(dir, {withFileTypes: true}).flatMap(e => e.isDirectory() ? walk(join(dir, e.name)) : [join(dir, e.name)]);
  const sources = new Map(folders.flatMap(walk).filter(f => !skip(f) && /\.clas\.abap$/.test(f)).map(f => [f.split('/').at(-1).split('.')[0].replaceAll('#', '/').toUpperCase(), f]));
  const wanted = new Set(objects), queue = [...objects];
  while (queue.length) {
    const file = sources.get(queue.shift());
    if (!file) continue;
    for (const match of readFileSync(file, 'utf8').matchAll(/\b(?:CL|CX|ZCL|ZCX)_[A-Z0-9_]+\b/gi)) {
      const name = match[0].toUpperCase();
      if (sources.has(name) && !wanted.has(name)) { wanted.add(name); queue.push(name); }
    }
  }
  let program, registry;
  const session = {};
  for (let round = 0; round < 12; round++) {
    program = compileProgram({folders, objects: [...wanted], tolerant: true, skip, registry, session, ...compileOptions});
    registry = program.reg;
    const refs = new Set([...referencedClasses(program), ...program.missing]);
    for (const name of wanted) { const sup = registry.getObject('CLAS', name)?.getDefinition()?.getSuperClass(); if (sup) refs.add(sup.toUpperCase()); }
    const more = [...refs].filter(name => sources.has(name) && !wanted.has(name));
    if (!more.length) break;
    for (const name of more) wanted.add(name);
    if (round === 11) throw new Error('OSG dependency closure did not settle');
  }
  return {program, registry, session, wanted, sources};
}

// Tolerant stubs remain named runtime refusals.
export function osgRefusals(program) {
  return {diagnostics: program.diagnostics, skipped: program.skipped, missing: [...program.missing],
    stubs: program.classes.flatMap(c => (c.stubs ?? []).map(m => ({class: c.name, method: m.name, reason: m.reason})))};
}

export async function osgEmitGo({tool, program, transform = go => go}) {
  const {emitGo} = await tool('emit-go.mjs');
  return transform(emitGo(program, 'main', null, true));
}

// Release gogen's registry/IR before the memory-heavy Go compiler starts.
// Takes the state osgCompileClosure returned: nulling its fields drops the
// last live references (program holds registry; both must go, a const
// destructuring would keep them alive through the whole build).
export function osgRelease(state) {
  state.program = null; state.registry = null;
  for (const name of Object.keys(state.session)) delete state.session[name];
  global.gc?.();
}

// Build flags change compile time and memory only; the identity gate in
// docs/osg-harness-speed.md compares full title/status multisets per shard.
// ABAPITI_OSG_DEBUG=1 keeps DWARF, symbols and the .abap //line directives
// (otherwise panics point into generated Go, not .abap). gogen reads
// GOGEN_NOLINE when emit-go.mjs is imported; call this first.
export function osgBuildSettings(env = process.env) {
  const debug = env.ABAPITI_OSG_DEBUG === '1';
  if (!debug && env.GOGEN_NOLINE === undefined) env.GOGEN_NOLINE = '1';
  const mode = env.ABAPITI_OSG_BUILD_MODE ?? 'fast';
  if (!['default', 'fast'].includes(mode)) throw new Error('ABAPITI_OSG_BUILD_MODE must be default or fast');
  const gcflags = [...(mode === 'fast' ? ['-N', '-l'] : []), ...(debug ? [] : ['-dwarf=false'])];
  return {debug, mode, gcflags};
}

export function osgBuildArgs(cmdName, {gcflags, debug}) {
  return ['build', '-p', '1', '-trimpath', ...(debug ? [] : ['-ldflags=-s -w']),
    ...(gcflags.length ? ['-gcflags=osg/gogen/cmd/' + cmdName + '=' + gcflags.join(' ')] : [])];
}

// GOGC/GOMEMLIMIT stay at Go defaults: a 12 GiB soft limit made the objects
// compile thrash (1,336 s vs 496 s) and GOGC=50 cost 38 % time there.
// GNU time measures the heaviest build child, not just the small Go driver.
export function osgTimedBuild({go, out, cmdName, args, binary}) {
  const rssFile = join(out, 'build-rss.txt');
  const timed = existsSync('/usr/bin/time');
  const build = spawnSync(timed ? '/usr/bin/time' : 'go',
    timed ? ['-v', '-o', rssFile, 'go', ...args, '-o', binary, './cmd/' + cmdName] : [...args, '-o', binary, './cmd/' + cmdName],
    {cwd: go, env: {...process.env, GOMAXPROCS: process.env.GOMAXPROCS ?? '8'}, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024});
  let buildPeakRSSKiB = null;
  if (timed && existsSync(rssFile)) {
    const match = readFileSync(rssFile, 'utf8').match(/Maximum resident set size \(kbytes\):\s*(\d+)/);
    if (match) buildPeakRSSKiB = Number(match[1]);
  }
  return {build, buildPeakRSSKiB};
}

export function osgCopyRuntime(osg, go) {
  cpSync(join(osg, 'tools/gogen/go'), go, {recursive: true, filter: src => !/(?:^|\/)zz_[^/]+\.go$/.test(src)});
}
