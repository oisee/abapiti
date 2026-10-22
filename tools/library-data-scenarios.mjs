#!/usr/bin/env node
// Extract the 41 PR #839 mutations without its host/ADT/build dependencies.
// Usage: node tools/library-data-scenarios.mjs OWN_OSG_PR839_CLONE > scenarios.json
import {readFileSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {runInNewContext} from 'node:vm';
import {createHash} from 'node:crypto';
const root=resolve(process.argv[2]);
const source=readFileSync(join(root,'tools/step3/check-outline-diff.mjs'),'utf8');
const sourceSHA256=createHash('sha256').update(source).digest('hex');
if(sourceSHA256!=='5431a72a665deaf8fe1bb8e0a0e3c5575097fb897acaeaf260f5fe3b9265a8cf')throw new Error('expected the pinned PR #839 scenario source');
function constant(name) {
 const match=source.match(new RegExp(`const ${name}=([\\s\\S]*?);\\n`));
 if(!match)throw new Error(`missing ${name}`);
 return runInNewContext(match[1],{readFileSync,join,originalRoot:root});
}
const mutationFiles=constant('mutationFiles'),shapes=constant('shapes'),dictionary=constant('dictionary'),casts=constant('casts');
const Files=[],Steps=[];
const query=(Type,Name)=>({Type,Name});
const Queries=[query('CLAS','ZNATIVE_OUTLINE'),query('DDLS','MISSING'),query('TABL','MISSING'),query('VIEW','MISSING')];
let current=mutationFiles.map(f=>({...f})),saved=[];
function step(Label,files,queries,after=[]) {
 const Updates=[];
 for(const old of saved)if(!files.some(f=>f.Filename===old.Filename))Updates.push({Filename:old.Filename,Raw:null});
 for(const f of files)if(!saved.some(o=>o.Filename===f.Filename&&o.Raw===f.Raw))Updates.push({...f});
 Steps.push({Label,Updates,Queries:queries,After:after});saved=files.map(f=>({...f}));
}
for(const label of ['locals-tests-macros','edited','broken','empty','include-removed','include-added']) {
 if(label==='edited')current[0].Raw=current[0].Raw.replace('METHOD run.',"METHOD run. WRITE 'edited'.");
 if(label==='broken')current[0].Raw='CLASS znative_outline DEFINITION PUBLIC. PUBLIC SECTION. METHODS run. ENDCLASS. CLASS znative_outline IMPLEMENTATION. METHOD run. invalid statement. ENDMETHOD. ENDCLASS.';
 if(label==='empty')current[0].Raw='';
 if(label==='include-removed')current=current.filter(f=>!f.Filename.includes('locals_imp'));
 if(label==='include-added')current.push({...mutationFiles[2]});
 step(label,current,[query('CLAS','ZNATIVE_OUTLINE')]);
}
for(const [i,source] of ['',mutationFiles[0].Raw,'broken inline.'].entries()) {
 const overlay=current.map((f,j)=>j===0?{...f,Raw:source}:{...f});
 const restore=[{...current[0]}];
 step(`inline-${i}`,overlay,[query('CLAS','ZNATIVE_OUTLINE')],restore);
 saved=current.map(f=>({...f}));
}
for(const [type,suffix,variants] of shapes) {
 for(const [i,Raw]of variants.entries())step(`${type}-${i}`,[{Filename:'/src/znative_mut'+suffix,Raw}],[query(type==='INCL'?'PROG':type,'ZNATIVE_MUT')]);
}
for(const [i,fields]of casts.entries()) {
 const Raw="@AbapCatalog.sqlViewName: 'ZNATIVE_GEN'\n@ObjectModel.writeEnabled: true\ndefine view znative_mut as select from zstg_demo { "+fields+' }';
 step(`dictionary-${i}`,[...dictionary,{Filename:'/src/znative_mut.ddls.asddls',Raw}],[query('DDLS','ZNATIVE_MUT'),query('TABL','ZSTG_DEMO'),query('VIEW','ZNATIVE_GEN')]);
}
if(Steps.length!==41)throw new Error(`expected 41 scenarios, got ${Steps.length}`);
const Config=JSON.stringify({syntax:{version:'open-abap',errorNamespace:'.'},rules:{parser_error:true,check_syntax:true}});
console.error(JSON.stringify({Scenarios:Steps.length,SourceSHA256:sourceSHA256}));
process.stdout.write(JSON.stringify({SourceSHA256:sourceSHA256,Files,Config,Queries,Steps},null,2)+'\n');
