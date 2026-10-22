#!/usr/bin/env node
// Read-only use of a caller-supplied pinned OSG checkout. Run under the shared
// OSG flock; all snapshots, frontend output, Go modules and binaries are external.
import {cpSync, mkdirSync, readFileSync, readdirSync, writeFileSync, existsSync} from 'node:fs';
import {join, resolve, dirname} from 'node:path';
import {pathToFileURL} from 'node:url';
import {createRequire} from 'node:module';
import {execFileSync, spawnSync} from 'node:child_process';
import {performance} from 'node:perf_hooks';
import {snapshotKey, readSnapshot, writeSnapshot, externalPath} from './osg-harness-cache.mjs';
import {waitForResources} from './osg-harness-resources.mjs';
import {resumeRunner} from './osg-runner-resume.mjs';

const [inputArg, outArg, osgArg] = process.argv.slice(2);
if (!inputArg || !outArg || !osgArg) throw new Error('usage: abaplint-tests-osg.mjs ABAP OUT OSG');
const input=resolve(inputArg), osg=resolve(osgArg), out=externalPath(outArg,osg);
const cacheRoot=process.env.ABAPITI_OSG_CACHE ? externalPath(process.env.ABAPITI_OSG_CACHE,osg) : null;
const pin=readFileSync(new URL('../.github/ci/osgo.ref',import.meta.url),'utf8').trim();
const git=(...args)=>execFileSync('git',['-C',osg,...args],{encoding:'utf8'}).trim();
if (git('rev-parse','HEAD')!==pin || git('status','--porcelain')) throw new Error('OSG must be clean at '+pin);
const admission=await waitForResources();
const tool=(name)=>import(pathToFileURL(join(osg,'tools/gogen',name)));
const {unitInputs}=await tool('unit-inputs.mjs');
const config=JSON.parse(readFileSync(join(osg,'abap_transpile.json'),'utf8'));
const {folders,skip}=unitInputs({home:osg,config,fixture:input});
for (const folder of folders) {
 externalPath(out,folder);
 if (cacheRoot) externalPath(cacheRoot,folder);
}
mkdirSync(out,{recursive:true});
const abapFiles=readdirSync(input).filter(f=>/\.(clas|intf)\.abap$/.test(f));
const envelope={files:abapFiles.length,maxLineBytes:0,overlongLines:0,maxObjectNameBytes:0};
for (const file of abapFiles) {
 for (const line of readFileSync(join(input,file),'utf8').split('\n')) {
  const bytes=Buffer.byteLength(line);
  envelope.maxLineBytes=Math.max(envelope.maxLineBytes,bytes);if(bytes>255)envelope.overlongLines++;
 }
 envelope.maxObjectNameBytes=Math.max(envelope.maxObjectNameBytes,Buffer.byteLength(file.split('.')[0]));
}
if(envelope.overlongLines || envelope.maxObjectNameBytes>30)throw new Error('ABAP envelope violation: '+JSON.stringify(envelope));
const timingMs={cache:0,frontend:0,emit:0,release:0,goBuild:0,run:0};
// Build flags change compile time and memory only; the identity gate in
// docs/osg-harness-speed.md compares full title/status multisets per shard.
// ABAPITI_OSG_DEBUG=1 keeps DWARF, symbols and the .abap //line directives
// (otherwise panics point into generated Go, not .abap).
const debug=process.env.ABAPITI_OSG_DEBUG==='1';
// gogen reads GOGEN_NOLINE when emit-go.mjs is imported; set it first.
if (!debug && process.env.GOGEN_NOLINE===undefined) process.env.GOGEN_NOLINE='1';
const mode=process.env.ABAPITI_OSG_BUILD_MODE ?? 'fast';
if (!['default','fast'].includes(mode)) throw new Error('ABAPITI_OSG_BUILD_MODE must be default or fast');
const gcflags=[...(mode==='fast'?['-N','-l']:[]),...(debug?[]:['-dwarf=false'])];
const buildArgs=['build','-p','1','-trimpath',...(debug?[]:['-ldflags=-s -w']),...(gcflags.length?['-gcflags=osg/gogen/cmd/abapitests='+gcflags.join(' ')]:[])];
// GOGC/GOMEMLIMIT stay at Go defaults: a 12 GiB soft limit made the objects
// compile thrash (1,336 s vs 496 s) and GOGC=50 cost 38 % time there.
const buildEnv={...process.env,GOMAXPROCS:process.env.GOMAXPROCS ?? '8'};
const go=join(out,'go'), cmd=join(go,'cmd','abapitests'), binary=join(out,'abaplint-tests-osg');
const generated={'zz_generated.go':join(cmd,'zz_generated.go'),'main.go':join(cmd,'main.go')};
let frontKey, binaryKey, refusals, regexAdapter, buildPeakRSSKiB=null, frontendPeakRSSKiB=null;
const cache={frontend:cacheRoot?'miss':'disabled',binary:cacheRoot?'miss':'disabled'};
let started=performance.now();
if (cacheRoot) {
 const require=createRequire(join(osg,'tools/gogen/frontend.mjs'));
 const transpiler=require.resolve('@abaplint/transpiler/package.json');
 const core=createRequire(transpiler).resolve('@abaplint/core/package.json');
 // Hash bytes, not mtime or scratch paths: every registry-visible file
 // (including new/omitted closure members), gogen, its Go runtime and toolchain.
 const trees=[...folders,join(osg,'tools'),join(osg,'node_modules'),dirname(transpiler),dirname(core)];
 const files=['abap_transpile.json','package-lock.json','libs.lock.json'].map(f=>join(osg,f)).filter(existsSync);
 frontKey=snapshotKey([...trees,...files],{
  kind:'frontend',pin,node:process.version,
  gogenEnv:Object.fromEntries(Object.entries(process.env).filter(([k])=>k.startsWith('GOGEN_')).sort()),
  bridge:readFileSync(new URL(import.meta.url),'utf8'),
  frontendBridge:readFileSync(new URL('./abaplint-tests-osg-frontend.mjs',import.meta.url),'utf8'),
  cache:readFileSync(new URL('./osg-harness-cache.mjs',import.meta.url),'utf8'),
  resume:readFileSync(new URL('./osg-runner-resume.mjs',import.meta.url),'utf8'),
  main:readFileSync(new URL('./osg-runner-main.mjs',import.meta.url),'utf8'),
  // Invocation-local IR/Go adaptations change the emitted frontend output.
  boundaries:readFileSync(new URL('./osgo-test-boundaries.mjs',import.meta.url),'utf8'),
  regex:readFileSync(new URL('./osgo-test-regex.mjs',import.meta.url),'utf8'),
  // The path-dependent exclusion predicate must remain identical too.
  skipped:folders.map(folder=>{
   const walk=dir=>readdirSync(dir,{withFileTypes:true}).flatMap(e=>e.isDirectory()?walk(join(dir,e.name)):[join(dir,e.name)]);
   return walk(folder).map(file=>[file.slice(folder.length+1),skip(file)]).sort();
  }),
 });
 binaryKey=snapshotKey([],{kind:'binary',frontKey,buildArgs,goFlags:process.env.GOFLAGS ?? '',
  go:execFileSync('go',['env','GOVERSION','GOOS','GOARCH','GOAMD64','GOARM','GOARM64','GO386','GOMIPS','GOMIPS64','GOPPC64','GORISCV64','GOEXPERIMENT','CGO_ENABLED','CC','CXX','CGO_CFLAGS','CGO_CPPFLAGS','CGO_CXXFLAGS','CGO_LDFLAGS'],{encoding:'utf8'})});
 const hit=readSnapshot(cacheRoot,binaryKey,{binary});
 if (hit) { ({refusals,regexAdapter}=hit); cache.binary='hit'; cache.frontend='skipped'; }
}
timingMs.cache=Math.round(performance.now()-started);
if (cache.binary==='hit') writeFileSync(join(out,'build.log'),'content-addressed executable cache hit\n');
else {
 cpSync(join(osg,'tools/gogen/go'),go,{recursive:true,filter:src=>!/(?:^|\/)zz_[^/]+\.go$/.test(src)});
 mkdirSync(cmd,{recursive:true});
 const front=cacheRoot ? readSnapshot(cacheRoot,frontKey,generated) : null;
 if (front) { ({refusals,regexAdapter}=front); cache.frontend='hit'; }
 else {
  // The pinned frontend retains module globals; process exit releases all of
  // them before compilation, even when an explicit GC cannot collect them.
  const frontend=spawnSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-osg-frontend.mjs'),input,out,osg,cacheRoot ?? ''],{env:process.env,stdio:'inherit'});
  if(frontend.status!==0)throw new Error(`OSG frontend exited ${frontend.status}: ${frontend.error?.message ?? frontend.signal ?? ''}`);
  const state=JSON.parse(readFileSync(join(out,'frontend.json'),'utf8'));
  ({refusals,regexAdapter}=state);
  frontendPeakRSSKiB=state.nodePeakRSSKiB;
  Object.assign(timingMs,state.timingMs);Object.assign(cache,state.cache);
  if(cacheRoot)writeSnapshot(cacheRoot,frontKey,generated,{refusals,regexAdapter});
 }
 // Recheck after frontend/emission: other jobs may have consumed the budget.
 await waitForResources();
 started=performance.now();
 // GNU time measures the heaviest build child, not just the small Go driver.
 const rssFile=join(out,'build-rss.txt');
 const timed=existsSync('/usr/bin/time');
 const build=spawnSync(timed?'/usr/bin/time':'go',timed?['-v','-o',rssFile,'go',...buildArgs,'-o',binary,'./cmd/abapitests']:[...buildArgs,'-o',binary,'./cmd/abapitests'],{cwd:go,env:buildEnv,encoding:'utf8',maxBuffer:32*1024*1024});
 timingMs.goBuild=Math.round(performance.now()-started);
 if(timed && existsSync(rssFile)){
  const match=readFileSync(rssFile,'utf8').match(/Maximum resident set size \(kbytes\):\s*(\d+)/);
  if(match)buildPeakRSSKiB=Number(match[1]);
 }
 writeFileSync(join(out,'build.log'),(build.stdout??'')+(build.stderr??''));
 if(build.status!==0){
  process.stderr.write(build.stderr??build.error?.message??'OSG build failed');
  writeFileSync(join(out,'osg-refusals.json'),JSON.stringify(refusals,null,2)+'\n');
  writeFileSync(join(out,'timing.json'),JSON.stringify({pin,timingMs,status:2,envelope,regexAdapter,mode,admission,cache,buildPeakRSSKiB,frontendPeakRSSKiB,nodePeakRSSKiB:Math.max(frontendPeakRSSKiB ?? 0,process.resourceUsage().maxRSS)},null,2)+'\n');
  process.exit(2);
 }
 if(cacheRoot)writeSnapshot(cacheRoot,binaryKey,{binary},{refusals,regexAdapter});
}
writeFileSync(join(out,'osg-refusals.json'),JSON.stringify(refusals,null,2)+'\n');
started=performance.now();
// Cache code only. Each restart replays registration in a fresh session, skips
// only known dumping initialization actions, and validates stable test indices.
let status=2;
try {
 if (process.env.ABAPITI_OSG_BUILD_ONLY === '1') {
  status=0;
 } else {
  const run=resumeRunner(binary,{cwd:out,args:[join(input,'names.json')],diagnostic:text=>process.stderr.write(text)});
  process.stdout.write(JSON.stringify(run.report)+'\n');
  status=run.status;
 }
} catch (error) { process.stderr.write(String(error)+'\n'); }
timingMs.run=Math.round(performance.now()-started);
writeFileSync(join(out,'timing.json'),JSON.stringify({pin,timingMs,status,envelope,regexAdapter,mode,admission,cache:{...cache,frontKey,binaryKey},buildPeakRSSKiB,frontendPeakRSSKiB,nodePeakRSSKiB:Math.max(frontendPeakRSSKiB ?? 0,process.resourceUsage().maxRSS)},null,2)+'\n');
process.exitCode=status;
