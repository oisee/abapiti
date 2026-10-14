#!/usr/bin/env node
// Reconcile against an independently executed original mocha run, including
// duplicate titles. Unregistered tests are not-run, never assumed failures.
import {readFileSync, writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';

export function reconcileResults(oracle, run) {
const available = new Map();
for (const title of oracle.titles) available.set(title, (available.get(title) ?? 0) + 1);
const report = {total:oracle.total ?? oracle.titles.length, passed:0, failed:0, trapped:0, asyncPending:0, notRun:0, registered:run.results.length, notRunReasons:{asyncOrDone:0, skippedOrFocused:0, unregistered:0}, unmatched:[], topTraps:[]};
report.initialization = run.initialization ?? [];
report.asyncInitializationFailures = report.initialization.filter(r => r.message.includes('unsupported-async')).length;
const traps = new Map();
const pending = new Map();
for (const title of oracle.pendingTitles ?? []) pending.set(title, (pending.get(title) ?? 0) + 1);
const audited = Array.isArray(oracle.pendingTitles);
if (audited) {
 for (const [title,n] of pending) if (n !== available.get(title)) throw new Error(`ambiguous active/pending title collision: ${title}`);
 report.nodeActive = {total:oracle.titles.length - oracle.pendingTitles.length, passed:0, failed:0, trapped:0, asyncPending:0, skipped:0, unregistered:0};
 report.nodePending = {total:oracle.pendingTitles.length, skipped:0, unregistered:0};
 report.skippedOrFocusedTests = [];
}
for (const r of run.results) {
 const n = available.get(r.title) ?? 0;
 if (!n) {report.unmatched.push(r); continue;}
 available.set(r.title,n-1);
 const key = {passed:'passed', failed:'failed', trapped:'trapped', 'not-run':'notRun', 'async-pending':'asyncPending'}[r.status];
 if (!key) throw new Error(`unknown result status ${r.status}`);
 report[key]++;
 if (audited) {
  const nodePending = pending.has(r.title);
  if (nodePending && key !== 'notRun') throw new Error(`harness executed Node-pending test: ${r.title} (${r.status})`);
  if (key === 'notRun') {
   if (nodePending) report.nodePending.skipped++;
   else report.nodeActive.skipped++;
   report.skippedOrFocusedTests.push({title:r.title, node:nodePending ? 'pending' : 'active', message:r.message});
  } else report.nodeActive[key]++;
 }
 if (key === 'notRun') report.notRunReasons[(r.message.includes('unsupported-async') || r.message.includes('unsupported-test-done')) ? 'asyncOrDone' : 'skippedOrFocused']++;
 if (key === 'trapped') {
  // Group diagnostic text, not a closure's varying creation location. Keep an
  // actual representative message with TS location for follow-up.
  const message = r.message.replace(/^.*?lowering trap: /s,'').replace(/(?:\/[^\s:;]+\/)?((?:src|test|runtime)\/[^\s:;]+):\d+:\d+/g,'$1').replace(/node \d+ \([^)]*\): /g,'');
  const item=traps.get(message) ?? {message,count:0,example:r.message}; item.count++;traps.set(message,item);
 }
}
// Keep the exact missing multiset for registration-root audits, including
// pending Node cases that never registered. Do not infer execution statuses.
report.unregisteredTitles = [...available].flatMap(([title, count]) => Array(count).fill(title));
report.notRunReasons.unregistered = report.unregisteredTitles.length;
report.notRun += report.notRunReasons.unregistered;
if (audited) {
 for (const title of report.unregisteredTitles) report[pending.has(title) ? 'nodePending' : 'nodeActive'].unregistered++;
 const a=report.nodeActive;
 a.notPassing=a.total-a.passed;
 if (a.total !== a.passed+a.failed+a.trapped+a.asyncPending+a.skipped+a.unregistered) throw new Error('Node-active reconciliation mismatch');
 if (report.nodePending.total !== report.nodePending.skipped+report.nodePending.unregistered) throw new Error('Node-pending reconciliation mismatch');
}
report.topTraps = [...traps.values()].sort((a,b)=>b.count-a.count || a.message.localeCompare(b.message)).slice(0,20);
if (report.passed+report.failed+report.trapped+report.asyncPending+report.notRun !== report.total) throw new Error('oracle reconciliation did not sum to total');
return report;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
 const [oraclePath, goPath, output] = process.argv.slice(2);
 if (!output) throw new Error('usage: abaplint-tests-report.mjs NODE.json GO.json OUTPUT.json');
 const report = reconcileResults(JSON.parse(readFileSync(oraclePath, 'utf8')), JSON.parse(readFileSync(goPath, 'utf8')));
 writeFileSync(output,JSON.stringify(report,null,2)+'\n');
 console.log(JSON.stringify(report));
}
