#!/usr/bin/env node
// Isolate pinned gogen module globals from the subsequent Go compiler process.
import {readFileSync, readdirSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {performance} from 'node:perf_hooks';
import {wireCheckpoints} from './osg-runner-resume.mjs';
import {runnerMain} from './osg-runner-main.mjs';
import {protectTestBoundaries} from './osgo-test-boundaries.mjs';
import {adaptTestRegex} from './osgo-test-regex.mjs';

const [inputArg, outArg, osgArg, cacheArg] = process.argv.slice(2);
const input=resolve(inputArg), out=resolve(outArg), osg=resolve(osgArg);
const cacheRoot=cacheArg || null;
const tool=name=>import(pathToFileURL(join(osg,'tools/gogen',name)));
const {unitInputs}=await tool('unit-inputs.mjs');
const config=JSON.parse(readFileSync(join(osg,'abap_transpile.json'),'utf8'));
const {folders,skip}=unitInputs({home:osg,config,fixture:input});
const abapFiles=readdirSync(input).filter(f=>/\.(clas|intf)\.abap$/.test(f));
const cmd=join(out,'go/cmd/abapitests');
const generated={'zz_generated.go':join(cmd,'zz_generated.go'),'main.go':join(cmd,'main.go')};
const timingMs={frontend:0,emit:0}, cache={};
let started,refusals,regexAdapter;
const {compileProgram}=await tool('frontend.mjs');
const {emitGo,referencedClasses}=await tool('emit-go.mjs');
// Newer gogen pins accept a warm per-object frontend cache (byte-identical
// output). Feature-detect it; older pins never see the extra argument.
const objectCache=process.env.ABAPITI_OSG_OBJECT_CACHE==='1';
const frontendCache=cacheRoot && objectCache && /\bcacheDir\b/.test(compileProgram.toString()) ? {cacheDir:join(cacheRoot,'gogen-frontend')} : {};
cache.gogenFrontend=frontendCache.cacheDir ? 'enabled' : objectCache ? 'unsupported' : 'disabled';
started=performance.now();
const objects=abapFiles.map(f=>f.split('.')[0].toUpperCase());
// Match unit.mjs's dependency closure, including native exception bases.
const walk=dir=>readdirSync(dir,{withFileTypes:true}).flatMap(e=>e.isDirectory()?walk(join(dir,e.name)):[join(dir,e.name)]);
const sources=new Map(folders.flatMap(walk).filter(f=>!skip(f)&&/\.clas\.abap$/.test(f)).map(f=>[f.split('/').at(-1).split('.')[0].replaceAll('#','/').toUpperCase(),f]));
const wanted=new Set(objects), queue=[...objects];
while(queue.length){
 const file=sources.get(queue.shift());if(!file)continue;
 for(const match of readFileSync(file,'utf8').matchAll(/\b(?:CL|CX|ZCL|ZCX)_[A-Z0-9_]+\b/gi)){
  const name=match[0].toUpperCase();if(sources.has(name)&&!wanted.has(name)){wanted.add(name);queue.push(name)}
 }
}
let program,registry;
const session={};
for(let round=0;round<12;round++){
 program=compileProgram({folders,objects:[...wanted],tolerant:true,skip,registry,session,...frontendCache});registry=program.reg;
 const refs=new Set([...referencedClasses(program),...program.missing]);
 for(const name of wanted){const sup=registry.getObject('CLAS',name)?.getDefinition()?.getSuperClass();if(sup)refs.add(sup.toUpperCase())}
 const more=[...refs].filter(name=>sources.has(name)&&!wanted.has(name));
 if(!more.length)break;
 for(const name of more)wanted.add(name);
 if(round===11)throw new Error('OSG dependency closure did not settle');
}
timingMs.frontend=Math.round(performance.now()-started);
// names.json is part of the fixture folder, hence of frontKey.
regexAdapter=adaptTestRegex(program,JSON.parse(readFileSync(join(input,'names.json'),'utf8')));
// Tolerant stubs remain named runtime refusals. Driver-only boundaries keep
// source catches separate from native NOT_COMPILED. The parent records host
// dumps at the active checkpoint and restarts; no results are fabricated.
refusals={diagnostics:program.diagnostics,skipped:program.skipped,missing:[...program.missing],stubs:program.classes.flatMap(c=>(c.stubs??[]).map(m=>({class:c.name,method:m.name,reason:m.reason})))};
started=performance.now();
writeFileSync(generated['zz_generated.go'],wireCheckpoints(protectTestBoundaries(emitGo(program,'main',null,true),program,{resume:true})));
writeFileSync(generated['main.go'],runnerMain);
timingMs.emit=Math.round(performance.now()-started);
writeFileSync(join(out,'frontend.json'),JSON.stringify({refusals,regexAdapter,timingMs,cache,nodePeakRSSKiB:process.resourceUsage().maxRSS})+'\n');
