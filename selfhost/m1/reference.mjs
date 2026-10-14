// Build a deterministic workload and the same fresh-source Node oracle as the kit.
import {readFileSync,writeFileSync,mkdirSync,existsSync} from 'node:fs';
import {resolve,join,relative,dirname} from 'node:path';
import {pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';
import {createRequire} from 'node:module';
import {execFileSync} from 'node:child_process';
import {performance} from 'node:perf_hooks';
import {buildUpstream,upstreamPin} from '../../tools/statements-upstream.mjs';
const root=resolve(process.argv[2] || '.tmp/selfhost/osg');
const upstream=resolve(process.argv[3] || '.tmp/abaplint');
const out=resolve('selfhost/m1'), kit=resolve('.tmp/selfhost/osg-check-kit');
const sha='3ee404ca1260464221f5eba7d86a06a38eabc6c4';
const hash=s=>createHash('sha256').update(s).digest('hex');
const normal=s=>s.replace(/^\uFEFF/,'').replace(/\r\n?/g,'\n');
const json=(file,value)=>writeFileSync(file,JSON.stringify(value,null,2)+'\n');
const {layers,filesIn,libraryFiles,objectOf}=await import(pathToFileURL(join(root,'tools/osd-inputs.mjs')));
const {packsOf}=await import(pathToFileURL(join(root,'tools/osd-packs.mjs')));
const {cockpitAppsOf}=await import(pathToFileURL(join(root,'tools/osd-cockpit-apps.mjs')));
const {unfetched}=await import(pathToFileURL(join(root,'tools/osd-fetch.mjs')));
const env={OSD_TMP:'off'}; // default packs, no workstation layers
if(unfetched(root,env).length) throw new Error('default packs are unfetched');
if(!existsSync(join(root,'gen'))) throw new Error('build generators have not run');
const cfg=JSON.parse(readFileSync(join(root,'abap_transpile.json')));
const frozen=JSON.parse(readFileSync('.osg/libs.lock.json'));
const kitConfig=JSON.parse(readFileSync('.tmp/kit/zabapgit-check-kit/abaplint.json'));
// JSONC syntax settings are read with the pinned oracle's locked json5 parser.
const require=createRequire(import.meta.url);
const json5=require(join(upstream,'packages/core/node_modules/json5'));
const syntax=json5.parse(readFileSync(join(root,'abaplint.jsonc'),'utf8')).syntax;
const config={global:{...kitConfig.global,files:'/**/*.*',errorOnDuplicateFilenames:true},dependencies:[],syntax,rules:kitConfig.rules};
json(join(out,'abaplint.json'),config);
// OSGo leaves test/ out before resolving owners; use its folder order.
const full=layers(root,cfg,env);
const stack=layers(root,{...cfg,input_folder:full.folders.filter(f=>f!=='test')},env);
if(stack.duplicates.length) throw new Error(JSON.stringify(stack.duplicates));
const apps=cockpitAppsOf(root,stack.folders).map(a=>a.folder+'/');
const hidden=new Set(stack.hidden), omitted=[];
const sources=[];
const identify=name=>{
 const obj=objectOf(name);if(!obj) return;
 const [type,...parts]=obj.split(' ');
 if(!/^[A-Z0-9]{4}$/.test(type))return;
 return {type,name:parts.join(' ').replaceAll('#','/')};
};
function add(source,path,layer,library=false){
 const id=identify(path.split('/').pop());
 if(!id || !/\.(abap|xml)$/i.test(path) || /\.devc\.xml$/i.test(path)) return;
 if(path.endsWith('.clas.testclasses.abap') || /\.ddls\.xml$/i.test(path) || (!library && apps.some(a=>path.startsWith(a)))) {
  omitted.push({path,reason:'OSGo frontend excludes testclasses, DDLS metadata and cockpit web files'});return;
 }
 sources.push({source,path,layer,objectType:id.type,objectName:id.name});
}
for(const folder of stack.folders){
 for(const f of filesIn(root,folder)){
  if(hidden.has(f.file))continue;
  add(join(root,f.file),f.file,folder);
 }
}
const mainNames=new Set(sources.map(f=>f.path.split('/').pop()));
for(const lib of libraryFiles(root,cfg)){
 const folder=lib.folder.split('/').pop();
 for(const file of lib.files){
  const path=`libs/${folder}/${relative(lib.dir,file)}`;
  if(mainNames.has(file.split('/').pop())){omitted.push({path,reason:'OSGo input filename overrides library'});continue;}
  add(file,path,`lib:${folder}`,true);
 }
}
sources.sort((a,b)=>a.path<b.path?-1:a.path>b.path?1:0);
const filenames=new Set();
for(const file of sources){
 const basename=file.path.split('/').pop();
 if(filenames.has(basename))throw new Error(`unresolved duplicate filename ${basename}`);
 filenames.add(basename);
 const raw=normal(readFileSync(file.source,'utf8'));file.sha256=hash(raw);
 mkdirSync(dirname(join(kit,file.path)),{recursive:true});writeFileSync(join(kit,file.path),raw);
}
const files=sources.map(({source,...f})=>f);
const count=key=>Object.fromEntries([...new Set(files.map(f=>f[key]))].sort().map(k=>[k,{files:files.filter(f=>f[key]===k).length,objects:new Set(files.filter(f=>f[key]===k).map(f=>`${f.objectType} ${f.objectName}`)).size}]));
const packs=packsOf(root,env).map(p=>({name:p.name,order:p.order,folders:p.abap.map(f=>relative(root,f)),sources:p.sources.map(s=>({folder:s.folder,repo:s.repo,ref:s.ref,path:s.path,exclude:s.exclude}))}));
const manifest={schema:'selfhost-workload/1',osgSha:sha,libs:frozen.libraries,abaplintPin:{sha:upstreamPin,package:'@abaplint/core',version:'2.120.56'},normalization:'UTF-8; strip one leading BOM; CRLF and CR to LF; preserve trailing newline',selection:{folders:stack.folders,defaultPacks:packs,excludedFolder:'test',librarySelection:'libraryFiles(abap_transpile.json); input filename overrides library; no library testclasses',generated:true,overridden:stack.overridden,omitted},counts:{files:files.length,objects:new Set(files.map(f=>`${f.objectType} ${f.objectName}`)).size,byObjectType:count('objectType'),byLayer:count('layer')},files};
json(join(out,'workload.json'),manifest);
console.log(JSON.stringify(manifest.counts,null,2));
writeFileSync(join(kit,'files.txt'),files.map(f=>f.path).join('\n')+'\n');
writeFileSync(join(kit,'deps.txt'),'');
writeFileSync(join(kit,'abaplint.json'),readFileSync(join(out,'abaplint.json')));
const buildStart=performance.now(),built=buildUpstream(upstream);
const buildSeconds=(performance.now()-buildStart)/1000;
try{
 const ts=require(join(upstream,'node_modules/typescript'));
 const source=readFileSync('tsfront/testdata/registrycorpus/harness/registry_run.ts','utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText.replace(/require\("\.\.\/src\/([^" ]+)"\)/g,(_,p)=>`require(${JSON.stringify(join(built.core,p+'.js'))})`);
 const harness=resolve('.tmp/selfhost/node-harness.cjs');writeFileSync(harness,js);
 const {RegistryRun}=require(harness),h=new RegistryRun();
 const addStart=performance.now();
 for(const file of files)h.addFile(file.path,readFileSync(join(kit,file.path),'utf8'));
 const addSeconds=(performance.now()-addStart)/1000;
 const start=performance.now(),reg=h.parse(JSON.stringify(config)),dump=h.report(reg),checkSeconds=(performance.now()-start)/1000;
 writeFileSync(join(kit,'expected-clean.txt'),dump+'\n');
 const groups=new Map(files.map(f=>[`${f.objectType} ${f.objectName}`,[]]));
 const byPath=new Map(files.map(f=>[f.path,f]));
 const rows=dump.split('\n'),total=Number(rows.shift()),rules={};
 for(const row of rows){
  const [rule,severity,path,start,end,...message]=row.split('|');
  const file=byPath.get(path);if(!file) throw new Error(`unmapped issue ${path}`);
  const issue={rule,severity,file:path,start:start.split(':').map(Number),end:end.split(':').map(Number),message:message.join('|')};
  groups.get(`${file.objectType} ${file.objectName}`).push(issue);rules[rule]=(rules[rule]||0)+1;
 }
 const objects=[...groups].sort(([a],[b])=>a<b?-1:a>b?1:0).map(([object,issues])=>({object,issues:issues.sort((a,b)=>JSON.stringify(a)<JSON.stringify(b)?-1:JSON.stringify(a)>JSON.stringify(b)?1:0)}));
 const canonical=JSON.stringify(objects)+'\n';
 const normalizedSha256=hash(canonical);
 const topObjects=objects.filter(o=>o.issues.length).map(o=>({object:o.object,issues:o.issues.length})).sort((a,b)=>b.issues-a.issues||a.object.localeCompare(b.object)).slice(0,20);
 const result={schema:'selfhost-node-reference/1',abaplintPin:manifest.abaplintPin,workloadSha256:hash(readFileSync(join(out,'workload.json'))),configSha256:hash(readFileSync(join(out,'abaplint.json'))),totalIssues:total,byRule:rules,topObjects,normalizedSha256,normalization:'objects sorted by type/name; issues sorted by compact JSON UTF-16 lexical order; sha256(JSON.stringify(objects) + LF); positions are 1-based; paths relative to synthetic root',kitDumpSha256:hash(dump),timings:{buildSeconds,addSeconds,checkSeconds,stages:h.timings(),peakRSSKiB:process.resourceUsage().maxRSS,node:process.version},registryObjects:[...reg.getObjects()].length,objects};
 const fullJson=JSON.stringify(result,null,2)+'\n';
 if(Buffer.byteLength(fullJson)>5*1024*1024){writeFileSync('.tmp/selfhost/node-reference-full.json',fullJson);delete result.objects;result.fullResult={path:'.tmp/selfhost/node-reference-full.json',sha256:hash(fullJson),bytes:Buffer.byteLength(fullJson)};}
 json(join(out,'node-reference.json'),result);
 console.log(JSON.stringify({...result,objects:undefined},null,2));
}finally{built.dispose();}
