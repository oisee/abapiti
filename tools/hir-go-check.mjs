#!/usr/bin/env node
// One machine, one runner: byte comparisons and full-check stage/resource timings.
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {resolve,join,basename} from 'node:path';
import {createRequire} from 'node:module';
import {spawnSync} from 'node:child_process';
import {PerformanceObserver,performance} from 'node:perf_hooks';
import {buildUpstream} from './statements-upstream.mjs';
const require=createRequire(import.meta.url);
if(process.argv[2]==='--node-worker'){
 const [harness,kit,variant]=process.argv.slice(3);
 const {RegistryRun}=require(harness);const h=new RegistryRun();
 const input=variant==='clean'?'zabapgit_standalone.prog.abap':'seeded/zabapgit_standalone.prog.abap';
 h.addFile(basename(input),readFileSync(join(kit,input),'utf8'));
 for(const p of readFileSync(join(kit,'deps.txt'),'utf8').split(/\r?\n/).filter(Boolean))h.addDependency(basename(p.replaceAll('\\','/')),readFileSync(resolve(kit,p),'utf8'));
 const cfg=readFileSync(join(kit,'abaplint.json'),'utf8');let gcMs=0;
 const observer=new PerformanceObserver(items=>{for(const e of items.getEntries())gcMs+=e.duration});observer.observe({entryTypes:['gc']});
 const start=performance.now();const dump=h.run(cfg);const elapsed=performance.now()-start;
 process.stdout.write(dump+'\n');
 await new Promise(r=>setTimeout(r,100));
 for(const e of observer.takeRecords())gcMs+=e.duration;
 console.error(JSON.stringify({Seconds:elapsed/1000,GCWallFraction:gcMs/elapsed,Stages:h.timings(),PeakRSSKiB:process.resourceUsage().maxRSS}));observer.disconnect();
}else{
 const [kitArg,goArg,outArg,releaseArg,upstreamArg]=process.argv.slice(2);
 if(!outArg)throw new Error('usage: hir-go-check.mjs KIT GO_BINARY OUTPUT_DIR [RELEASE_BINARY] [UPSTREAM]');
 const kit=resolve(kitArg),go=resolve(goArg),out=resolve(outArg);mkdirSync(out,{recursive:true});
 const upstream=resolve(upstreamArg||'/home/alice/dev/abaplint');const built=buildUpstream(upstream);
 try{
  const ts=require(join(upstream,'node_modules/typescript'));
  const source=readFileSync(new URL('../tsfront/testdata/registrycorpus/harness/registry_run.ts',import.meta.url),'utf8');
  const harness=join(out,'node-harness.cjs');
  const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText.replace(/require\("\.\.\/src\/([^" ]+)"\)/g,(_,p)=>`require(${JSON.stringify(join(built.core,p+'.js'))})`);
  writeFileSync(harness,js);
  const rows=[];
  function run(host,variant,command,args,extraEnv={}){
   console.error(`${host} ${variant}: running`);
   const prefix=join(out,`${host}-${variant}`),time=prefix+'.time';
   const result=spawnSync('/usr/bin/time',['-f','%e %U %S %M','-o',time,command,...args],{cwd:kit,env:{...process.env,...extraEnv},encoding:'utf8',maxBuffer:64*1024*1024});
   writeFileSync(prefix+'.txt',result.stdout||'');writeFileSync(prefix+'.err',result.stderr||'');
   if(result.error||result.status!==0)throw new Error(`${host} ${variant} refused (${result.status}): ${(result.stderr||'').slice(0,4000)}`);
   const expected=readFileSync(join(kit,`expected-${variant}.txt`),'utf8');
   const actual=result.stdout.replace(/^ms: .*\n?/mg,'');
   if(actual!==expected)throw new Error(`${host} ${variant} differs byte-for-byte from kit Node output`);
   const [wall,user,sys,rss]=readFileSync(time,'utf8').trim().split(' ').map(Number);
   let metric={};for(const line of result.stderr.split('\n')){if(line.startsWith('{')){try{metric=JSON.parse(line)}catch{}}}
   const stageText=metric.Stages||/^ms: (.*)$/m.exec(result.stdout)?.[1]||'';
   const stages=Object.fromEntries([...stageText.matchAll(/([a-z_]+)=(\d+)/g)].map(m=>[m[1],Number(m[2])/1000]));
   const rules=['unknown_types','allowed_object_naming','check_syntax','implement_methods','superclass_final','parser_error'].reduce((a,k)=>a+(stages[k]||0),0);
   let releaseGCCPUSeconds=0;
   if(host==='Go-ABAP')for(const m of result.stderr.matchAll(/([\d.]+)\+([\d.]+)\/([\d.]+)\/([\d.]+)\+([\d.]+) ms cpu/g))releaseGCCPUSeconds+=m.slice(1).map(Number).reduce((a,v)=>a+v,0)/1000;
   const row={releaseGCCPUFraction:host==='Go-ABAP'?releaseGCCPUSeconds/(user+sys):undefined,host,variant,issues:Number(actual.split('\n')[0]),checkSeconds:metric.Seconds||wall,wallSeconds:wall,cpuSeconds:user+sys,peakRSSMiB:rss/1024,goGCCPUFraction:metric.GCCPUFraction,goGCProcessCPUFraction:metric.GCCPUSeconds===undefined?undefined:metric.GCCPUSeconds/(user+sys),nodeGCWallFraction:metric.GCWallFraction,...stages,rules};rows.push(row);console.error(JSON.stringify(row));
  }
  for(const variant of ['clean','seeded']){
   const input=variant==='clean'?'zabapgit_standalone.prog.abap':'seeded/zabapgit_standalone.prog.abap';
   run('Node',variant,process.execPath,[resolve(import.meta.filename),'--node-worker',harness,kit,variant]);
   run('Go-HIR',variant,go,['--file',input,'--config','abaplint.json','--deps','deps.txt','--metrics','--cpu-profile',join(out,`${variant}.cpu`),'--mem-profile',join(out,`${variant}.alloc`)]);
  }
  if(releaseArg)run('Go-ABAP','clean',resolve(releaseArg),['--file','zabapgit_standalone.prog.abap','--config','abaplint.json','--deps','deps.txt','--times','-allow-read','.'],{GODEBUG:'gctrace=1'});
  writeFileSync(join(out,'results.json'),JSON.stringify({GOGC:process.env.GOGC||'default',rows},null,2)+'\n');
  console.log('| Host | Input | Check s | Lexer | Statements | Structures | Syntax | Rules | Peak MiB | GC share |');
  console.log('|---|---|---:|---:|---:|---:|---:|---:|---:|---:|');
  for(const r of rows)console.log(`| ${r.host} | ${r.variant} | ${r.checkSeconds.toFixed(3)} | ${r.lexer?.toFixed(3)||'—'} | ${r.statements?.toFixed(3)||'—'} | ${r.structures?.toFixed(3)||'—'} | ${r.syntax?.toFixed(3)||'—'} | ${r.rules.toFixed(3)} | ${r.peakRSSMiB.toFixed(1)} | ${((r.goGCProcessCPUFraction??r.goGCCPUFraction??r.nodeGCWallFraction??r.releaseGCCPUFraction??NaN)*100).toFixed(2)}% |`);
  const profile=spawnSync('go',['tool','pprof','-top','-nodecount=10',go,join(out,'clean.cpu')],{encoding:'utf8',env:process.env});
  if(profile.status!==0)throw new Error(profile.stderr);writeFileSync(join(out,'top10.txt'),profile.stdout);console.log(profile.stdout);
 }finally{built.dispose()}
}
