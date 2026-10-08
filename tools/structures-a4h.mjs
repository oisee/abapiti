#!/usr/bin/env node
// Package generated files without modifying their ABAP. Benchmark source is
// embedded as quoted base64 data; the only executable entry point is ours.
import {copyFileSync,mkdirSync,readdirSync,writeFileSync} from "node:fs";
import {join} from "node:path";
const [generated,out]=process.argv.slice(2);
if(!generated||!out)throw new Error("usage: structures-a4h.mjs generated-dir output-dir");
mkdirSync(join(out,"src"),{recursive:true});
writeFileSync(join(out,".abapgit.xml"),`<?xml version="1.0" encoding="utf-8"?>
<abapGit version="v1.0.0"><asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0"><asx:values><DATA><MASTER_LANGUAGE>E</MASTER_LANGUAGE><STARTING_FOLDER>/src/</STARTING_FOLDER><FOLDER_LOGIC>PREFIX</FOLDER_LOGIC></DATA></asx:values></asx:abap></abapGit>\n`);
const objects=[];
for(const n of readdirSync(generated).sort()){
 const m=n.match(/^(.+)\.(clas|intf|prog)\.abap$/);if(!m)continue;
 copyFileSync(join(generated,n),join(out,"src",n));
 const name=m[1].toUpperCase(),kind=m[2].toUpperCase();objects.push(`${kind} ${name}`);
 let metadata;
 if(kind==="CLAS")metadata=`<VSEOCLASS><CLSNAME>${name}</CLSNAME><LANGU>E</LANGU><DESCRIPT>Translated abaplint phase 3</DESCRIPT><STATE>1</STATE><CLSCCINCL>X</CLSCCINCL><FIXPT>X</FIXPT><UNICODE>X</UNICODE></VSEOCLASS>`;
 else if(kind==="INTF")metadata=`<VSEOINTERF><CLSNAME>${name}</CLSNAME><LANGU>E</LANGU><DESCRIPT>Translated abaplint interface</DESCRIPT><EXPOSURE>2</EXPOSURE><STATE>1</STATE><UNICODE>X</UNICODE></VSEOINTERF>`;
 else metadata=`<PROGDIR><NAME>${name}</NAME><SUBC>1</SUBC><FIXPT>X</FIXPT><UCCHECK>X</UCCHECK></PROGDIR><TPOOL><item><ID>R</ID><ENTRY>Phase 3 parser benchmark</ENTRY><LENGTH>24</LENGTH></item></TPOOL>`;
 const serializer={CLAS:"LCL_OBJECT_CLAS",INTF:"LCL_OBJECT_INTF",PROG:"LCL_OBJECT_PROG"}[kind];
 writeFileSync(join(out,"src",`${m[1]}.${m[2]}.xml`),`<?xml version="1.0" encoding="utf-8"?>\n<abapGit version="v1.0.0" serializer="${serializer}" serializer_version="v1.0.0"><asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0"><asx:values>${metadata}</asx:values></asx:abap></abapGit>\n`);
}
if(!objects.includes('PROG ZPHASE3_STRUCTURES_BENCH'))throw new Error("benchmark PROG is missing");
writeFileSync(join(out,"objects.txt"),objects.join("\n")+"\n");
console.log(`${objects.length} abapGit objects; report ZPHASE3_STRUCTURES_BENCH`);
