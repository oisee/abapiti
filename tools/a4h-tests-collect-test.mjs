import {test} from 'node:test';
import assert from 'node:assert/strict';
import {collectResults,classifyFailure} from './a4h-tests-collect.mjs';
import {reconcileResults} from './abaplint-tests-report.mjs';
const pass={title:'a',status:'passed',passed:true,message:''};
const fail={title:'b',status:'failed',passed:false,message:'FIND PCRE: match limit exceeded'};
test('PCRE is an additional category, never a pass or suppressed FAIL',()=>{
 const r=collectResults({results:[pass,fail],initialization:[]});
 assert.equal(r.categories['pcre-drift'],1);assert.equal(r.failed,1);
 assert.equal(reconcileResults({titles:['a','b']},r).failed,1);
 assert.equal(classifyFailure({...fail,message:'expected regex value'}).category,undefined);
 assert.equal(classifyFailure({...pass,message:'FIND PCRE'}).category,undefined);
 assert.equal(classifyFailure({...fail,message:'CX_SY_REGEX_TOO_COMPLEX'}).category,'pcre-drift');
});
test('ordered chunk rows and dump resume retain every executed test',()=>{
 const events=[{kind:'test',index:1,title:'a'},{kind:'result',index:1,record:pass},{kind:'test',index:2,title:'b'},{kind:'resume',startIndex:3},{kind:'complete',report:{results:[]}}];
 const rows=events.flatMap((e,i)=>{const text=JSON.stringify(e);return Array.from({length:Math.ceil(text.length/15)},(_,j)=>({RUN_ID:'run',SEQ:i+1,CHUNK_NO:j+1,USED:Math.min(15,text.length-j*15),SHARD:'utils',PAYLOAD:text.slice(j*15,j*15+15).padEnd(255,' ')}))}).reverse();
 const r=collectResults(rows,{shard:'utils',dumps:{2:'REPLACE PCRE: kernel dump'}});
 assert.equal(r.passed,1);assert.equal(r.trapped,1);assert.equal(r.categories['pcre-drift'],1);
 assert.deepEqual(r.results.map(r=>r.title),['a','b']);
 assert.throws(()=>collectResults(rows.slice(1),{shard:'utils'}),/chunk|JSON/);
 assert.throws(()=>collectResults(rows,{shard:'rules'}),/mixed shard/);
});
test('incomplete, duplicate and malformed verdicts fail closed',()=>{
 assert.throws(()=>collectResults([{kind:'test',index:1,title:'a'}]),/incomplete/);
 assert.throws(()=>collectResults({results:[{...pass,passed:false}]}),/invalid/);
 assert.throws(()=>collectResults([{kind:'test',index:1,title:'a'},{kind:'result',index:1,record:pass},{kind:'result',index:1,record:pass}]),/duplicate/);
});
