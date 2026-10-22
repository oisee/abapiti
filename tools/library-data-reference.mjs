#!/usr/bin/env node
// Independent original Node @abaplint/core 2.120.73 data API oracle.
// Usage: node tools/library-data-reference.mjs CORE_DIR MANIFEST [--legacy]
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {createRequire} from 'node:module';
const require=createRequire(import.meta.url),corePath=resolve(process.argv[2]);
if(require(resolve(corePath,'package.json')).version!=='2.120.73')throw new Error('expected core 2.120.73');
const {Registry,Config,MemoryFile,Nodes,BasicTypes}=require(corePath);
const input=JSON.parse(readFileSync(process.argv[3],'utf8')),legacy=process.argv.includes('--legacy');
const registry=new Registry(new Config(input.Config));
for(const f of input.Files)registry.addFile(new MemoryFile(f.Filename,f.Raw));
registry.parse();
const pos=p=>({Row:p.getRow(),Col:p.getCol()});
function expression(n){
 counts.Expressions++;
 const out={Kind:n.get().constructor.name,Text:n.concatTokens()};
 if(!legacy){out.Start=pos(n.getFirstToken().getStart());out.End=pos(n.getLastToken().getEnd());}
 const children=n.getChildren().filter(c=>c instanceof Nodes.ExpressionNode).map(expression);
 out.Children=children.length?children:null;return out;
}
const counts={Snapshots:0,Found:0,DDLS:0,TABL:0,VIEW:0,Expressions:0};
function inspect(Type,Name){
 registry.parse();const obj=registry.getObject(Type,Name);
 const out={Found:obj!==undefined,Files:null,Tree:null,SQLView:legacy?'':null,Components:null,Keys:null};
 counts.Snapshots++;
 if(!obj)return out;
 counts.Found++;if(Type in counts)counts[Type]++;
 const files=obj.getFiles().map(f=>({Filename:f.getFilename(),Raw:f.getRaw()}));out.Files=files.length?files:null;
 if(Type==='DDLS'){
  const tree=obj.getTree();out.Tree=tree===undefined?null:expression(tree);
  const sql=obj.getSQLViewName();out.SQLView=legacy?(sql??''):(sql===undefined?null:sql);
 }else if(Type==='TABL'||Type==='VIEW'){
  const parsed=obj.parseType(registry,undefined,undefined);
  if(parsed instanceof BasicTypes.StructureType)out.Components=parsed.getComponents().map(c=>c.name);
  const keys=Type==='TABL'?obj.listKeys(registry):obj.listKeys();out.Keys=keys.length?keys:null;
 }
 return out;
}
const json=v=>JSON.stringify(v).replace(/[<>&\u2028\u2029]/g,c=>'\\u'+c.charCodeAt(0).toString(16).padStart(4,'0'));
const queries=input.Queries??[...registry.getObjects()].filter(o=>['DDLS','TABL','VIEW'].includes(o.getType())).map(o=>({Type:o.getType(),Name:o.getName()})).sort((a,b)=>(a.Type+'\0'+a.Name)<(b.Type+'\0'+b.Name)?-1:1);
const dump=(Label,Queries)=>{for(const {Type,Name}of Queries)process.stdout.write(json({Label,Type,Name,Value:inspect(Type,Name)})+'\n');};
function update({Filename,Raw}){
 const before=registry.getFileByName(Filename);
 if(Raw===null){if(before)registry.removeFile(before);}
 else if(before)registry.updateFile(new MemoryFile(Filename,Raw));
 else registry.addFile(new MemoryFile(Filename,Raw));
}
dump('baseline',queries);
for(const s of input.Steps??[]){for(const u of s.Updates)update(u);dump(s.Label,s.Queries);if(s.After?.length){for(const u of s.After)update(u);dump(s.Label+'-restored',s.Queries);}}
console.error(JSON.stringify(counts));
