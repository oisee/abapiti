#!/usr/bin/env node
// Read-only use of a caller-supplied pinned OSG checkout. The gogen Go module,
// frontend output and binary live under OUT, never in the OSG checkout.
import {cpSync, mkdirSync, readFileSync, readdirSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {execFileSync, spawnSync} from 'node:child_process';
import {performance} from 'node:perf_hooks';
import {protectTestBoundaries} from './osgo-test-boundaries.mjs';

const [inputArg, outArg, osgArg] = process.argv.slice(2);
if (!inputArg || !outArg || !osgArg) throw new Error('usage: abaplint-tests-osg.mjs ABAP OUT OSG');
const input=resolve(inputArg), out=resolve(outArg), osg=resolve(osgArg);
const pin=readFileSync(new URL('../.github/ci/osgo.ref',import.meta.url),'utf8').trim();
const git=(...args)=>execFileSync('git',['-C',osg,...args],{encoding:'utf8'}).trim();
if (git('rev-parse','HEAD')!==pin || git('status','--porcelain')) throw new Error('OSG must be clean at '+pin);
const tool=(name)=>import(pathToFileURL(join(osg,'tools/gogen',name)));
const {compileProgram}=await tool('frontend.mjs');
const {emitGo,referencedClasses}=await tool('emit-go.mjs');
const {unitInputs}=await tool('unit-inputs.mjs');
const config=JSON.parse(readFileSync(join(osg,'abap_transpile.json'),'utf8'));
const {folders,skip}=unitInputs({home:osg,config,fixture:input});
const objects=readdirSync(input).filter(f=>/\.(clas|intf)\.abap$/.test(f)).map(f=>f.split('.')[0].toUpperCase());
mkdirSync(out,{recursive:true});
const abapFiles=readdirSync(input).filter(f=>/\.(clas|intf)\.abap$/.test(f));
const lineBytes=abapFiles.flatMap(f=>readFileSync(join(input,f),'utf8').split('\n').map(line=>Buffer.byteLength(line)));
const envelope={files:abapFiles.length,maxLineBytes:0,overlongLines:0,maxObjectNameBytes:0};
for(const bytes of lineBytes){envelope.maxLineBytes=Math.max(envelope.maxLineBytes,bytes);if(bytes>255)envelope.overlongLines++}
for(const file of abapFiles)envelope.maxObjectNameBytes=Math.max(envelope.maxObjectNameBytes,Buffer.byteLength(file.split('.')[0]));
if(envelope.overlongLines || envelope.maxObjectNameBytes>30)throw new Error('ABAP envelope violation: '+JSON.stringify(envelope));
const timingMs={frontend:0,emit:0,goBuild:0,run:0};
let started=performance.now();
// Match unit.mjs's dependency closure, including the native exception base
// constructors. Registry visibility alone does not compile their Go symbols.
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
 program=compileProgram({folders,objects:[...wanted],tolerant:true,skip,registry,session});registry=program.reg;
 const refs=new Set([...referencedClasses(program),...program.missing]);
 for(const name of wanted){const sup=registry.getObject('CLAS',name)?.getDefinition()?.getSuperClass();if(sup)refs.add(sup.toUpperCase())}
 const more=[...refs].filter(name=>sources.has(name)&&!wanted.has(name));
 if(!more.length)break;
 for(const name of more)wanted.add(name);
 if(round===11)throw new Error('OSG dependency closure did not settle');
}
timingMs.frontend=Math.round(performance.now()-started);
const refusals={diagnostics:program.diagnostics,skipped:program.skipped,missing:[...program.missing],stubs:program.classes.flatMap(c=>(c.stubs??[]).map(m=>({class:c.name,method:m.name,reason:m.reason})))};
writeFileSync(join(out,'osg-refusals.json'),JSON.stringify(refusals,null,2)+'\n');
// Tolerant frontend stubs are retained as named runtime refusals, never empty
// methods. Driver-only boundaries record native NOT_COMPILED as a trap after
// the action/body unwinds; no source catch can intercept it. Other panics that
// escape the driver remain infrastructure failures. No registrations are made up.
started=performance.now();
const go=join(out,'go');
cpSync(join(osg,'tools/gogen/go'),go,{recursive:true,filter:src=>!/(?:^|\/)zz_[^/]+\.go$/.test(src)});
const cmd=join(go,'cmd','abapitests');mkdirSync(cmd,{recursive:true});
writeFileSync(join(cmd,'zz_generated.go'),protectTestBoundaries(emitGo(program,'main',null,true),program));
writeFileSync(join(cmd,'main.go'),`package main
import("encoding/json";"fmt";"os";"osg/gogen/abap")
var driverNames map[string]struct{ID string \x60json:"id"\x60}
func driverExceptionName(class string) string {
 if entry, ok := driverNames[class]; ok { return entry.ID }
 return class
}
func main(){
 names,err:=os.ReadFile(${JSON.stringify(join(input,'names.json'))});if err!=nil{panic(err)}
 if err=json.Unmarshal(names,&driverNames);err!=nil{panic(err)}
 defer func(){if x:=recover();x!=nil {fmt.Fprintf(os.Stderr,"OSG harness refused: %v\\n",x);os.Exit(2)}}()
 text:=ZCL_ABAPITI_TEST_RUNNER_RUN(&abap.Session{})
 var report struct {Failed int \x60json:"failed"\x60; Trapped int \x60json:"trapped"\x60; AsyncPending int \x60json:"asyncPending"\x60; Initialization []struct{Status string \x60json:"status"\x60} \x60json:"initialization"\x60}
 if err:=json.Unmarshal([]byte(text),&report);err!=nil{panic(err)}
 fmt.Println(text)
 refusedInit:=false;for _,r:=range report.Initialization{if r.Status=="failed" || r.Status=="trapped"{refusedInit=true}}
 if report.Failed+report.Trapped+report.AsyncPending>0 || refusedInit{os.Exit(1)}
}
`);
timingMs.emit=Math.round(performance.now()-started);
const binary=join(out,'abaplint-tests-osg');
started=performance.now();
const build=spawnSync('go',['build','-trimpath','-o',binary,'./cmd/abapitests'],{cwd:go,env:process.env,encoding:'utf8',maxBuffer:32*1024*1024});
timingMs.goBuild=Math.round(performance.now()-started);
writeFileSync(join(out,'build.log'),(build.stdout??'')+(build.stderr??''));
let status=2;
if(build.status===0){
 started=performance.now();
 const run=spawnSync(binary,[],{cwd:out,env:process.env,encoding:'utf8',maxBuffer:128*1024*1024});
 timingMs.run=Math.round(performance.now()-started);
 process.stdout.write(run.stdout??'');process.stderr.write(run.stderr??'');status=run.status??2;
}else process.stderr.write(build.stderr??build.error?.message??'OSG build failed');
writeFileSync(join(out,'timing.json'),JSON.stringify({pin,timingMs,status,envelope},null,2)+'\n');
process.exitCode=status;
