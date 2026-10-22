#!/usr/bin/env node
// Takes the validated frozen M1 manifest from library-tree-manifest.mjs.
import {readFileSync} from 'node:fs';
const input=JSON.parse(readFileSync(process.argv[2],'utf8'));
const file=input.Files.find(f=>f.Filename.endsWith('/zcl_osd_adt_uri.clas.abap'));
if(!file||!file.Raw.match(/\bMETHOD\s+[^.]+\./i))throw Error('missing M1 URI method');
const names=new Set(['ZCL_OSD_ADT_URI']);
for(const f of input.Files)if((f.Raw.toUpperCase().match(/(\/[A-Z0-9_]{1,10}\/)?[A-Z0-9_]{1,40}/g)||[]).includes('ZCL_OSD_ADT_URI')&&f.Filename.includes('.clas.'))names.add(f.Filename.split('/').pop().split('.')[0].toUpperCase());
input.Queries=[...names].sort().map(Name=>({Type:'CLAS',Name}));
input.Steps=[];
for(const [Label,Raw]of [
 ['undefined',file.Raw.replace(/(\bMETHOD\s+[^.]+\.)/i,'$1\n    step3_undefined = 1.')],
 ['parser-error',file.Raw.replace(/(\bMETHOD\s+[^.]+\.)/i,'$1\n    invalid statement.')],
 ['empty',''],['comment',file.Raw+'\n* incremental comment\n'],
])input.Steps.push({Label,Updates:[{Filename:file.Filename,Raw}],Queries:input.Queries,After:[file]});
process.stdout.write(JSON.stringify(input)+'\n');
