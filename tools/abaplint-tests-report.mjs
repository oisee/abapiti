#!/usr/bin/env node
// Reconcile against an independently executed original mocha run, including
// duplicate titles. Unregistered tests are not-run, never assumed failures.
import {readFileSync, writeFileSync} from 'node:fs';
const [oraclePath, goPath, output] = process.argv.slice(2);
if (!output) throw new Error('usage: abaplint-tests-report.mjs NODE.json GO.json OUTPUT.json');
const oracle = JSON.parse(readFileSync(oraclePath, 'utf8'));
const run = JSON.parse(readFileSync(goPath, 'utf8'));
const available = new Map();
for (const title of oracle.titles) available.set(title, (available.get(title) ?? 0) + 1);
const report = {total:oracle.total ?? oracle.titles.length, passed:0, failed:0, trapped:0, notRun:0, registered:run.results.length, notRunReasons:{asyncOrDone:0, skippedOrFocused:0, unregistered:0}, unmatched:[], topTraps:[]};
report.initialization = run.initialization ?? [];
report.asyncInitializationFailures = report.initialization.filter(r => r.message.includes('unsupported-async')).length;
const traps = new Map();
for (const r of run.results) {
 const n = available.get(r.title) ?? 0;
 if (!n) {report.unmatched.push(r); continue;}
 available.set(r.title,n-1);
 const key = {passed:'passed', failed:'failed', trapped:'trapped', 'not-run':'notRun'}[r.status];
 if (!key) throw new Error(`unknown result status ${r.status}`);
 report[key]++;
 if (key === 'notRun') report.notRunReasons[r.message.includes('unsupported-async') ? 'asyncOrDone' : 'skippedOrFocused']++;
 if (key === 'trapped') {
  // Group diagnostic text, not a closure's varying creation location. Keep an
  // actual representative message with TS location for follow-up.
  const message = r.message.replace(/^.*?lowering trap: /s,'').replace(/(?:\/[^\s:;]+\/)?((?:src|test|runtime)\/[^\s:;]+):\d+:\d+/g,'$1').replace(/node \d+ \([^)]*\): /g,'');
  const item=traps.get(message) ?? {message,count:0,example:r.message}; item.count++;traps.set(message,item);
 }
}
report.notRunReasons.unregistered = [...available.values()].reduce((a,b)=>a+b,0);
report.notRun += report.notRunReasons.unregistered;
report.topTraps = [...traps.values()].sort((a,b)=>b.count-a.count || a.message.localeCompare(b.message)).slice(0,20);
if (report.passed+report.failed+report.trapped+report.notRun !== report.total) throw new Error('oracle reconciliation did not sum to total');
writeFileSync(output,JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify(report));
