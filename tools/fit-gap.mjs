#!/usr/bin/env node
// Dynamic evidence, not a proof of reachability. Declaration identity matches registry-coverage.
import {readFileSync, readdirSync, statSync, mkdirSync, writeFileSync, rmSync, existsSync} from 'node:fs';
import {join, resolve, relative} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createRequire} from 'node:module';
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {buildUpstream, verifyUpstream, upstreamPin} from './statements-upstream.mjs';
import {declarationCandidates} from './registry-coverage-identity.mjs';
import {validateWorkload} from './registry-workload.mjs';

const self = fileURLToPath(import.meta.url);
const repo = resolve(fileURLToPath(new URL('..', import.meta.url)));
const scratch = '/tmp/claude-1000/-home-alice-dev-abapiti/3f37988e-1ca8-4480-a1b0-02daa31f268f/scratchpad';
const sha = data => createHash('sha256').update(data).digest('hex');
const compare = (a,b) => a < b ? -1 : a > b ? 1 : 0;
function files(root) {
 return readdirSync(root).sort().flatMap(n => {
  const p = join(root,n);
  return statSync(p).isDirectory() ? files(p) : [p];
 });
}

if (process.argv[2] === '--all-worker') {
 const [core,input,deps,config] = process.argv.slice(3);
 const a = createRequire(import.meta.url)(join(core,'index.js'));
 const six = new a.Config(readFileSync(config,'utf8'));
 const cfg = a.Config.getDefault(six.getSyntaxSetttings().version);
 const reg = new a.Registry(cfg);
 for (const p of files(input)) reg.addFile(new a.MemoryFile(relative(input,p),readFileSync(p,'utf8')));
 for (const p of files(deps)) reg.addDependency(new a.MemoryFile(relative(deps,p),readFileSync(p,'utf8')));
 reg.parse();
 const issues = reg.findIssues();
 for (const i of issues) [i.getKey(),i.getSeverity(),i.getFilename(),i.getStart().getRow(),i.getStart().getCol(),i.getEnd().getRow(),i.getEnd().getCol(),i.getMessage()].join('|');
 console.log(JSON.stringify({workload:'ALL',rules:cfg.getEnabledRules().length,issues:issues.length}));
} else {
 const argv = process.argv.slice(2);
 const testsOnly = argv.includes('--tests-only');
 const args = argv.filter(a => a !== '--tests-only');
 if (args.includes('--help') || args.length > 6) {
  console.log('usage: nice -n 10 node tools/fit-gap.mjs [out input deps config negatives upstream] [--tests-only]');
  process.exit(args.includes('--help') ? 0 : 1);
 }
 const [outArg=join(scratch,'gapdyn'),inputArg=join(scratch,'zabapgit/in'),depsArg=join(scratch,'zabapgit/deps/src'),configArg=join(scratch,'zabapgit/cfg/ci-abaplint.json'),negativeArg=join(scratch,'neg/negative.json'),upstreamArg='/home/alice/dev/abaplint'] = args;
 const [out,input,deps,config,negatives,upstream] = [outArg,inputArg,depsArg,configArg,negativeArg,upstreamArg].map(p=>resolve(p));
 process.env.TMPDIR ??= '/mnt/ram/abapiti/tmp';
 process.env.npm_config_cache = '/mnt/ssd-510/osg/abapiti/npm-cache';
 // Fail before building if the requested workloads are missing. Partial runs are explicit.
 if (!testsOnly) {
  for (const p of [input,deps,config,negatives]) if (!existsSync(p)) throw new Error('missing workload input: '+p+' (use --tests-only only for an explicitly partial report)');
  validateWorkload(input,deps,negatives);
 }
 const embeddedPath = join(repo,'tsfront/testdata/registrycorpus/reachability.json');
 const embeddedRaw = readFileSync(embeddedPath,'utf8');
 const embedded = JSON.parse(embeddedRaw);
 if (embedded.schema !== 2 || embedded.upstreamPin !== upstreamPin) throw new Error('embedded manifest schema/pin mismatch');
 const ts = createRequire(import.meta.url)(verifyUpstream(upstream));
 const require = createRequire(join(upstream,'packages/core/package.json'));
 const {SourceMapConsumer} = require('source-map');
 const embeddedKeys = new Set(embedded.spans.filter(s=>s.executed).map(s=>s.file+':'+s.start));
 const sources = new Map(), declarations = [];
 const srcRoot = join(upstream,'packages/core/src');
 for (const path of files(srcRoot).filter(p=>p.endsWith('.ts'))) {
  const rel = relative(srcRoot,path), source = readFileSync(path,'utf8');
  const sf = ts.createSourceFile(rel,source,ts.ScriptTarget.Latest,true);
  const candidates = [];
  function visit(n) {
   if ((ts.isMethodDeclaration(n)||ts.isFunctionDeclaration(n)||ts.isConstructorDeclaration(n)||ts.isGetAccessor(n)||ts.isSetAccessor(n)) && n.body) {
    candidates.push(n);
    const start = n.getStart(sf), parts = rel.split('/');
    const ctor = ts.isConstructorDeclaration(n), name = ctor ? 'constructor' : n.name?.getText(sf);
    const owner = n.parent.name?.getText(sf);
    const symbol = (ts.isMethodDeclaration(n)||ts.isGetAccessor(n)||ts.isSetAccessor(n)||ctor) ? `${owner ?? '<anonymous>'}.${name}` : name;
    const file = 'src/'+rel, byteStart = Buffer.byteLength(source.slice(0,start));
    declarations.push({key:rel+':'+start,file,start:byteStart,symbol,line:sf.getLineAndCharacterOfPosition(start).line+1,area:parts[0]==='abap' ? 'abap/'+parts[1] : parts.length===1 ? '(root)' : parts[0],inEmbedded:embeddedKeys.has(file+':'+byteStart),six:testsOnly ? null : false,all:testsOnly ? null : false,tests:false});
   }
   ts.forEachChild(n,visit);
  }
  visit(sf); sources.set(rel,{sf,candidates});
 }
 declarations.sort((a,b)=>compare(a.file,b.file)||a.start-b.start);
 const byKey = new Map(declarations.map(d=>[d.key,d]));
 mkdirSync(out,{recursive:true});
 const v8 = join(out,'v8');
 rmSync(v8,{recursive:true,force:true});
 for (const w of ['SIX','ALL','TESTS']) mkdirSync(join(v8,w),{recursive:true});
 const environment = w => ({...process.env,NODE_V8_COVERAGE:join(v8,w)});
 function run(w,args,allowFailures=false) {
  const result = spawnSync('nice',['-n','10','node',...args],{env:environment(w),encoding:'utf8',timeout:1800000,maxBuffer:64*1024*1024});
  const text = (result.stdout ?? '')+(result.stderr ?? '');
  writeFileSync(join(out,w+'.log'),text);
  process.stdout.write(text);
  if (result.error || result.signal || (result.status !== 0 && !allowFailures)) throw result.error ?? new Error(`${w} exited ${result.status}, signal ${result.signal}`);
  return {status:result.status,text};
 }
 const build = buildUpstream(upstream);
 try {
  const testRoot = resolve(build.core,'../test');
  const mocha = require.resolve('mocha/bin/mocha.js');
  if (!testsOnly) {
   // Exactly registry-coverage's four constituent workloads, unioned in SIX.
   const worker = join(repo,'tools/registry-coverage.mjs');
   for (const w of ['DEPLOYMENT','NEGATIVE','OBSERVATION']) {
    const r = spawnSync('nice',['-n','10','node',worker,'--worker',build.core,input,deps,config,negatives,w,join(out,'observations')],{env:environment('SIX'),encoding:'utf8',timeout:1800000,maxBuffer:64*1024*1024});
    writeFileSync(join(out,'SIX-'+w+'.log'),(r.stdout ?? '')+(r.stderr ?? ''));
    if (r.error || r.signal || r.status !== 0) throw r.error ?? new Error(`SIX ${w} failed: ${r.status}`);
   }
   const selected = ['check_syntax','unknown_types','implement_methods','superclass_final','parser_error','allowed_object_naming'].map(n=>join(testRoot,'rules',n+'.js'));
   selected.push(...files(join(testRoot,'abap/syntax')).filter(p=>p.endsWith('.js')));
   run('SIX',[mocha,'--timeout','1000000','--reporter','dot',...selected]);
   run('ALL',[self,'--all-worker',build.core,input,deps,config]);
  }
  const testFiles = files(testRoot).filter(p=>p.endsWith('.js'));
  const testResult = run('TESTS',[mocha,'--timeout','1000000','--reporter','dot',...testFiles],true);
  const stripANSI = testResult.text.replace(/\x1b\[[0-9;]*m/g,'');
  const failures = Number(stripANSI.match(/\b(\d+) failing\b/)?.[1] ?? 0);
  if (!/\b\d+ passing\b/.test(stripANSI) || (testResult.status !== 0 && failures === 0)) throw new Error('TESTS did not finish normally; see TESTS.log');
  const cache = new Map();
  const mappingStats = {};
  for (const [w,field] of [['SIX','six'],['ALL','all'],['TESTS','tests']]) {
   if (testsOnly && w !== 'TESTS') continue;
   const executed = new Set(), ambiguous = new Set();
   let unmappedExecuted = 0;
   for (const path of files(join(v8,w)).filter(p=>p.endsWith('.json'))) {
    for (const script of JSON.parse(readFileSync(path,'utf8')).result) {
     if (!script.url.startsWith('file:')) continue;
     const jsPath = fileURLToPath(script.url);
     if (!jsPath.startsWith(build.core+'/') || !jsPath.endsWith('.js')) continue;
     const rel = relative(build.core,jsPath).replace(/\.js$/,'.ts');
     if (!sources.has(rel)) continue;
     if (!cache.has(jsPath)) {
      const raw = readFileSync(jsPath,'utf8');
      const map = new SourceMapConsumer(JSON.parse(readFileSync(jsPath+'.map','utf8')));
      const generated = ts.createSourceFile(jsPath,raw,ts.ScriptTarget.Latest,true,ts.ScriptKind.JS), kinds = new Map();
      function visit(n) {
       if (ts.isClassDeclaration(n)||ts.isClassExpression(n)) kinds.set(n.getStart(generated),'Constructor');
       else if (ts.isMethodDeclaration(n)||ts.isFunctionDeclaration(n)||ts.isGetAccessor(n)||ts.isSetAccessor(n)) kinds.set(n.getStart(generated),ts.SyntaxKind[n.kind]);
       ts.forEachChild(n,visit);
      }
      visit(generated);
      const offsets = [0]; for (let i=0;i<raw.length;i++) if (raw[i]==='\n') offsets.push(i+1);
      const mappings = []; map.eachMapping(m=>{if(m.originalLine != null) mappings.push({...m,offset:offsets[m.generatedLine-1]+m.generatedColumn});});
      map.destroy?.();
      cache.set(jsPath,{kinds,mappings,matches:new Map()});
     }
     const cached = cache.get(jsPath), {sf,candidates} = sources.get(rel);
     for (const fn of script.functions.slice(1)) {
      const r = fn.ranges[0], signature = `${r.startOffset}:${r.endOffset}:${fn.functionName}`;
      if (!cached.matches.has(signature)) cached.matches.set(signature,declarationCandidates(ts,sf,candidates,cached.mappings,fn,cached.kinds.get(r.startOffset)).map(n=>rel+':'+n.getStart(sf)));
      const matches = cached.matches.get(signature);
      if (matches.length !== 1) {
       if (r.count) unmappedExecuted++;
       for (const key of matches) {ambiguous.add(key);executed.delete(key);}
      } else if (r.count && !ambiguous.has(matches[0])) executed.add(matches[0]);
     }
    }
   }
   for (const key of executed) byKey.get(key)[field] = true;
   mappingStats[w] = {executed:executed.size,ambiguous:ambiguous.size,unmappedExecuted};
  }
  const liveKeys = new Set(declarations.filter(d=>d.inEmbedded).map(d=>d.key));
  const sixKeys = new Set(declarations.filter(d=>d.six).map(d=>d.key));
  const diff = testsOnly ? null : {sixOnly:[...sixKeys].filter(k=>!liveKeys.has(k)).sort(),embeddedOnly:[...liveKeys].filter(k=>!sixKeys.has(k)).sort()};
  const needed = d => d.all || d.tests;
  const count = f => declarations.filter(f).length;
  const totals = {declarations:declarations.length,six:testsOnly ? null : count(d=>d.six),all:testsOnly ? null : count(d=>d.all),tests:count(d=>d.tests),needed:count(needed),live:liveKeys.size,gap:count(d=>needed(d)&&!d.inEmbedded)};
  function aggregate(field) {
   const groups = new Map();
   for (const d of declarations) {
    const key = d[field]; if (!groups.has(key)) groups.set(key,{name:key,total:0,live:0,needed:0,gap:0});
    const g = groups.get(key);g.total++;g.live+=Number(d.inEmbedded);g.needed+=Number(!!needed(d));g.gap+=Number(needed(d)&&!d.inEmbedded);
   }
   return [...groups.values()];
  }
  const areas = aggregate('area').sort((a,b)=>b.gap-a.gap||compare(a.name,b.name));
  const topFiles = aggregate('file').sort((a,b)=>b.gap-a.gap||compare(a.name,b.name)).slice(0,30);
  // One row per rule class, listing all locally defined run/runParsed/check entry methods.
  const ruleClasses = new Map();
  for (const [rel,{sf}] of sources) if (rel.startsWith('rules/')) {
   function visit(n) {
    if (ts.isClassDeclaration(n) && n.name && !rel.split('/').at(-1).startsWith('_') && !n.name.text.endsWith('Conf')) {
     const symbol = n.name.text;
     const methods = declarations.filter(d=>d.file==='src/'+rel && ['run','runParsed','check'].some(m=>d.symbol===symbol+'.'+m));
     ruleClasses.set(rel+':'+symbol,{file:'src/'+rel,symbol,methods:methods.map(d=>({symbol:d.symbol,inEmbedded:d.inEmbedded,all:d.all,tests:d.tests}))});
    }
    ts.forEachChild(n,visit);
   }
   visit(sf);
  }
  const rules = [...ruleClasses.values()].sort((a,b)=>compare(a.file,b.file)||compare(a.symbol,b.symbol));
  const report = {schema:1,upstreamPin,partial:testsOnly,embedded:{file:relative(repo,embeddedPath),sha256:sha(embeddedRaw),recorded:embedded.spans.length,executed:embeddedKeys.size,unmatchedExecuted:embeddedKeys.size-liveKeys.size},testFiles:testFiles.length,testFailures:failures,testExitCode:testResult.status,mappingStats,sixDiff:diff,totals,areas,topFiles,rules,declarations:declarations.map(({key,...d})=>d)};
  writeFileSync(join(out,'fit-gap.json'),JSON.stringify(report,null,2)+'\n');
  const yes = v => v === null ? 'not measured' : v ? 'yes' : 'no';
  const table = (headers,rows) => ['| '+headers.join(' | ')+' |','| '+headers.map(()=>'---').join(' | ')+' |',...rows.map(r=>'| '+r.join(' | ')+' |')].join('\n');
  const lines = ['# Dynamic fit-gap of translated abaplint','',`Upstream: \`${upstreamPin}\`. Denominator: every method, function, constructor and get/set accessor with a body in core src, including unloaded files. Arrow functions and function expressions are excluded, matching registry-coverage identities. Root V8 function counts only; ambiguous/unmapped executions do not imply coverage.`, '',testsOnly ? '**PARTIAL: SIX and ALL inputs unavailable/not run. Needed and gap below are TESTS-only lower bounds, NOT ALL ∪ TESTS.**' : 'SIX reproduces registry-coverage: clean deployment, negatives, oracle observations, six-rule tests and syntax tests. ALL uses Config.getDefault with the SIX syntax version; no negative variants.', '',`Embedded manifest: ${embedded.spans.length} recorded, ${embeddedKeys.size} executed, ${report.embedded.unmatchedExecuted} unmatched executed spans.`, `Tests: ${testFiles.length} files; ${failures} failures; exit ${testResult.status}.`, testsOnly ? 'SIX sanity diff: not measured.' : `SIX sanity symmetric difference: ${diff.sixOnly.length+diff.embeddedOnly.length} (${diff.sixOnly.length} SIX-only; ${diff.embeddedOnly.length} embedded-only).`, '', '## Totals','',table(['Metric','Declarations'],[['Declarations in src',totals.declarations],['SIX',totals.six ?? 'not measured'],['ALL',totals.all ?? 'not measured'],['TESTS',totals.tests],[testsOnly ? 'Needed (TESTS only)' : 'ALL ∪ TESTS',totals.needed],['Currently live (embedded)',totals.live],['Gap (needed minus live)',totals.gap]]),'','## By area','', 'Gap % = gap / needed (0 when needed is zero). Root-level src files are grouped as `(root)`.','',table(['Area','Total','Live','Needed','Gap','Gap %'],areas.map(g=>[g.name,g.total,g.live,g.needed,g.gap,(g.needed ? 100*g.gap/g.needed : 0).toFixed(1)+'%'])),'','## Top 30 files by gap','',table(['File','Total','Live','Needed','Gap'],topFiles.map(g=>[g.name,g.total,g.live,g.needed,g.gap])),'','## Rules','', 'Local entry methods are run, runParsed and check. Classes with no local entry show n/a (e.g. inherited implementations). Each cell lists the method-specific evidence.','',table(['File / class','Entry methods','Live','ALL','TESTS'],rules.map(r=>[r.file+' / '+r.symbol,r.methods.map(m=>m.symbol.split('.').at(-1)).join(', ')||'n/a',...['inEmbedded','all','tests'].map(k=>r.methods.map(m=>m.symbol.split('.').at(-1)+': '+yes(m[k])).join('; ')||'n/a')])),'','## Mapping diagnostics','', 'UnmappedExecuted counts V8 function records, not declarations (including unsupported callbacks); ambiguity is excluded, not assumed live.','',table(['Workload','Mapped executed','Ambiguous declarations','Unmapped executed records'],Object.entries(mappingStats).map(([w,s])=>[w,s.executed,s.ambiguous,s.unmappedExecuted])),''];
  writeFileSync(join(out,'fit-gap.md'),lines.join('\n'));
  console.log(`FIT-GAP${testsOnly ? ' PARTIAL' : ''}: total=${totals.declarations} live=${totals.live} SIX=${totals.six ?? 'unmeasured'} ALL=${totals.all ?? 'unmeasured'} TESTS=${totals.tests} needed=${totals.needed} gap=${totals.gap} testFailures=${failures} sixDiff=${diff ? diff.sixOnly.length+diff.embeddedOnly.length : 'unmeasured'}`);
 } finally { build.dispose(); }
}
