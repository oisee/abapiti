#!/usr/bin/env node
// Step 4b activation probe: OSG's gogen frontend over gogen-as-ABAP (emit-abap
// output), on a caller-supplied clean OSG checkout at .github/ci/osgo.ref.
// Writes OUT/osg-refusals.json (frontend diagnostics, missing objects, stubs).
// With --build it also emits Go and runs `go build` (empty main) into OUT/go;
// --main makes main call ZCL_ABAPITI_MAIN=>RUN (emit-abap --initialize) and
// report an ABAP exception or Go panic with its host location, exit 3.
// Usage: selfhost-step4b-osg.mjs [--build [--main]] ABAP-SRC OUT OSG
import {cpSync, mkdirSync, readFileSync, readdirSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {execFileSync, spawnSync} from 'node:child_process';
import {performance} from 'node:perf_hooks';

const args = process.argv.slice(2);
const build = args[0] === '--build';
const entry = build && args[1] === '--main';
const [inputArg, outArg, osgArg] = args.slice(Number(build) + Number(entry));
if (!inputArg || !outArg || !osgArg) throw new Error('usage: selfhost-step4b-osg.mjs [--build [--main]] ABAP-SRC OUT OSG');
const input = resolve(inputArg), out = resolve(outArg), osg = resolve(osgArg);
const pin = readFileSync(new URL('../.github/ci/osgo.ref', import.meta.url), 'utf8').trim();
const git = (...a) => execFileSync('git', ['-C', osg, ...a], {encoding: 'utf8'}).trim();
if (git('rev-parse', 'HEAD') !== pin || git('status', '--porcelain')) throw new Error('OSG must be clean at ' + pin);
if (out.startsWith(osg + '/')) throw new Error('OUT must be outside the OSG checkout');
mkdirSync(out, {recursive: true});
const tool = (name) => import(pathToFileURL(join(osg, 'tools/gogen', name)));
const {unitInputs} = await tool('unit-inputs.mjs');
const config = JSON.parse(readFileSync(join(osg, 'abap_transpile.json'), 'utf8'));
const {folders, skip} = unitInputs({home: osg, config, fixture: input});
const {compileProgram} = await tool('frontend.mjs');
const {emitGo, referencedClasses} = await tool('emit-go.mjs');
const objects = readdirSync(input).filter(f => /\.(clas|intf)\.abap$/.test(f)).map(f => f.split('.')[0].toUpperCase());
const timing = {};
const entryMain = `package main
import("fmt";"os";"runtime";"strings";"osg/gogen/abap")
func hostLocation() string {
 pcs:=make([]uintptr,256);n:=runtime.Callers(3,pcs)
 frames:=runtime.CallersFrames(pcs[:n])
 for {f,more:=frames.Next()
  if !strings.HasPrefix(f.Function,"runtime.") {return fmt.Sprintf("%s:%d",f.File,f.Line)}
  if !more {return "unknown host location"}
 }
}
func main(){
 defer func(){if x:=recover();x!=nil {fmt.Fprintf(os.Stderr,"step4b: run failed: %s (at %s)\\n",strings.SplitN(fmt.Sprint(x),"\\n",2)[0],hostLocation());os.Exit(3)}}()
 ZCL_ABAPITI_MAIN_RUN(&abap.Session{})
}
`;
let started = performance.now();
// Same settle loop as tools/abaplint-tests-osg.mjs: add referenced library classes.
const walk = dir => readdirSync(dir, {withFileTypes: true}).flatMap(e => e.isDirectory() ? walk(join(dir, e.name)) : [join(dir, e.name)]);
const sources = new Map(folders.flatMap(walk).filter(f => !skip(f) && /\.clas\.abap$/.test(f)).map(f => [f.split('/').at(-1).split('.')[0].replaceAll('#', '/').toUpperCase(), f]));
// Lexical closure and superclasses as in tools/abaplint-tests-osg.mjs: the
// native exception bases (CX_ROOT, CX_STATIC_CHECK, ...) are library classes.
const wanted = new Set(objects), queue = [...objects];
const own = name => join(input, readdirSync(input).find(f => f.split('.')[0].toUpperCase() === name && /\.clas\.abap$/.test(f)) ?? '');
while (queue.length) {
  const name = queue.shift(), file = sources.get(name) ?? (objects.includes(name) ? own(name) : undefined);
  if (!file || !/\.abap$/.test(file)) continue;
  for (const match of readFileSync(file, 'utf8').matchAll(/\b(?:CL|CX|ZCL|ZCX)_[A-Z0-9_]+\b/gi)) {
    const dep = match[0].toUpperCase();
    if (sources.has(dep) && !wanted.has(dep)) { wanted.add(dep); queue.push(dep); }
  }
}
let program, registry;
const session = {};
for (let round = 0; round < 12; round++) {
  program = compileProgram({folders, objects: [...wanted], tolerant: true, skip, registry, session});
  registry = program.reg;
  const refs = new Set([...referencedClasses(program), ...program.missing]);
  for (const name of wanted) {
    const sup = registry.getObject('CLAS', name)?.getDefinition()?.getSuperClass();
    if (sup) refs.add(sup.toUpperCase());
  }
  const more = [...refs].filter(name => sources.has(name) && !wanted.has(name));
  if (!more.length) break;
  for (const name of more) wanted.add(name);
}
timing.frontendMs = Math.round(performance.now() - started);
const stubs = program.classes.flatMap(c => (c.stubs ?? []).map(m => ({class: c.name, method: m.name, reason: m.reason})));
const refusals = {objects: objects.length, classes: program.classes.length, diagnostics: program.diagnostics, skipped: program.skipped, missing: [...program.missing], stubs};
writeFileSync(join(out, 'osg-refusals.json'), JSON.stringify(refusals, null, 1) + '\n');
console.log(`osg-frontend: ${objects.length} objects, ${program.classes.length} classes, ${(program.diagnostics ?? []).length} diagnostics, ${refusals.missing.length} missing, ${stubs.length} stubs, ${timing.frontendMs} ms`);
if (build) {
  const go = join(out, 'go'), cmd = join(go, 'cmd', 'step4b');
  cpSync(join(osg, 'tools/gogen/go'), go, {recursive: true, filter: src => !/(?:^|\/)zz_[^/]+\.go$/.test(src)});
  mkdirSync(cmd, {recursive: true});
  started = performance.now();
  writeFileSync(join(cmd, 'zz_generated.go'), emitGo(program, 'main', null, true));
  writeFileSync(join(cmd, 'main.go'), entry ? entryMain : 'package main\nfunc main(){}\n');
  timing.emitMs = Math.round(performance.now() - started);
  program = null; registry = null;
  started = performance.now();
  const result = spawnSync('go', ['build', '-p', '1', '-trimpath', '-gcflags=-N -l', '-o', join(out, 'step4b-osg'), './cmd/step4b'], {cwd: go, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024});
  timing.goBuildMs = Math.round(performance.now() - started);
  writeFileSync(join(out, 'build.log'), (result.stdout ?? '') + (result.stderr ?? ''));
  console.log(`osg-build: exit ${result.status}, ${timing.goBuildMs} ms (build.log)`);
  process.exitCode = result.status === 0 ? 0 : 2;
}
writeFileSync(join(out, 'timing.json'), JSON.stringify({pin, ...timing}, null, 1) + '\n');
