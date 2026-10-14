// M1 fresh-source RegistryRun oracle and canonical object/issue contract.
import {readFileSync,writeFileSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {createHash} from 'node:crypto';
import {createRequire} from 'node:module';
import {performance} from 'node:perf_hooks';
import {buildUpstream} from '../../tools/statements-upstream.mjs';
const upstream=resolve(process.argv[2] || '.tmp/selfhost-m2/abaplint');
const out=resolve('selfhost/m2'),kit=resolve('.tmp/selfhost-m2/zlnt-check-kit');
const manifest=JSON.parse(readFileSync(join(out,'workload.json')));
const files=manifest.files.filter(f=>f.checkable);
const config=JSON.parse(readFileSync(join(out,'abaplint.json')));
const hash=s=>createHash('sha256').update(s).digest('hex');
const json=(path,obj)=>writeFileSync(path,JSON.stringify(obj,null,2)+'\n');
const require=createRequire(import.meta.url);
const buildStart=performance.now(),built=buildUpstream(upstream);
const buildSeconds=(performance.now()-buildStart)/1000;
try{
 const ts=require(join(upstream,'node_modules/typescript'));
 const source=readFileSync('tsfront/testdata/registrycorpus/harness/registry_run.ts','utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText.replace(/require\("\.\.\/src\/([^" ]+)"\)/g,(_,p)=>`require(${JSON.stringify(join(built.core,p+'.js'))})`);
 const harness=resolve('.tmp/selfhost-m2/node-harness.cjs');writeFileSync(harness,js);
 const {RegistryRun}=require(harness),h=new RegistryRun();
 const addStart=performance.now();
 for(const file of files)h.addFile(file.path,readFileSync(join(kit,file.path),'utf8'));
 const addSeconds=(performance.now()-addStart)/1000;
 const start=performance.now(),reg=h.parse(JSON.stringify(config)),dump=h.report(reg),checkSeconds=(performance.now()-start)/1000;
 writeFileSync(join(kit,'expected-clean.txt'),dump+'\n');
 const groups=new Map(files.map(f=>[`${f.objectType} ${f.objectName}`,[]]));
 const byPath=new Map(files.map(f=>[f.path,f]));
 const rows=dump.split('\n'),total=Number(rows.shift()),rules=Object.fromEntries(Object.keys(config.rules).map(k=>[k,0]));
 writeFileSync(join(kit,'issues-sorted.txt'),rows.slice().sort().join('\n')+(rows.length?'\n':''));
 for(const row of rows.filter(Boolean)){
  const [rule,severity,path,start,end,...message]=row.split('|');
  const file=byPath.get(path);if(!file) throw new Error(`unmapped issue ${path}`);
  const issue={rule,severity,file:path,start:start.split(':').map(Number),end:end.split(':').map(Number),message:message.join('|')};
  groups.get(`${file.objectType} ${file.objectName}`).push(issue);rules[rule]=(rules[rule]||0)+1;
 }
 const objects=[...groups].sort(([a],[b])=>a<b?-1:a>b?1:0).map(([object,issues])=>({object,issues:issues.sort((a,b)=>JSON.stringify(a)<JSON.stringify(b)?-1:JSON.stringify(a)>JSON.stringify(b)?1:0)}));
 if(objects.reduce((n,o)=>n+o.issues.length,0)!==total)throw new Error('issue count mismatch');
 const canonical=JSON.stringify(objects)+'\n';
 const normalizedSha256=hash(canonical);
 const topObjects=objects.filter(o=>o.issues.length).map(o=>({object:o.object,issues:o.issues.length})).sort((a,b)=>b.issues-a.issues||a.object.localeCompare(b.object)).slice(0,20);
 const result={schema:'selfhost-node-reference/1',abaplintPin:manifest.abaplintPin,workloadSha256:hash(readFileSync(join(out,'workload.json'))),configSha256:hash(readFileSync(join(out,'abaplint.json'))),totalIssues:total,byRule:rules,byObject:objects.filter(o=>o.issues.length).map(o=>({object:o.object,issues:o.issues.length})),topObjects,normalizedSha256,normalization:'objects sorted by type/name; issues sorted by compact JSON UTF-16 lexical order; sha256(JSON.stringify(objects) + LF); positions are 1-based; paths relative to synthetic root',kitDumpSha256:hash(dump),expectedFile:{path:'.tmp/selfhost-m2/zlnt-check-kit/expected-clean.txt',sha256:hash(dump+'\n'),bytes:Buffer.byteLength(dump+'\n')},sortedIssues:{path:'.tmp/selfhost-m2/zlnt-check-kit/issues-sorted.txt',sha256:hash(readFileSync(join(kit,'issues-sorted.txt')))},timings:{buildSeconds,addSeconds,checkSeconds,stages:h.timings(),peakRSSKiB:process.resourceUsage().maxRSS,node:process.version},registryObjects:[...reg.getObjects()].length,objects};
 const fullJson=JSON.stringify(result,null,2)+'\n';
 if(Buffer.byteLength(fullJson)>4*1024*1024){writeFileSync('.tmp/selfhost-m2/node-reference-full.json',fullJson);delete result.objects;result.fullResult={path:'.tmp/selfhost-m2/node-reference-full.json',sha256:hash(fullJson),bytes:Buffer.byteLength(fullJson)};}
 json(join(out,'node-reference.json'),result);
 console.log(JSON.stringify({...result,objects:undefined},null,2));
}finally{built.dispose();}
