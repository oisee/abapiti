#!/usr/bin/env node
// Coverage is evidence for a workload, never a proof of general reachability.
import {readFileSync, readdirSync, statSync, mkdirSync, writeFileSync} from 'node:fs';
import {join, resolve, relative} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createRequire} from 'node:module';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {buildUpstream, verifyUpstream, upstreamPin} from './statements-upstream.mjs';
const hash = text => createHash('sha256').update(text).digest('hex');
const args = process.argv.slice(2);
if (args[0] === '--worker') {
 const [, core, input, deps, config, negatives] = args;
 const a = createRequire(import.meta.url)(join(core, 'index.js'));
 const cfg = readFileSync(config, 'utf8');
 const files = [];
 function walk(dir, prefix = '') { for (const n of readdirSync(dir).sort()) { const p = join(dir,n), name = prefix+n; if (statSync(p).isDirectory()) walk(p,name+'/'); else files.push([name,readFileSync(p,'utf8')]); } }
 walk(deps);
 const main = readdirSync(input).sort().map(n => [n,readFileSync(join(input,n),'utf8')]);
 const variants = JSON.parse(readFileSync(negatives,'utf8'));
 for (const variant of [null,...variants]) {
  const reg = new a.Registry(new a.Config(cfg));
  for (const [name,text] of main) reg.addFile(new a.MemoryFile(variant && name.endsWith('.abap') ? variant.filename : name,text+(variant && name.endsWith('.abap') ? variant.edit : '')));
  for (const [name,text] of files) reg.addDependency(new a.MemoryFile(name,text));
  reg.parse(); const issues = reg.findIssues();
  if (variant ? !issues.some(i=>i.getKey()===variant.target) : issues.length!==0) throw new Error('coverage workload failed: '+(variant?.target ?? 'north-star'));
  console.log(JSON.stringify({target:variant?.target ?? 'north-star', issues:issues.length}));
 }
} else {
 const [outArg,input,deps,config,negatives,upstreamArg='/home/alice/dev/abaplint'] = args;
 if (!negatives) throw new Error('usage: registry-coverage.mjs out-dir input-dir deps-src config negatives [upstream]');
 const out = resolve(outArg), upstream = resolve(upstreamArg);
 mkdirSync(out); mkdirSync(join(out,'v8'));
 const ts = createRequire(import.meta.url)(verifyUpstream(upstream));
 const require = createRequire(join(upstream,'packages/core/package.json'));
 const {SourceMapConsumer} = require('source-map');
 const build = buildUpstream(upstream);
 try {
  const env = {...process.env,NODE_V8_COVERAGE:join(out,'v8')};
  execFileSync(process.execPath,[fileURLToPath(import.meta.url),'--worker',build.core,resolve(input),resolve(deps),resolve(config),resolve(negatives)],{env,stdio:'inherit',timeout:1800000});
  const testRoot = join(build.core,'../test');
  const tests = ['check_syntax','unknown_types','implement_methods','superclass_final','parser_error','allowed_object_naming'].map(n=>join(testRoot,'rules',n+'.js'));
  function testFiles(dir) { return readdirSync(dir).sort().flatMap(n=>statSync(join(dir,n)).isDirectory()?testFiles(join(dir,n)):n.endsWith('.js')?[join(dir,n)]:[]); }
  tests.push(...testFiles(join(testRoot,'abap/syntax')));
  execFileSync(process.execPath,[require.resolve('mocha/bin/mocha.js'),'--timeout','1000000','--reporter','dot',...tests],{env,stdio:'inherit',timeout:1800000});
  // Union root function counts across every worker/test. Nested block counts do
  // not decide whether a function was called. Unmapped bodies remain live.
  const evidence = new Map();
  for (const file of readdirSync(join(out,'v8')).sort()) for (const script of JSON.parse(readFileSync(join(out,'v8',file),'utf8')).result) {
   if (!script.url.startsWith('file:')) continue;
   const path = fileURLToPath(script.url);
   if (!path.startsWith(build.core+'/') || !path.endsWith('.js')) continue;
   const raw = readFileSync(path,'utf8'), map = new SourceMapConsumer(JSON.parse(readFileSync(path+'.map','utf8')));
   const offsets = [0]; for (let i=0;i<raw.length;i++) if (raw[i]==='\n') offsets.push(i+1);
   const mappings = []; map.eachMapping(m=>{if (m.originalLine != null) mappings.push({...m,offset:offsets[m.generatedLine-1]+m.generatedColumn});});
   const rel = relative(build.core,path).replace(/\.js$/,'.ts');
   const source = readFileSync(join(upstream,'packages/core/src',rel),'utf8');
   const sf = ts.createSourceFile(rel,source,ts.ScriptTarget.Latest,true);
   const candidates = [];
   function visit(n) {
    if ((ts.isMethodDeclaration(n)||ts.isFunctionDeclaration(n)||ts.isConstructorDeclaration(n)||ts.isGetAccessor(n)||ts.isSetAccessor(n)) && n.body) candidates.push(n);
    ts.forEachChild(n,visit);
   } visit(sf);
   for (const fn of script.functions.slice(1)) {
    const r = fn.ranges[0];
    const inside = mappings.filter(m=>m.offset>=r.startOffset && m.offset<r.endOffset);
    if (!inside.length) continue;
    const m = inside[0], pos = sf.getPositionOfLineAndCharacter(m.originalLine-1,m.originalColumn);
    const n = candidates.find(n=>n.getStart(sf)<=pos && pos<n.body.getStart(sf));
    if (!n) continue;
    const name = ts.isConstructorDeclaration(n) ? n.parent.name?.text : n.name?.text;
    if (fn.functionName !== name && !(ts.isConstructorDeclaration(n) && fn.functionName==='constructor')) continue;
    const start = n.getStart(sf), end = n.end, key = rel+':'+start;
    const prev = evidence.get(key);
    const symbol = ts.isMethodDeclaration(n) || ts.isGetAccessor(n) || ts.isSetAccessor(n) ? n.parent.name?.text+'.'+name : name;
    evidence.set(key,{file:'src/'+rel,start:Buffer.byteLength(source.slice(0,start)),end:Buffer.byteLength(source.slice(0,end)),kind:ts.isConstructorDeclaration(n)?'Constructor':ts.SyntaxKind[n.kind],symbol,line:sf.getLineAndCharacterOfPosition(start).line+1,sha256:hash(source.slice(start,end)),executed:!!r.count || !!prev?.executed});
   }
  }
  const spans = [...evidence.values()].sort((a,b)=>(a.file<b.file?-1:a.file>b.file?1:0)||a.start-b.start);
  const inputs = [];
  function fingerprint(dir,prefix) { for(const n of readdirSync(dir).sort()) {const p=join(dir,n);if(statSync(p).isDirectory())fingerprint(p,prefix+n+'/');else inputs.push({file:prefix+n,sha256:hash(readFileSync(p))});} }
  fingerprint(resolve(input),'input/'); fingerprint(resolve(deps),'dependencies/');
  inputs.push({file:'config.json',sha256:hash(readFileSync(config))},{file:'negative-issues.json',sha256:hash(readFileSync(negatives))});
  const manifest={schema:1,upstreamPin,workloads:['north-star',...JSON.parse(readFileSync(negatives,'utf8')).map(v=>v.target),'six-rule-unit-tests','abap-syntax-unit-tests'],inputs,spans};
  writeFileSync(join(out,'reachability.json'),JSON.stringify(manifest,null,2)+'\n');
  console.log(JSON.stringify({mapped:spans.length,executed:spans.filter(s=>s.executed).length,traps:spans.filter(s=>!s.executed).length}));
 } finally { build.dispose(); }
}
