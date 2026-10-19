// Inventory only: adapt the generated host loader, never the translated checker.
import {readFileSync,writeFileSync,cpSync,mkdirSync} from 'node:fs';
import {resolve,join,relative} from 'node:path';
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {performance} from 'node:perf_hooks';
const kit=resolve('.tmp/selfhost/osg-check-kit'), generated=resolve('.tmp/selfhost/go-build/go');
const manifest=JSON.parse(readFileSync('selfhost/m1/workload.json'));
const binary=join(generated,'zabaplint-go');
const files=manifest.files.map(f=>f.path), first=files.find(f=>f.endsWith('.clas.abap'));
const sha=s=>createHash('sha256').update(s).digest('hex');
if(process.argv[2]==='--kit-runner'){
 const extracted=resolve(process.argv[3]||'.tmp/selfhost/zip-check/osg-check-kit');
 for(const f of manifest.files)if(sha(readFileSync(join(extracted,f.path)))!==f.sha256)throw new Error(`extracted file hash ${f.path}`);
 const start=performance.now();
 const p=spawnSync('sh',[join(extracted,'check.sh'),resolve('.tmp/selfhost/go-multifile/zabaplint-go')],{encoding:'utf8',timeout:600000});
 const record={exitCode:p.status,signal:p.signal,wallSeconds:(performance.now()-start)/1000,stderr:p.stderr.trim(),stdout:p.stdout.trim(),extractedWorkloadHashesVerified:true};
 const previous=JSON.parse(readFileSync('selfhost/m1/go-probe.json'));
 previous.kitRunner=record;writeFileSync('selfhost/m1/go-probe.json',JSON.stringify(previous,null,2)+'\n');
 console.log(JSON.stringify(record,null,2));process.exit(0);
}
const result={schema:'selfhost-go-probe/1',compilerCommit:spawnSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).stdout.trim(),scope:'core',go:spawnSync('go',['version'],{encoding:'utf8'}).stdout.trim()};
const buildLog=readFileSync('.tmp/selfhost/go-build.log','utf8');
result.generationSeconds=Number(/TS-HG@Go generation: ([\d.]+)s/.exec(buildLog)?.[1]);
result.originalBuildSeconds=Number(/TS-HG@Go built: .*?([\d.]+)s/.exec(buildLog)?.[1]);
function run(name,cmd,args,cwd){
 const began=performance.now();
 const r=spawnSync(cmd,args,{cwd,encoding:'utf8',maxBuffer:32*1024*1024,timeout:10*60*1000});
 writeFileSync(`.tmp/selfhost/${name}.txt`,r.stdout||'');writeFileSync(`.tmp/selfhost/${name}.err`,r.stderr||'');
 return {exitCode:r.status,signal:r.signal,error:r.error?.code,wallSeconds:(performance.now()-began)/1000,stdoutSha256:sha(r.stdout||''),stderr:(r.stderr||'').replaceAll(resolve('.tmp'),'.tmp').trim(),stdout:r.stdout||''};
}
// Test the shipped host contract: one checked file, remaining files dependencies.
writeFileSync(join(kit,'single-deps.txt'),files.filter(f=>f!==first).join('\n')+'\n');
const single=run('go-single',binary,['--file',first,'--config','abaplint.json','--deps','single-deps.txt','--metrics'],kit);
result.existingCLI={...single,stdout:undefined,checkedFile:first,totalIssues:single.exitCode===0?Number(single.stdout.split('\n')[0]):null,limitation:'--file accepts one main file; --deps registers other objects as dependencies, so this is not a full-workload parity test'};
// Minimal generated host change: --file FILES_LIST loads every listed path
// via addFile and keeps synthetic relative paths. All HIR/runtime files copied.
const adapter=resolve('.tmp/selfhost/go-multifile');mkdirSync(adapter,{recursive:true});
cpSync(generated,adapter,{recursive:true});
let main=readFileSync(join(adapter,'main.go'),'utf8');
const oldName=String.raw`name:=filepath.Base(strings.ReplaceAll(path,"\\","/"))`;
const oldLoad='cfg:=read(*config);add(*file,false)';
if(!main.includes(oldName)||!main.includes(oldLoad))throw new Error('generated loader shape changed');
main=main.replace(oldName,'name:=filepath.ToSlash(path)').replace(oldLoad,`cfg:=read(*config); scanFiles:=bufio.NewScanner(strings.NewReader(read(*file))); for scanFiles.Scan(){p:=strings.TrimSuffix(scanFiles.Text(),"\\r");if p!=""{add(p,false)}};if err:=scanFiles.Err();err!=nil{panic(err)}`);
writeFileSync(join(adapter,'main.go'),main);
const built=run('go-multifile-build','go',['build','-o','zabaplint-go','.'],adapter);
result.adapterBuild={...built,stdout:undefined};
if(built.exitCode===0){
 const full=run('go-full',join(adapter,'zabaplint-go'),['--file','files.txt','--config','abaplint.json','--metrics'],kit);
 const expected=readFileSync(join(kit,'expected-clean.txt'),'utf8');
 const lines=s=>s.trimEnd().split('\n').slice(1).sort();
 const actualLines=lines(full.stdout),nodeLines=lines(expected);
 const multisetDifference=(a,b)=>{const counts=new Map();for(const x of b)counts.set(x,(counts.get(x)||0)+1);return a.filter(x=>{const n=counts.get(x)||0;if(n){counts.set(x,n-1);return false;}return true;});};
 result.fullWorkload={...full,stdout:undefined,loader:'generated main.go host-only adaptation: all files addFile; preserve synthetic paths; no dependencies',totalIssues:full.exitCode===0?Number(full.stdout.split('\n')[0]):null,byteEqualToNode:full.exitCode===0&&full.stdout===expected,issueMultisetEqualToNode:full.exitCode===0&&JSON.stringify(actualLines)===JSON.stringify(nodeLines),nodeOnly:full.exitCode===0?multisetDifference(nodeLines,actualLines):null,goOnly:full.exitCode===0?multisetDifference(actualLines,nodeLines):null};
}
writeFileSync('selfhost/m1/go-probe.json',JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result,null,2));
