#!/usr/bin/env node
// Step 4b activation probe: OSG's gogen frontend over gogen-as-ABAP (emit-abap
// output), on a caller-supplied clean OSG checkout at .github/ci/osgo.ref.
// Writes OUT/osg-refusals.json (frontend diagnostics, missing objects, stubs).
// With --build it also emits Go and runs `go build` (empty main) into OUT/go;
// --main makes main call ZCL_ABAPITI_MAIN=>RUN (emit-abap --initialize) and
// report an ABAP exception or Go panic with its host location, exit 3.
// Input assembly and build flags come from osg-harness-input.mjs, shared with
// tools/abaplint-tests-osg.mjs so the two cannot drift.
// Usage: selfhost-step4b-osg.mjs [--build [--main]] ABAP-SRC OUT OSG
import {mkdirSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {performance} from 'node:perf_hooks';
import {osgNodeHeap, osgPinnedCheckout, osgInputFolders, osgFixtureObjects, osgCompileClosure, osgRefusals, osgEmitGo, osgRelease, osgBuildSettings, osgBuildArgs, osgTimedBuild, osgCopyRuntime} from './osg-harness-input.mjs';

osgNodeHeap();
const args = process.argv.slice(2);
const build = args[0] === '--build';
const entry = build && args[1] === '--main';
const [inputArg, outArg, osgArg] = args.slice(Number(build) + Number(entry));
if (!inputArg || !outArg || !osgArg) throw new Error('usage: selfhost-step4b-osg.mjs [--build [--main]] ABAP-SRC OUT OSG');
const input = resolve(inputArg), out = resolve(outArg), osg = resolve(osgArg);
const pin = osgPinnedCheckout(osg);
if (out.startsWith(osg + '/')) throw new Error('OUT must be outside the OSG checkout');
mkdirSync(out, {recursive: true});
const tool = (name) => import(pathToFileURL(join(osg, 'tools/gogen', name)));
const settings = osgBuildSettings();
const {folders, skip} = await osgInputFolders({tool, home: osg, fixture: input});
const {objects} = osgFixtureObjects(input);
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
const closure = await osgCompileClosure({tool, folders, skip, objects});
timing.frontendMs = Math.round(performance.now() - started);
const refusals = {objects: objects.length, classes: closure.program.classes.length, ...osgRefusals(closure.program)};
writeFileSync(join(out, 'osg-refusals.json'), JSON.stringify(refusals, null, 1) + '\n');
console.log(`osg-frontend: ${objects.length} objects, ${closure.program.classes.length} classes, ${(closure.program.diagnostics ?? []).length} diagnostics, ${refusals.missing.length} missing, ${refusals.stubs.length} stubs, ${timing.frontendMs} ms`);
if (build) {
  const go = join(out, 'go'), cmd = join(go, 'cmd', 'step4b');
  osgCopyRuntime(osg, go);
  mkdirSync(cmd, {recursive: true});
  started = performance.now();
  writeFileSync(join(cmd, 'zz_generated.go'), await osgEmitGo({tool, program: closure.program}));
  writeFileSync(join(cmd, 'main.go'), entry ? entryMain : 'package main\nfunc main(){}\n');
  timing.emitMs = Math.round(performance.now() - started);
  osgRelease(closure);
  started = performance.now();
  const {build: result, buildPeakRSSKiB} = osgTimedBuild({go, out, cmdName: 'step4b', args: osgBuildArgs('step4b', settings), binary: join(out, 'step4b-osg')});
  timing.goBuildMs = Math.round(performance.now() - started);
  writeFileSync(join(out, 'build.log'), (result.stdout ?? '') + (result.stderr ?? ''));
  console.log(`osg-build: exit ${result.status}, ${timing.goBuildMs} ms, peak RSS ${buildPeakRSSKiB === null ? 'n/a' : Math.round(buildPeakRSSKiB / 1048576 * 10) / 10 + ' GiB'} (build.log)`);
  process.exitCode = result.status === 0 ? 0 : 2;
  timing.buildPeakRSSKiB = buildPeakRSSKiB;
}
writeFileSync(join(out, 'timing.json'), JSON.stringify({pin, ...timing}, null, 1) + '\n');
