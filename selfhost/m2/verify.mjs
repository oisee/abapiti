// M1 independent Registry API cross-check, plus exact M2 archive coverage.
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {createRequire} from 'node:module';
import {join,resolve} from 'node:path';
import {execFileSync} from 'node:child_process';
import {buildUpstream,verifyUpstream} from '../../tools/statements-upstream.mjs';
import {compilerPin} from './prepare.mjs';
const hash=s=>createHash('sha256').update(s).digest('hex');
const out='selfhost/m2',kit='.tmp/selfhost-m2/zlnt-check-kit';
const m=JSON.parse(readFileSync(join(out,'workload.json'))),r=JSON.parse(readFileSync(join(out,'node-reference.json')));
const full=r.fullResult?readFileSync(r.fullResult.path):null;
if(full && hash(full)!==r.fullResult.sha256)throw new Error('full reference hash');
const objects=r.objects??JSON.parse(full).objects;
if(hash(JSON.stringify(objects)+'\n')!==r.normalizedSha256)throw new Error('normalized reference hash');
if(hash(readFileSync(join(out,'workload.json')))!==r.workloadSha256 || hash(readFileSync(join(out,'abaplint.json')))!==r.configSha256)throw new Error('input manifest hashes');
for(const f of m.files)if(hash(readFileSync(join(kit,f.path)))!==f.sha256)throw new Error(`file hash ${f.path}`);
const paths=m.files.filter(f=>f.checkable).map(f=>f.path);
if(readFileSync(join(kit,'files.txt'),'utf8')!==paths.join('\n')+'\n')throw new Error('files.txt');
if(readFileSync(join(kit,'deps.txt'),'utf8')!=='')throw new Error('dependencies');
const expected=readFileSync(join(kit,'expected-clean.txt'),'utf8');
if(Number(expected.split('\n')[0])!==r.totalIssues || objects.reduce((n,o)=>n+o.issues.length,0)!==r.totalIssues || Object.values(r.byRule).reduce((a,b)=>a+b,0)!==r.totalIssues)throw new Error('reference counts');
if(hash(expected)!==r.expectedFile.sha256||hash(expected.slice(0,-1))!==r.kitDumpSha256)throw new Error('kit dump hash');
const sorted=readFileSync(join(kit,'issues-sorted.txt'),'utf8');
const expectedRows=expected.trimEnd().split('\n').slice(1).sort();
if(hash(sorted)!==r.sortedIssues.sha256||expectedRows.join('\n')+(expectedRows.length?'\n':'')!==sorted)throw new Error('sorted issue hash');
verifyUpstream(resolve('.tmp/selfhost-m2/abaplint'));
if(m.compilerCommit!==compilerPin)throw new Error('compiler pin');
// Every frozen tracked compiler file must match the archive used for emission.
const entries=execFileSync('git',['ls-tree','-rz',compilerPin],{encoding:'utf8',maxBuffer:16*1024*1024}).split('\0').filter(Boolean);
let compilerFiles=0;
for(const entry of entries){
 const [meta,path]=entry.split('\t');
 if(!meta.includes(' blob '))continue;
 const frozen=execFileSync('git',['show',`${compilerPin}:${path}`],{maxBuffer:128*1024*1024});
 if(hash(readFileSync(join('.tmp/selfhost-m2/compiler',path)))!==hash(frozen))throw new Error(`compiler archive changed: ${path}`);
 compilerFiles++;
}
// Validate raw ZIP member coverage independently of the inventory writer.
execFileSync('python3',['-c',`import json,zipfile,hashlib
m=json.load(open('selfhost/m2/workload.json'))
with zipfile.ZipFile('.tmp/selfhost-m2/a4h/zlnt.zip') as z:
 assert sorted(z.namelist())==[f['path'] for f in m['files']]
 for f in m['files']:
  raw=z.read(f['path']).decode('utf-8').removeprefix('\\ufeff').replace('\\r\\n','\\n').replace('\\r','\\n').encode()
  assert hashlib.sha256(raw).hexdigest()==f['sha256']
`]);
const built=buildUpstream(resolve('.tmp/selfhost-m2/abaplint'));
try{
 const require=createRequire(import.meta.url),core=require(join(built.core,'index.js'));
 const reg=new core.Registry(new core.Config(readFileSync(join(out,'abaplint.json'),'utf8')));
 for(const path of paths)reg.addFile(new core.MemoryFile(path,readFileSync(join(kit,path),'utf8')));
 reg.parse();
 const dump=reg.findIssues().map(i=>`${i.getKey()}|${i.getSeverity()}|${i.getFilename()}|${i.getStart().getRow()}:${i.getStart().getCol()}|${i.getEnd().getRow()}:${i.getEnd().getCol()}|${i.getMessage()}`).sort();
 const wanted=expected.trimEnd().split('\n').slice(1).sort();
 if(JSON.stringify(dump)!==JSON.stringify(wanted))throw new Error('direct Registry API differs from kit harness');
 if([...reg.getObjects()].length!==m.counts.objects)throw new Error('registry object count');
 console.log(`Verified ${m.files.length} archive member hashes, ${compilerFiles} frozen compiler files, clean oracle pin, canonical hashes, and ${dump.length} issues via direct Registry API.`);
}finally{built.dispose();}
