#!/usr/bin/env node
// Original pinned Node oracle for Update -> CheckObject/Tree -> restore.
// Usage: node tools/library-incremental-reference.mjs CORE_DIR MANIFEST [--native]
// The host invalidates direct lexical readers, including comments and strings.
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {createRequire} from 'node:module';
const require=createRequire(import.meta.url),path=resolve(process.argv[2]);
if(require(resolve(path,'package.json')).version!=='2.120.73')throw Error('expected core 2.120.73');
const {Registry,Config,MemoryFile,ABAPObject,Nodes,Tokens}=require(path);
const input=JSON.parse(readFileSync(process.argv[3],'utf8'));
const registry=new Registry(new Config(input.Config));
for(const f of input.Files)registry.addFile(new MemoryFile(f.Filename,f.Raw));
registry.parse();
const pos=p=>({Row:p.getRow(),Col:p.getCol()});
const token=t=>({Str:t.getStr(),Start:pos(t.getStart()),End:pos(t.getEnd()),Pragma:t instanceof Tokens.Pragma});
const statement=n=>({Kind:n.get().constructor.name,Statement:true,Start:pos(n.getFirstToken().getStart()),End:pos(n.getLastToken().getEnd()),Tokens:n.getTokens().map(token),Children:null,FirstStatement:null});
function structure(n){
 const start=n.getChildren().length?pos(n.getFirstToken().getStart()):{Row:0,Col:0};
 return {Kind:n.get().constructor.name,Statement:false,Start:start,End:n.getChildren().length?pos(n.getLastToken().getEnd()):start,Tokens:null,Children:n.getChildren().map(c=>c instanceof Nodes.StatementNode?statement(c):structure(c)),FirstStatement:null};
}
const issue=i=>({Filename:i.getFilename(),Key:i.getKey(),Severity:i.getSeverity(),Message:i.getMessage(),Row:i.getStart().getRow(),Column:i.getStart().getCol(),EndRow:i.getEnd().getRow(),EndColumn:i.getEnd().getCol()});
const json=v=>JSON.stringify(v).replace(/[<>&\u2028\u2029]/g,c=>'\\u'+c.charCodeAt(0).toString(16).padStart(4,'0'));
function dump(Label,queries){for(const {Type,Name}of queries){
 const o=registry.getObject(Type,Name),Issues=o?registry.findIssuesObject(o).map(issue):null;
 let Files=o?[]:null;
 if(o instanceof ABAPObject){
  let files=o.getABAPFiles();
  if(Type==='CLAS'||Type==='INTF')files=o.getSequencedFiles();
  else {const main=o.getMainABAPFile();files=[...(main?[main]:[]),...files.filter(f=>f!==main)];}
  Files=files.map(f=>({Filename:f.getFilename(),Structure:f.getStructure()?structure(f.getStructure()):null,Statements:f.getStatements().map(statement)}));
 }
 process.stdout.write(json({Label,Type,Name,Found:!!o,Issues,Files})+'\n');
}}
function update({Filename,Raw}){
 const before=registry.getFileByName(Filename);
 if(Raw===null&&!before||Raw!==null&&before?.getRaw()===Raw)return;
 const file=Raw===null?before:new MemoryFile(Filename,Raw),name=file.getObjectName().toUpperCase();
 if(Raw===null)registry.removeFile(before);else if(before)registry.updateFile(file);else registry.addFile(file);
 const direct=[];
 if(!process.argv.includes('--native'))for(const o of registry.getObjects()){
  if(o.getFiles().some(f=>(f.getRaw().toUpperCase().match(/(\/[A-Z0-9_]{1,10}\/)?[A-Z0-9_]{1,40}/g)||[]).includes(name))){o.setDirty();direct.push(o.getType()+':'+o.getName());}
 }
 console.error(JSON.stringify({Filename,Direct:direct.sort(),DirtyABAP:[...registry.getObjects()].filter(o=>o instanceof ABAPObject&&o.isDirty()).map(o=>o.getType()+':'+o.getName()).sort()}));
}
dump('baseline',input.Queries);
for(const s of input.Steps??[]){for(const u of s.Updates)update(u);dump(s.Label,s.Queries);if(s.After?.length){for(const u of s.After)update(u);dump(s.Label+'-restored',s.Queries);}}
