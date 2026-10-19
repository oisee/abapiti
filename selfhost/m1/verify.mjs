// Independently check hashes, source-clean pins and the direct Registry API.
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {createRequire} from 'node:module';
import {join,resolve} from 'node:path';
import {execFileSync} from 'node:child_process';
import {buildUpstream} from '../../tools/statements-upstream.mjs';
const hash=s=>createHash('sha256').update(s).digest('hex');
const m=JSON.parse(readFileSync('selfhost/m1/workload.json'));
const r=JSON.parse(readFileSync('selfhost/m1/node-reference.json'));
const objects=r.objects??JSON.parse(readFileSync('.tmp/selfhost/node-reference-full.json')).objects;
if(hash(JSON.stringify(objects)+'\n')!==r.normalizedSha256)throw new Error('normalized reference hash');
if(hash(readFileSync('selfhost/m1/workload.json'))!==r.workloadSha256 || hash(readFileSync('selfhost/m1/abaplint.json'))!==r.configSha256)throw new Error('input manifest hashes');
for(const file of m.files)if(hash(readFileSync(join('.tmp/selfhost/osg-check-kit',file.path)))!==file.sha256)throw new Error(`file hash ${file.path}`);
for(const [dir,pin] of [['.osg',m.osgSha],...m.libs.map(l=>[`.osg/.local/lars/${l.folder}`,l.ref])]){
 if(execFileSync('git',['-C',dir,'rev-parse','HEAD'],{encoding:'utf8'}).trim()!==pin)throw new Error(`pin ${dir}`);
 if(execFileSync('git',['-C',dir,'status','--porcelain','--untracked-files=no'],{encoding:'utf8'}).trim())throw new Error(`source changes ${dir}`);
}
const built=buildUpstream(resolve('.tmp/abaplint'));
try{
 const require=createRequire(import.meta.url),core=require(join(built.core,'index.js'));
 const reg=new core.Registry(new core.Config(readFileSync('selfhost/m1/abaplint.json','utf8')));
 for(const f of m.files)reg.addFile(new core.MemoryFile(f.path,readFileSync(join('.tmp/selfhost/osg-check-kit',f.path),'utf8')));
 reg.parse();
 const dump=reg.findIssues().map(i=>`${i.getKey()}|${i.getSeverity()}|${i.getFilename()}|${i.getStart().getRow()}:${i.getStart().getCol()}|${i.getEnd().getRow()}:${i.getEnd().getCol()}|${i.getMessage()}`).sort();
 const expected=readFileSync('.tmp/selfhost/osg-check-kit/expected-clean.txt','utf8').trimEnd().split('\n').slice(1).sort();
 if(JSON.stringify(dump)!==JSON.stringify(expected))throw new Error('direct Registry API differs from kit harness');
 console.log(`Verified ${m.files.length} file hashes, ${m.libs.length+1} frozen source-clean checkouts, normalized SHA-256, and ${dump.length} issues via direct Registry API.`);
}finally{built.dispose();}
