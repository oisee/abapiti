#!/usr/bin/env node
// Compile-cost benchmark only; never execute its signature-only dependencies.
// The factory ABAP is retained verbatim. Other implementations are removed to
// isolate the generated factory package from unrelated suite function costs.
// Usage: node tools/class-factory-bench.mjs ABAP PINNED_OSG OUT
import {readFileSync,readdirSync,writeFileSync,mkdirSync,cpSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {performance} from 'node:perf_hooks';
const args=process.argv.slice(2);
if(args.length!==3)throw Error('usage: class-factory-bench.mjs ABAP PINNED_OSG OUT');
const [abap,osg,out]=args.map(v=>resolve(v));
const load=n=>import(pathToFileURL(join(osg,'tools/gogen',n)));
const {compileProgram}=await load('frontend.mjs');
const {emitGo,referencedClasses}=await load('emit-go.mjs');
const {unitInputs}=await load('unit-inputs.mjs');
mkdirSync(out,{recursive:true});const input=join(out,'signatures');mkdirSync(input,{recursive:true});
let factory;
for(const f of readdirSync(abap).filter(f=>/\.(clas|intf)\.abap$/.test(f))){
 let src=readFileSync(join(abap,f),'utf8');
 if(src.includes('CLASS-METHODS new_args'))factory=f.split('.')[0].toUpperCase();
 else src=src.replace(/(^\s*METHOD\s+[^\n]+\n)[\s\S]*?(^\s*ENDMETHOD\.)/gmi,'$1$2');
 writeFileSync(join(input,f),src);
}
if(!factory)throw Error('no argument factory');
const {folders,skip}=unitInputs({home:osg,config:JSON.parse(readFileSync(join(osg,'abap_transpile.json'))),fixture:input});
const objects=readdirSync(input).map(f=>f.split('.')[0].toUpperCase());objects.push('CX_NO_CHECK','CX_ROOT');
let started=performance.now();
const program=compileProgram({folders,objects,tolerant:true,skip});
const f=program.classes.find(c=>c.name===factory);
if(!f || f.stubs?.length)throw Error('factory refused: '+JSON.stringify(f?.stubs));
writeFileSync(join(out,'refusals.json'),JSON.stringify({diagnostics:program.diagnostics,factoryStubs:f.stubs??[],missing:[...program.missing]},null,2));
const frontendMs=performance.now()-started;
const go=join(out,'go');cpSync(join(osg,'tools/gogen/go'),go,{recursive:true,filter:s=>!/(?:^|\/)zz_[^/]+\.go$/.test(s)});
const dir=join(go,'generated/factory');mkdirSync(dir,{recursive:true});
started=performance.now();writeFileSync(join(dir,'zz_generated.go'),emitGo(program,'factory',null,true));
writeFileSync(join(out,'timing.json'),JSON.stringify({factory,dependencyMode:'signature-only (not executable)',classes:program.classes.length,methods:f.methods.length,frontendMs,emitMs:performance.now()-started},null,2)+'\n');
