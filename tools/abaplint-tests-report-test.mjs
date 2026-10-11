#!/usr/bin/env node
import assert from 'node:assert/strict';
import {mkdtempSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
const dir=mkdtempSync(join(tmpdir(),'abaplint-report-'));
try {
 const oracle={total:5,titles:['duplicate','duplicate','async','pass','unregistered']};
 const run={results:[
  {title:'duplicate',status:'trapped',message:'/tmp/closure/test/a.ts:1:1: lowering trap: unsupported-call /tmp/closure/src/root.ts:2:3: missing call'},
  {title:'duplicate',status:'trapped',message:'/tmp/closure/test/a.ts:4:1: lowering trap: unsupported-call /tmp/closure/src/root.ts:8:3: missing call'},
  {title:'async',status:'not-run',message:'unsupported-async: async callback'},
  {title:'pass',status:'passed',message:''},
  {title:'unexpected',status:'passed',message:''},
 ]};
 writeFileSync(join(dir,'node.json'),JSON.stringify(oracle));
 writeFileSync(join(dir,'go.json'),JSON.stringify(run));
 execFileSync(process.execPath,[join(import.meta.dirname,'abaplint-tests-report.mjs'),join(dir,'node.json'),join(dir,'go.json'),join(dir,'report.json')]);
 const r=JSON.parse(readFileSync(join(dir,'report.json'),'utf8'));
 assert.deepEqual([r.passed,r.failed,r.trapped,r.notRun,r.total],[1,0,2,2,5]);
 assert.deepEqual(r.notRunReasons,{asyncOrDone:1,skippedOrFocused:0,unregistered:1});
 assert.equal(r.unmatched.length,1);
 assert.equal(r.topTraps.length,1);assert.equal(r.topTraps[0].count,2);
 console.log('report multiset, async, unregistered, unmatched, trap ranking: ok');
} finally {rmSync(dir,{recursive:true,force:true});}
