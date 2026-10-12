#!/usr/bin/env node
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
const dir=mkdtempSync(join(tmpdir(),'abaplint-report-'));
try {
 const oracle={total:6,titles:['duplicate','duplicate','async','pending','pass','unregistered']};
 const run={initialization:[{file:"test/a.ts",action:"module",message:"unsupported-async initialization",status:"trapped"}],results:[
  {title:'duplicate',status:'trapped',message:'/tmp/closure/test/a.ts:1:1: lowering trap: unsupported-call /tmp/closure/src/root.ts:2:3: missing call'},
  {title:'duplicate',status:'trapped',message:'/tmp/closure/test/a.ts:4:1: lowering trap: unsupported-call /tmp/closure/src/root.ts:8:3: missing call'},
  {title:'async',status:'not-run',message:'unsupported-async: async callback'},
  {title:'pending',status:'async-pending',message:'unsupported-async: pending provider'},
  {title:'pass',status:'passed',message:''},
  {title:'unexpected',status:'passed',message:''},
 ]};
 writeFileSync(join(dir,'node.json'),JSON.stringify(oracle));
 writeFileSync(join(dir,'go.json'),JSON.stringify(run));
 execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-report.mjs'),join(dir,'node.json'),join(dir,'go.json'),join(dir,'report.json')]);
 const r=JSON.parse(readFileSync(join(dir,'report.json'),'utf8'));
 assert.deepEqual([r.passed,r.failed,r.trapped,r.asyncPending,r.notRun,r.total],[1,0,2,1,2,6]);
 assert.deepEqual(r.notRunReasons,{asyncOrDone:1,skippedOrFocused:0,unregistered:1});
 assert.deepEqual(r.unregisteredTitles,['unregistered']);
 // Missing duplicates are retained as a multiset, including pending titles.
 const duplicates = {...oracle,total:9,titles:[...oracle.titles,'unregistered','missing pending','missing pending']};
 writeFileSync(join(dir,'duplicates.json'),JSON.stringify(duplicates));
 execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-report.mjs'),join(dir,'duplicates.json'),join(dir,'go.json'),join(dir,'duplicates-report.json')]);
 const dr=JSON.parse(readFileSync(join(dir,'duplicates-report.json'),'utf8'));
 assert.deepEqual(dr.unregisteredTitles,['unregistered','unregistered','missing pending','missing pending']);
 assert.equal(dr.notRunReasons.unregistered,4);
 assert.equal(r.unmatched.length,1);
 assert.deepEqual(r.initialization,run.initialization); assert.equal(r.asyncInitializationFailures,1);
 assert.equal(r.topTraps.length,1);assert.equal(r.topTraps[0].count,2);
 for (const name of ['rules','lexer','statements','structures']) {
  writeFileSync(join(dir,`node-${name}.json`),JSON.stringify({...oracle,passed:4,failed:0,pending:1}));
  writeFileSync(join(dir,`go-${name}.json`),JSON.stringify(run));
  writeFileSync(join(dir,`report-${name}.json`),JSON.stringify(r));
  mkdirSync(join(dir,name));
  writeFileSync(join(dir,name,'closure.json'),JSON.stringify({upstreamPin:'pin',roots:['test/probe.ts']}));
 }
 execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-summary.mjs'),dir,join(dir,'summary.json')]);
 const s=JSON.parse(readFileSync(join(dir,'summary.json'),'utf8'));
 assert.equal(s.uniqueTotal.total,24); assert.equal(s.uniqueTotal.asyncOrDone,4);
 assert.equal(s.uniqueTotal.asyncPending,4); assert.equal(s.uniqueTotal.unregistered,4); assert.equal(s.registrationParity,false);
 assert.deepEqual(s.shards.map(row=>row.unregisteredTitles),Array(4).fill(['unregistered']));
 assert.equal(s.asyncInitializationFailures,4); assert.equal(s.top20Traps[0].count,8); assert.equal(s.shards.length,4);
 const auditedOracle={total:6,titles:['pass','trap','active skip','pending skip','active missing','pending missing'],pendingTitles:['pending skip','pending missing']};
 const auditedRun={results:[{title:'pass',status:'passed',message:''},{title:'trap',status:'trapped',message:'typed refusal'},{title:'active skip',status:'not-run',message:'skipped/focused'},{title:'pending skip',status:'not-run',message:'skipped/focused'}]};
 writeFileSync(join(dir,'audit-node.json'),JSON.stringify(auditedOracle));
 writeFileSync(join(dir,'audit-go.json'),JSON.stringify(auditedRun));
 const audit=()=>execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-report.mjs'),join(dir,'audit-node.json'),join(dir,'audit-go.json'),join(dir,'audit-report.json')],{stdio:'pipe'});
 audit();const ar=JSON.parse(readFileSync(join(dir,'audit-report.json')));
 assert.deepEqual(ar.nodeActive,{total:4,passed:1,failed:0,trapped:1,asyncPending:0,skipped:1,unregistered:1,notPassing:3});
 assert.deepEqual(ar.nodePending,{total:2,skipped:1,unregistered:1});
 assert.deepEqual(ar.skippedOrFocusedTests.map(t=>[t.title,t.node]),[['active skip','active'],['pending skip','pending']]);
 writeFileSync(join(dir,'audit-go.json'),JSON.stringify({results:[{title:'pending skip',status:'passed',message:''}]}));
 assert.throws(audit,/harness executed Node-pending test/);
 writeFileSync(join(dir,'audit-node.json'),JSON.stringify({total:2,titles:['collision','collision'],pendingTitles:['collision']}));
 assert.throws(audit,/ambiguous active\/pending title collision/);
 console.log('report multiset, async, unregistered, unmatched, Node-active/pending audit, four-shard summary, trap ranking: ok');
} finally {rmSync(dir,{recursive:true,force:true});}
