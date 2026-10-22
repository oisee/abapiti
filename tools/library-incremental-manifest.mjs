#!/usr/bin/env node
// Takes the validated frozen M1 manifest from library-tree-manifest.mjs.
import {readFileSync} from 'node:fs';
const input=JSON.parse(readFileSync(process.argv[2],'utf8'));
const file=input.Files.find(f=>f.Filename.endsWith('/zcl_osd_adt_uri.clas.abap'));
if(!file||!file.Raw.match(/\bMETHOD\s+[^.]+\./i))throw Error('missing M1 URI method');
const names=new Set(['ZCL_OSD_ADT_URI']);
for(const f of input.Files)if((f.Raw.toUpperCase().match(/(\/[A-Z0-9_]{1,10}\/)?[A-Z0-9_]{1,40}/g)||[]).includes('ZCL_OSD_ADT_URI')&&f.Filename.includes('.clas.'))names.add(f.Filename.split('/').pop().split('.')[0].toUpperCase());
input.Queries=[...names].sort().map(Name=>({Type:'CLAS',Name}));
if(process.argv.includes('--transitive')){
 const objects=new Map();
 for(const f of input.Files){
  const [name,type]=f.Filename.split('/').pop().split('.');if(!type)continue;
  const Name=name.toUpperCase().replaceAll('%23','/').replaceAll('#','/'),Type=type.toUpperCase(),key=Type+':'+Name;
  if(!objects.has(key))objects.set(key,{Type,Name,words:new Set()});
  for(const word of f.Raw.toUpperCase().match(/(\/[A-Z0-9_]{1,10}\/)?[A-Z0-9_]{1,40}/g)||[])objects.get(key).words.add(word);
 }
 const closure=new Set(['ZCL_OSD_ADT_URI']),queries=new Map();
 for(const name of closure)for(const [key,obj]of objects)if(obj.Name===name||obj.words.has(name)){closure.add(obj.Name);queries.set(key,{Type:obj.Type,Name:obj.Name});}
 input.Queries=[...queries.values()].sort((a,b)=>(a.Type+'\0'+a.Name).localeCompare(b.Type+'\0'+b.Name));
}

input.Steps=[];
for(const [Label,Raw]of [
 ['undefined',file.Raw.replace(/(\bMETHOD\s+[^.]+\.)/i,'$1\n    step3_undefined = 1.')],
 ['parser-error',file.Raw.replace(/(\bMETHOD\s+[^.]+\.)/i,'$1\n    invalid statement.')],
 ['empty',''],['comment',file.Raw+'\n* incremental comment\n'],
])input.Steps.push({Label,Updates:[{Filename:file.Filename,Raw}],Queries:input.Queries,After:[file]});
process.stdout.write(JSON.stringify(input)+'\n');
