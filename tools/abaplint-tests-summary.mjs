#!/usr/bin/env node
// Compact directory-shard evidence; never estimates missing tests.
import {readFileSync, writeFileSync, existsSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {execFileSync} from 'node:child_process';
const [directory, output, prefix = ''] = process.argv.slice(2);
if (!output) throw new Error('usage: abaplint-tests-summary.mjs RUN OUTPUT.json [artifact-prefix]');
const read = name => JSON.parse(readFileSync(join(directory, name), 'utf8'));
const shards = [];
const traps = new Map();
const totals = {passed:0, failed:0, trapped:0, asyncPending:0, notRun:0, registered:0, total:0, asyncOrDone:0, skippedOrFocused:0, unregistered:0};
let pin;
const manifest = existsSync(join(directory,'shards.json')) ? read('shards.json') : null;
const specs = manifest?.shards ?? ['rules','lexer','statements','structures'].map(id => ({id, name:id === 'rules' ? id : `abap/${id}`}));
const nodeTotals = {passed:0, failed:0, pending:0, total:0};
const titles = new Map();
for (const {id:name, name:label} of specs) {
 const report = read(`${prefix}report-${name}.json`);
 const run = read(`${prefix}go-${name}.json`);
 const node = read(`node-${name}.json`);
 const closure = read(`${prefix}${name}/closure.json`);
 if (pin && pin !== closure.upstreamPin) throw new Error('mixed upstream pins');
 pin = closure.upstreamPin;
 const clean = text => text.replaceAll(resolve(directory, `${prefix}${name}`) + '/', '');
 const row = {name:label, files:closure.roots.length,
  passed:report.passed, failed:report.failed, trapped:report.trapped, asyncPending:report.asyncPending ?? 0, notRun:report.notRun,
  registered:report.registered, total:report.total, node:{passed:node.passed, failed:node.failed, pending:node.pending},
  notRunReasons:report.notRunReasons, unmatched:report.unmatched.length,
  initialization:(run.initialization ?? []).map(r => ({...r, action:clean(r.action), message:clean(r.message)})),
  failures:run.results.filter(r => r.status === 'failed').map(r => ({title:r.title, message:clean(r.message).replace(/0x[0-9a-f]+/g,'<address>')}))};
 if (node.borrowed) row.borrowed = {...node.borrowed, registered:run.borrowedRegistered, rawRegistered:run.rawRegistered};
 if (run.infrastructureFailure) row.infrastructureFailure = clean(run.infrastructureFailure);
 if (row.total !== node.total) throw new Error('oracle total mismatch');
 for (const key of Object.keys(nodeTotals)) nodeTotals[key] += node[key];
 for (const title of node.titles) titles.set(title, (titles.get(title) ?? 0) + 1);
 shards.push(row);
 for (const key of ['passed','failed','trapped','asyncPending','notRun','registered','total']) totals[key] += row[key];
 for (const key of ['asyncOrDone','skippedOrFocused','unregistered']) totals[key] += report.notRunReasons[key] ?? 0;
 for (const r of run.results.filter(r => r.status === 'trapped')) {
  const message = clean(r.message).replace(/^.*?lowering trap: /s,'').replace(/(?:\/[^\s:;]+\/)?((?:src|test|runtime)\/[^\s:;]+):\d+:\d+/g,'$1').replace(/node \d+ \([^)]*\): /g,'');
  const item = traps.get(message) ?? {message,count:0,shards:{},example:clean(r.message)};
  item.count++; item.shards[name] = (item.shards[name] ?? 0) + 1; traps.set(message,item);
 }
}
let wholeSuite;
if (manifest) {
 const roots = specs.flatMap(s => s.roots).sort();
 if (JSON.stringify(roots) !== JSON.stringify(manifest.files) || new Set(roots).size !== roots.length) throw new Error('test roots are not a disjoint exhaustive partition');
 const all = read('node-all.json');
 for (const title of all.titles) titles.set(title, (titles.get(title) ?? 0) - 1);
 if ([...titles.values()].some(n => n !== 0)) throw new Error('shard titles differ from whole-suite Node multiset');
 for (const key of Object.keys(nodeTotals)) if (nodeTotals[key] !== all[key]) throw new Error(`whole-suite Node ${key} mismatch`);
 if (all.passed + all.failed !== 11060) throw new Error(`pinned active suite denominator changed: ${all.passed + all.failed}`);
 if (all.total !== all.passed + all.failed + all.pending) throw new Error('Node active/pending total mismatch');
 wholeSuite = {files:manifest.files.length, ...nodeTotals, active:all.passed + all.failed, titlesReconciled:true, excluded:manifest.excluded,
  wiringFailures:shards.filter(s => s.infrastructureFailure).map(s => ({name:s.name, files:specs.find(p => p.name === s.name).roots, reason:s.infrastructureFailure}))};
}
const result = {upstreamPin:pin, codeCommit:execFileSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim(),
 mainCommit:execFileSync('git',['rev-parse','origin/main'],{encoding:'utf8'}).trim(),
 artifacts:directory, artifactPrefix:prefix, shards, uniqueTotal:totals, nodeTotal:nodeTotals,
 ...(wholeSuite ? {wholeSuite} : {}),
 countingRule:'Typed lowering/runtime refusals are trapped; genuine assertion mismatches and other non-refusal throws are failed. Async-pending and not-run remain separate; no failed record is relabeled by the report.',
 asyncInitializationFailures:shards.flatMap(s => s.initialization).filter(r => r.message.includes('unsupported-async')).length,
 registrationParity:shards.every(s => s.registered === s.total && s.unmatched === 0),
 top20Traps:[...traps.values()].sort((a,b) => b.count-a.count || a.message.localeCompare(b.message)).slice(0,20)};
if (existsSync(join(directory,'verification.json'))) result.verification = read('verification.json');
writeFileSync(output,JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(totals));
