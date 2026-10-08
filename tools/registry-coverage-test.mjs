import {test} from 'node:test';
import {spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {strict as assert} from 'node:assert';
import {mkdtempSync,mkdirSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createRequire} from 'node:module';
import {validateWorkload} from './registry-workload.mjs';
import {declarationCandidates} from './registry-coverage-identity.mjs';
const ts=createRequire(import.meta.url)('/home/alice/dev/abaplint/node_modules/typescript');
test('class header mapping must not hide an executed constructor',()=>{
 const sf=ts.createSourceFile('probe.ts','export class Probe { constructor() { console.log(1); } }',ts.ScriptTarget.Latest,true);
 const ctor=sf.statements[0].members[0];
 const mappings=[{offset:0,originalLine:1,originalColumn:0},{offset:10,originalLine:1,originalColumn:ctor.body.getStart(sf)+1}];
 const fn={functionName:'Probe',ranges:[{startOffset:0,endOffset:60,count:1}]};
 assert.deepEqual(declarationCandidates(ts,sf,[ctor],mappings,fn,'Constructor'),[ctor]);
 // Two source declarations with the same name and kind are not resolved by
 // selecting whichever header happens to occur first in a generated range.
 const other=ts.createSourceFile('p.ts','class A { same() {} } class B { same() {} }',ts.ScriptTarget.Latest,true);
 const candidates=other.statements.map(n=>n.members[0]);
 const headers=candidates.map((n,i)=>({offset:i,originalLine:1,originalColumn:n.getStart(other)}));
 assert.equal(declarationCandidates(ts,other,candidates,headers,{functionName:'same',ranges:[{startOffset:0,endOffset:20}]},'MethodDeclaration').length,2);
 assert.equal(declarationCandidates(ts,other,candidates,headers,{functionName:'same',ranges:[{startOffset:0,endOffset:20}]},'GetAccessor').length,0);
});
test('empty workloads and missing or repeated negative targets fail',()=>{
 const root=mkdtempSync(join(tmpdir(),'coverage-workload-'));
 const input=join(root,'input'),deps=join(root,'deps'),negative=join(root,'negative.json');
 mkdirSync(input);mkdirSync(deps);writeFileSync(negative,'[]');
 try {
  assert.throws(()=>validateWorkload(input,deps,negative),/main inputs/);
  for(const n of ['zabapgit_standalone.prog.abap','zabapgit_standalone.prog.xml']) writeFileSync(join(input,n),'populated');
  assert.throws(()=>validateWorkload(input,deps,negative),/dependency/);
  for(let i=0;i<360;i++) writeFileSync(join(deps,i+(i%2?'.xml':'.abap')),'populated');
  assert.throws(()=>validateWorkload(input,deps,negative),/six distinct/);
  const targets=['check_syntax','unknown_types','implement_methods','superclass_final','parser_error','allowed_object_naming'];
  const variants=targets.map(target=>({target,filename:'zprobe.prog.abap',edit:'WRITE.'}));
  writeFileSync(negative,JSON.stringify(variants));validateWorkload(input,deps,negative);
  variants[5].target=targets[0];writeFileSync(negative,JSON.stringify(variants));
  assert.throws(()=>validateWorkload(input,deps,negative),/six distinct/);
 } finally {rmSync(root,{recursive:true,force:true});}
});

test('collector rejects empty workload before building upstream',()=>{
 const root=mkdtempSync(join(tmpdir(),'coverage-empty-cli-'));
 try {
  const input=join(root,'input'),deps=join(root,'deps');mkdirSync(input);mkdirSync(deps);
  const config=join(root,'config.json'),negative=join(root,'negative.json');writeFileSync(config,'{}');writeFileSync(negative,'[]');
  const result=spawnSync(process.execPath,[fileURLToPath(new URL('./registry-coverage.mjs',import.meta.url)),join(root,'out'),input,deps,config,negative,join(root,'missing-upstream')],{encoding:'utf8',timeout:1800000});
  assert.match(result.stderr,/expected populated north-star main inputs/);
  assert.notEqual(result.status,0);
 } finally {rmSync(root,{recursive:true,force:true});}
});
