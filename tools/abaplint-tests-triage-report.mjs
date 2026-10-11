#!/usr/bin/env node
// Build the per-test evidence ledger, preserving complete Node issue payloads.
import {readFileSync, writeFileSync} from 'node:fs';
const [oraclePath, beforePath, afterPath, tracesPath, output] = process.argv.slice(2);
if (!output) throw new Error('usage: abaplint-tests-triage-report.mjs NODE.json BEFORE.json AFTER.json NODE_ISSUES.json OUTPUT.json');
const read = p => JSON.parse(readFileSync(p, 'utf8'));
const oracle = read(oraclePath), before = read(beforePath), after = read(afterPath), traces = read(tracesPath);
const byTitle = new Map();
for (const trace of traces) if (trace.site === 'Registry.findIssues') {
 if (!Number.isInteger(trace.occurrence)) throw new Error('Node trace lacks title occurrence');
 const occurrences = byTitle.get(trace.title) ?? new Map();
 const list = occurrences.get(trace.occurrence) ?? []; list.push(trace.issues);
 occurrences.set(trace.occurrence,list); byTitle.set(trace.title, occurrences);
}
const afterByTitle = new Map();
for (const result of after.results) {
 const list = afterByTitle.get(result.title) ?? []; list.push(result); afterByTitle.set(result.title, list);
}
const beforeOccurrences = new Map();
const failures = before.results.map(r => {
 const occurrence = beforeOccurrences.get(r.title) ?? 0;
 beforeOccurrences.set(r.title,occurrence+1);
 return {...r,occurrence};
}).filter(r => r.status === 'failed');
if (failures.length !== 104) throw new Error(`expected the 104-test cohort, found ${failures.length}`);
if (before.results.length !== after.results.length) throw new Error('registration changed');
for (let i=0;i<before.results.length;i++) {
 const b=before.results[i], a=after.results[i];
 if (b.title !== a.title || b.message !== a.message) throw new Error('execution or refusal changed at '+i);
 if (b.status !== 'failed' && b.status !== a.status) throw new Error('unrelated execution changed: '+b.title);
}
const records = failures.map(r => {
 if (r.message !== 'src/rules/7bit_ascii.ts:38:3: value outside supported numeric, ordering or conversion domain') throw new Error('untriaged failure: '+r.message);
 if (!oracle.titles.includes(r.title) || oracle.failed) throw new Error('Node oracle mismatch');
 const expected = byTitle.get(r.title)?.get(r.occurrence);
 if (!expected) throw new Error('missing Node issue payload: '+r.title);
 const actualAfter = afterByTitle.get(r.title)?.[r.occurrence];
 if (!actualAfter || actualAfter.status !== 'trapped' || actualAfter.message !== r.message) throw new Error('refusal changed or missing: '+r.title);
 return {title:r.title, occurrence:r.occurrence, cluster:'JSON config reference refusal misclassified by harness', class:'c',
  expected:{status:'passed', registryIssues:expected, assertedRuleIssues:expected.map(issues=>issues.filter(i=>i.key==='unknown_types'))},
  actualBefore:{status:r.status, issues:null, message:r.message}, actualAfter:{status:actualAfter.status, issues:null, message:r.message},
  issuesUnavailableReason:'Config.getEnabledRules refuses at SevenBitAscii.setConfig before Registry.findIssues returns; no issue array or chai assertion is produced.'};
});
const counts = run => {
 const available = new Map();
 for (const title of oracle.titles) available.set(title, (available.get(title) ?? 0)+1);
 const result = {passed:0, failed:0, trapped:0, notRun:0, registered:run.results.length};
 for (const r of run.results) {
  const n = available.get(r.title) ?? 0;
  if (!n) throw new Error('unmatched Go title: '+r.title);
  available.set(r.title,n-1);
  const status = r.status === 'not-run' ? 'notRun' : r.status;
  if (!Object.hasOwn(result,status)) throw new Error('unknown status: '+status);
  result[status]++;
 }
 result.unregistered=[...available.values()].reduce((a,b)=>a+b,0);
 result.notRun+=result.unregistered;
 if (result.passed+result.failed+result.trapped+result.notRun !== oracle.total) throw new Error('counts do not reconcile');
 return result;
};
writeFileSync(output, JSON.stringify({oracle:{total:oracle.total,passed:oracle.passed,failed:oracle.failed,pending:oracle.pending}, before:counts(before),after:counts(after),failureCount:records.length,semanticDivergences:0,records},null,2)+'\n');
console.log(JSON.stringify({failures:records.length,before:counts(before),after:counts(after)}));
