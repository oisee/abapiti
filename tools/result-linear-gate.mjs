#!/usr/bin/env node
// Result-only Go debug experiment. Canonical reports are compared in full;
// refusal status is recorded separately and is never an equality success.
import {spawnSync, execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {createRequire} from 'node:module';
import {basename, join, resolve} from 'node:path';
import {existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync} from 'node:fs';
import {buildUpstream} from './statements-upstream.mjs';

const args=Object.fromEntries(process.argv.slice(2).map(x=>{const i=x.indexOf('=');if(!x.startsWith('--')||i<3)throw Error('use --name=value');return [x.slice(2,i),x.slice(i+1)];}));
for(const key of ['normal','ownership','value','upstream','abapgit','zabapgit','out']){if(!args[key])throw Error('missing '+key);args[key]=resolve(args[key]);}
if(args.osg)args.osg=resolve(args.osg);
mkdirSync(args.out,{recursive:true});
const walk=dir=>readdirSync(dir).sort().flatMap(n=>{const p=join(dir,n);return statSync(p).isDirectory()&&!n.startsWith('.')?walk(p):[p];});
const sourceFiles=dir=>walk(dir).filter(p=>p.endsWith('.abap')||p.endsWith('.xml'));
const kit=args.zabapgit;
const dependencies=readFileSync(join(kit,'deps.txt'),'utf8').split(/\r?\n/).filter(Boolean).map(p=>resolve(kit,p));
const depList=join(args.out,'dependencies.txt');writeFileSync(depList,dependencies.join('\n')+'\n');
const config=readFileSync(join(kit,'abaplint.json'),'utf8');const configPath=join(args.out,'abaplint.json');writeFileSync(configPath,config);
const interfaces=sourceFiles(join(args.abapgit,'src')).filter(p=>p.endsWith('.intf.abap')).sort((a,b)=>statSync(a).size-statSync(b).size||a.localeCompare(b)).slice(0,5);
const inputSets={clean:[join(kit,'zabapgit_standalone.prog.abap')],seeded:[join(kit,'seeded/zabapgit_standalone.prog.abap')],
 'abapgit-interfaces':sourceFiles(join(args.abapgit,'src')).filter(p=>interfaces.includes(p)||interfaces.includes(p.replace(/\.xml$/,'.abap'))),
 'abapgit-full':sourceFiles(join(args.abapgit,'src'))};
if(args.osg)inputSets['osg-full']=sourceFiles(args.osg);
const requested=args.corpus?args.corpus.split(','):Object.keys(inputSets);
const build=buildUpstream(args.upstream);const require=createRequire(import.meta.url);
const {Registry}=require(join(build.core,'registry.js'));const {Config}=require(join(build.core,'config.js'));const {MemoryFile}=require(join(build.core,'files/memory_file.js'));
const sha=b=>createHash('sha256').update(b).digest('hex');
const results=existsSync(join(args.out,'results.json'))?JSON.parse(readFileSync(join(args.out,'results.json'),'utf8')):{schema:1,upstream_pin:execFileSync('git',['-C',args.upstream,'rev-parse','HEAD'],{encoding:'utf8'}).trim(),corpora:{}};
try{
 for(const label of requested){
  const inputs=inputSets[label];if(!inputs)throw Error('unknown corpus '+label);
  console.log(label+': '+inputs.length+' source/XML files; fresh vanilla Node oracle');
  const manifest=inputs.map(p=>({file:p,sha256:sha(readFileSync(p))}));writeFileSync(join(args.out,label+'-manifest.json'),JSON.stringify(manifest,null,2)+'\n');
  const reg=new Registry(new Config(config));for(const p of inputs)reg.addFile(new MemoryFile(basename(p),readFileSync(p,'utf8')));for(const p of dependencies)reg.addDependency(new MemoryFile(basename(p),readFileSync(p,'utf8')));reg.parse();const issues=reg.findIssues();
  const oracle=String(issues.length)+issues.map(i=>`\n${i.getKey()}|${i.getSeverity()}|${i.getFilename()}|${i.getStart().getRow()}:${i.getStart().getCol()}|${i.getEnd().getRow()}:${i.getEnd().getCol()}|${i.getMessage()}`).join('')+'\n';
  writeFileSync(join(args.out,label+'-oracle.txt'),oracle);
  if(['clean','seeded'].includes(label)&&readFileSync(join(kit,'expected-'+label+'.txt'),'utf8')!==oracle)throw Error('fresh Node differs from kit '+label);
  const list=join(args.out,label+'-files.txt');writeFileSync(list,inputs.join('\n')+'\n');
  const row={files:inputs.length,abap:inputs.filter(p=>p.endsWith('.abap')).length,oracle_issues:issues.length,oracle_sha256:sha(oracle),runs:results.corpora[label]?.runs??{}};if(results.corpora[label]&&results.corpora[label].oracle_sha256!==row.oracle_sha256)throw Error("oracle changed on resume");results.corpora[label]=row;
  for(const mode of (args.modes?args.modes.split(','):['normal','value','ownership'])){
   console.log(label+' '+mode+': running');const started=Date.now();
   const run=spawnSync(args[mode],['--files-list',list,'--config',configPath,'--deps',depList],{encoding:'utf8',maxBuffer:128*1024*1024,timeout:600000,env:{...process.env,ABAPITI_RESULT_REPORT:join(args.out,label+'-ownership.json')}});
   writeFileSync(join(args.out,label+'-'+mode+'.txt'),run.stdout??'');writeFileSync(join(args.out,label+'-'+mode+'.err'),run.stderr??'');
   row.runs[mode]={status:run.status,error:run.error?.message,seconds:(Date.now()-started)/1000,oracle_equal:run.status===0&&run.stdout===oracle,sha256:sha(run.stdout??'')};
   console.log(label+' '+mode+': '+JSON.stringify(row.runs[mode]));
  }
  const normal=row.runs.normal;row.value_equal=normal.status===0&&row.runs.value?.status===0&&normal.sha256===row.runs.value.sha256;row.monitor_output_equal=normal.status===0&&row.runs.ownership?.status===0&&normal.sha256===row.runs.ownership.sha256;
  writeFileSync(join(args.out,'results.json'),JSON.stringify(results,null,2)+'\n');
 }
}finally{build.dispose();writeFileSync(join(args.out,'results.json'),JSON.stringify(results,null,2)+'\n');}
