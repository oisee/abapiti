#!/usr/bin/env node
// Offline only. Input is runner JSON, checkpoint events, or fetched table rows.
import {readFileSync, writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';

export function classifyFailure(record) {
 if (record.status === 'passed' || record.status === 'not-run') return record;
 const message=record.message ?? '';
 // Kernel exception identities and explicit operation provenance only: neither
 // a regex-related test title nor an ordinary assertion proves PCRE drift.
 if (/\b(?:FIND|REPLACE)\s+PCRE\b|\bPCRE2?\b|\bCX_SY_(?:REGEX|INVALID_REGEX|REGEX_TOO_COMPLEX)\b/i.test(message)
   || /^(?:FIND|REPLACE) PCRE$/i.test(record.operation ?? '')) return {...record,category:'pcre-drift'};
 return record;
}

function eventsFromRows(rows, shard) {
 const runs=new Set(rows.map(r=>r.RUN_ID ?? r.run_id));
 if(runs.size!==1) throw new Error('fetch exactly one RUN_ID');
 const groups=new Map();
 for(const row of rows) {
  const get=k=>row[k] ?? row[k.toLowerCase()];
  if(shard && get('SHARD')?.trim()!==shard) throw new Error('mixed shard log');
  const seq=Number(get('SEQ')),part=Number(get('PART')),used=Number(get('USED'));
  if(!Number.isSafeInteger(seq)||seq<1||!Number.isSafeInteger(part)||part<1||!Number.isInteger(used)||used<1||used>255) throw new Error('invalid log chunk');
  const payload=get('PAYLOAD');
  if(typeof payload!=='string'||payload.length<used) throw new Error('short log chunk (fetch must preserve spaces)');
  const pieces=groups.get(seq) ?? new Map();
  if(pieces.has(part)) throw new Error('duplicate log chunk');
  pieces.set(part,payload.slice(0,used));groups.set(seq,pieces);
 }
 return [...groups].sort(([a],[b])=>a-b).map(([,pieces])=>{
  const parts=[...pieces].sort(([a],[b])=>a-b);
  if(parts.some(([part],i)=>part!==i+1)) throw new Error('missing log chunk');
  return JSON.parse(parts.map(([,text])=>text).join(''));
 });
}

export function collectResults(input,{shard,dumps={}}={}) {
 let report;
 if(!Array.isArray(input)) {
  if(!Array.isArray(input.results)) throw new Error('missing runner results');
  report={...input};
 } else {
  const events=input[0] && ('SEQ' in input[0] || 'seq' in input[0]) ? eventsFromRows(input,shard) : input;
  const records=new Map(), tests=new Map(), initialization=[];
  let complete=false;
  for(const event of events) {
   if(event.kind==='test') {
    if(tests.has(event.index)&&tests.get(event.index)!==event.title) throw new Error('unstable test registry');
    tests.set(event.index,event.title);
   } else if(event.kind==='result') {
    if(records.has(event.index)) throw new Error('duplicate test result; use a new RUN_ID for reruns');
    records.set(event.index,event.record);
   } else if(event.kind==='initialization') initialization.push(event.record);
   else if(event.kind==='complete') complete=true;
  }
  if(!complete) throw new Error('incomplete job: resume or provide final runner JSON; do not credit missing tests');
  for(const [index,title] of tests) if(!records.has(index)) records.set(index,{title,passed:false,status:'trapped',message:dumps[index] ?? 'A4H kernel job ended at this checkpoint without a result (fetch ST22 short text)'});
  for(const [index,record] of records) if(tests.get(index)!==record.title) throw new Error('result without matching checkpoint');
  report={results:[...records].sort(([a],[b])=>a-b).map(([,r])=>r),initialization};
 }
 const counts={passed:0,failed:0,trapped:0,asyncPending:0,notRun:0};
 report.results=report.results.map(r=>{
  const key={passed:'passed',failed:'failed',trapped:'trapped','async-pending':'asyncPending','not-run':'notRun'}[r.status];
  if(!key||typeof r.title!=='string'||typeof r.message!=='string'||r.passed!==(r.status==='passed')) throw new Error('invalid test result');
  counts[key]++;return classifyFailure(r);
 });
 report.initialization=(report.initialization ?? []).map(classifyFailure);
 return {...report,...counts,categories:{'pcre-drift':report.results.filter(r=>r.category==='pcre-drift').length},
  initializationCategories:{'pcre-drift':report.initialization.filter(r=>r.category==='pcre-drift').length}};
}

if(process.argv[1] && import.meta.url===pathToFileURL(resolve(process.argv[1])).href) {
 const [input,output,shard,dumpFile]=process.argv.slice(2);
 if(!output) throw new Error('usage: a4h-tests-collect.mjs FETCHED.json go-<shard-id>.json [shard] [ST22-index-message-map.json]');
 const report=collectResults(JSON.parse(readFileSync(input,'utf8')),{shard,dumps:dumpFile?JSON.parse(readFileSync(dumpFile,'utf8')):{}});
 writeFileSync(output,JSON.stringify(report,null,2)+'\n');
 console.log(JSON.stringify({...report,results:undefined,initialization:undefined}));
}
