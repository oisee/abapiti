// M1 host-only multi-file adapter. The translated checker/runtime stay byte-for-byte.
import {readFileSync,writeFileSync,cpSync,mkdirSync,readdirSync} from 'node:fs';
import {resolve,join,relative} from 'node:path';
import {spawnSync,execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {performance} from 'node:perf_hooks';
import {compilerPin} from './prepare.mjs';
const work=resolve('.tmp/selfhost-m2'),kit=join(work,'zlnt-check-kit'),generated=join(work,'go-build/go');
const manifest=JSON.parse(readFileSync('selfhost/m2/workload.json'));
const hash=s=>createHash('sha256').update(s).digest('hex');
function cgroup(){try{return {memoryMaxBytes:Number(readFileSync('/sys/fs/cgroup/memory.max','utf8').trim()),workspacePeakBytes:Number(readFileSync('/sys/fs/cgroup/memory.peak','utf8').trim()),events:Object.fromEntries(readFileSync('/sys/fs/cgroup/memory.events','utf8').trim().split('\n').map(s=>{const [k,v]=s.split(' ');return [k,Number(v)];}))};}catch{return null;}}
function run(name,cmd,args,cwd){
 const before=cgroup();
 const start=performance.now();
 const p=spawnSync(cmd,args,{cwd,encoding:'utf8',maxBuffer:64*1024*1024,timeout:20*60*1000,detached:true});
 // A timed-out shell runner must not leave its checker child computing.
 if(p.error?.code==='ETIMEDOUT'&&p.pid>0){try{process.kill(-p.pid,'SIGKILL');}catch(e){if(e.code!=='ESRCH')throw e;}}
 writeFileSync(join(work,`${name}.txt`),p.stdout||'');writeFileSync(join(work,`${name}.err`),p.stderr||'');
 const after=cgroup();
 const resourceEvidence=before&&after?{memoryMaxBytes:after.memoryMaxBytes,workspacePeakBytes:after.workspacePeakBytes,memoryEventsBefore:before.events,memoryEventsAfter:after.events,oomKillsDuringRun:after.events.oom_kill-before.events.oom_kill}:null;
 return {resourceEvidence,exitCode:p.status,signal:p.signal,error:p.error?.code,timeoutSeconds:1200,wallSeconds:(performance.now()-start)/1000,stdoutSha256:hash(p.stdout||''),stderrSha256:hash(p.stderr||''),stderr:(p.stderr||'').replaceAll(work,'.tmp/selfhost-m2').trim(),stdout:p.stdout||''};
}
if(['--kit-runner','--kit-verify'].includes(process.argv[2])){
 const extracted=resolve(process.argv[3]||'.tmp/selfhost-m2/zip-check/zlnt-check-kit');
 for(const f of manifest.files)if(hash(readFileSync(join(extracted,f.path)))!==f.sha256)throw new Error(`extracted hash ${f.path}`);
 if(process.argv[2]==='--kit-verify'){
  const reference=JSON.parse(readFileSync('selfhost/m2/node-reference.json'));
  for(const [name,wanted] of [['workload.json',reference.workloadSha256],['abaplint.json',reference.configSha256],['expected-clean.txt',reference.expectedFile.sha256],['issues-sorted.txt',reference.sortedIssues.sha256]])if(hash(readFileSync(join(extracted,name)))!==wanted)throw new Error(`extracted contract hash ${name}`);
  if(readFileSync(join(extracted,'files.txt'),'utf8')!==manifest.files.filter(f=>f.checkable).map(f=>f.path).join('\n')+'\n'||readFileSync(join(extracted,'deps.txt'),'utf8')!=='')throw new Error('extracted file-list contract');
  execFileSync('sh',['-n',join(extracted,'check.sh')]);
  const previous=JSON.parse(readFileSync('selfhost/m2/go-probe.json'));
  previous.kitExtraction={workloadFilesVerified:manifest.files.length,contractHashesVerified:true,runnerShellSyntaxVerified:true,nativeRunRepeated:false,note:'Archive extraction and runner syntax verified; native full-workload outcome is recorded separately.'};
  writeFileSync('selfhost/m2/go-probe.json',JSON.stringify(previous,null,2)+'\n');console.log(JSON.stringify(previous.kitExtraction,null,2));process.exit(0);
 }
 const record=run('kit-runner','sh',[join(extracted,'check.sh'),join(work,'go-multifile/zabaplint-go')],extracted);
 const previous=JSON.parse(readFileSync('selfhost/m2/go-probe.json'));
 previous.kitRunner={...record,stdout:undefined,extractedWorkloadHashesVerified:true};
 writeFileSync('selfhost/m2/go-probe.json',JSON.stringify(previous,null,2)+'\n');
 console.log(JSON.stringify(previous.kitRunner,null,2));
 process.exit(0);
}
const result={schema:'selfhost-go-probe/1',environment:{GOMAXPROCS:process.env.GOMAXPROCS??null,GOMEMLIMIT:process.env.GOMEMLIMIT??null},compilerCommit:compilerPin,scope:'core',go:execFileSync('go',['version'],{encoding:'utf8'}).trim(),multiFileSupport:'#167 is absent from frozen HEAD and origin/selfhost; M1 go-multifile host adapter',workloadSha256:hash(readFileSync('selfhost/m2/workload.json')),configSha256:hash(readFileSync('selfhost/m2/abaplint.json'))};
const log=readFileSync(join(work,'go-build.log'),'utf8');
result.blockingDiagnostics=Number(/(\d+) blocking diagnostics/.exec(log)?.[1]);
result.hirVerificationErrors=Number(/HIR verified: (\d+) errors/.exec(log)?.[1]);
result.generationSeconds=Number(/TS-HG@Go generation: ([\d.]+)s/.exec(log)?.[1]);
result.originalBuildSeconds=Number(/TS-HG@Go built: .*?([\d.]+)s/.exec(log)?.[1]);
const adapter=join(work,'go-multifile');mkdirSync(adapter,{recursive:true});
cpSync(generated,adapter,{recursive:true});
let main=readFileSync(join(adapter,'main.go'),'utf8');
const oldName=String.raw`name:=filepath.Base(strings.ReplaceAll(path,"\\","/"))`;
const oldLoad='cfg:=read(*config);add(*file,false)';
if(!main.includes(oldName)||!main.includes(oldLoad))throw new Error('generated loader shape changed');
result.originalHostSha256=hash(main);
main=main.replace(oldName,'name:=filepath.ToSlash(path)').replace(oldLoad,`cfg:=read(*config); scanFiles:=bufio.NewScanner(strings.NewReader(read(*file))); for scanFiles.Scan(){p:=strings.TrimSuffix(scanFiles.Text(),"\\r");if p!=""{add(p,false)}};if err:=scanFiles.Err();err!=nil{panic(err)}`);
writeFileSync(join(adapter,'main.go'),main);
result.adapterHostSha256=hash(main);
// Prove this is an I/O-only adapter: every other generated file stays unchanged.
let unchanged=0;
function verify(dir){for(const entry of readdirSync(dir,{withFileTypes:true})){
 const path=join(dir,entry.name),rel=relative(generated,path);
 if(entry.isDirectory())verify(path);
 else if(rel!=='main.go'&&rel!=='zabaplint-go'){if(hash(readFileSync(path))!==hash(readFileSync(join(adapter,rel))))throw new Error(`adapter changed ${rel}`);unchanged++;}
}}
verify(generated);result.unchangedGeneratedSourceFiles=unchanged;
const built=run('go-multifile-build','go',['build','-p','2','-o','zabaplint-go','.'],adapter);
result.adapterBuild={...built,stdout:undefined};
verify(generated);
if(built.exitCode===0){
 const full=run('go-full',join(adapter,'zabaplint-go'),['--file','files.txt','--config','abaplint.json','--deps','deps.txt','--metrics','-allow-read','.'],kit);
 const expected=readFileSync(join(kit,'expected-clean.txt'),'utf8');
 const lines=s=>s.trimEnd().split('\n').slice(1).sort();
 const difference=(a,b)=>{const counts=new Map();for(const x of b)counts.set(x,(counts.get(x)||0)+1);return a.filter(x=>{const n=counts.get(x)||0;if(n){counts.set(x,n-1);return false;}return true;});};
 const completed=full.exitCode===0 && /^\d+\n/.test(full.stdout);
 result.fullWorkload={...full,stdout:undefined,loader:'M1 generated main.go I/O adaptation; every files.txt member is addFile with relative path; no dependencies',files:manifest.counts.registryFiles,totalIssues:completed?Number(full.stdout.split('\n')[0]):null,byteEqualToNode:completed?full.stdout===expected:null,issueMultisetEqualToNode:completed?JSON.stringify(lines(full.stdout))===JSON.stringify(lines(expected)):null,nodeOnly:completed?difference(lines(expected),lines(full.stdout)):null,goOnly:completed?difference(lines(full.stdout),lines(expected)):null,status:completed?(full.stdout===expected?'equal':'different'):(full.stderr.includes('refused:')?'refused':'incomplete'),incompleteReason:!completed&&full.resourceEvidence?.oomKillsDuringRun>0?'cgroup_oom':(!completed&&full.error==='ETIMEDOUT'?'timeout':null)};
}
const fullJSON=JSON.stringify(result,null,2)+'\n';
if(Buffer.byteLength(fullJSON)>4*1024*1024){
 const path='.tmp/selfhost-m2/go-probe-full.json';writeFileSync(path,fullJSON);
 result.fullResult={path,sha256:hash(fullJSON),bytes:Buffer.byteLength(fullJSON)};
 for(const key of ['nodeOnly','goOnly']){result.fullWorkload[key+'Count']=result.fullWorkload[key]?.length??null;delete result.fullWorkload[key];}
}
writeFileSync('selfhost/m2/go-probe.json',JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result,null,2));
