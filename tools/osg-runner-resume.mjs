// Checkpoints are a private stderr protocol; stdout remains one legacy report.
import {spawnSync} from 'node:child_process';

export const checkpointPrefix = 'ABAPITI_OSG_CHECKPOINT ';

// OSG ignores ABAP WRITE. Adapt only the runner's deliberately empty output
// hook, never a test body or a NOT_COMPILED site. Fail closed on ABI drift.
export function wireCheckpoints(source) {
 const hook = /func ZCL_ABAPITI_TEST_RUNNER_CHECKPOINT\(s \*abap\.Session, value string\) \{\n(?:[ \t]*(?:_ = s|return)?\n)*\}/g;
 const matches = [...source.matchAll(hook)];
 if (matches.length !== 1) throw new Error('OSG checkpoint hook ABI changed');
 return source.replace(hook, 'func ZCL_ABAPITI_TEST_RUNNER_CHECKPOINT(s *abap.Session, value string) {\n abapitiCheckpoint(value)\n}');
}

function validRecord(record) {
 return record && typeof record.title === 'string' && typeof record.message === 'string'
  && ['passed','failed','trapped','not-run','async-pending'].includes(record.status)
  && record.passed === (record.status === 'passed');
}

export function reportStatus(report) {
 return report.failed + report.trapped + report.asyncPending > 0
  || report.initialization.some(r => ['failed','trapped'].includes(r.status)) ? 1 : 0;
}

function dumpMessage(run) {
 const lines = (run.stderr ?? '').split(/\r?\n/).filter(line => !line.startsWith(checkpointPrefix));
 return lines.find(line => /OSG harness refused:|NOT_COMPILED|^panic:|^fatal error:/.test(line))
  ?? lines.find(line => line.trim())
  ?? run.error?.message ?? `OSG host exited ${run.signal ?? run.status ?? 'without status'}`;
}

// Each abnormal invocation must consume one previously unconsumed test or
// initialization action. Thus retries are bounded by the emitted plan + registry,
// not an arbitrary wall-clock limit. No-progress/infrastructure errors fail closed.
export function resumeRunner(binary, {cwd, env = process.env, spawn = spawnSync, diagnostic = () => {}} = {}) {
 const results = [], initialization = new Map(), skipped = new Set();
 let start = 1, titles, initCount;
 for (;;) {
  const run = spawn(binary, [], {cwd, env:{...env,
   ABAPITI_OSG_START_INDEX:String(start), ABAPITI_OSG_SKIP_INIT:',' + [...skipped].join(',') + ','},
   encoding:'utf8', maxBuffer:128*1024*1024});
  let active, registrySeen = false, planSeen = false;
  const invocationResults = [], invocationInit = [];
  for (const line of (run.stderr ?? '').split(/\r?\n/)) {
   if (!line.startsWith(checkpointPrefix)) { if (line) diagnostic(line + '\n'); continue; }
   const event = JSON.parse(line.slice(checkpointPrefix.length));
   switch (event.kind) {
    case 'plan':
     if (planSeen || !Number.isSafeInteger(event.initializations) || event.initializations < 0
       || (initCount !== undefined && initCount !== event.initializations)) throw new Error('unstable OSG initialization plan');
     initCount = event.initializations; planSeen = true;
     break;
    case 'init':
     if (!planSeen || registrySeen || active || !Number.isSafeInteger(event.index) || event.index < 1
       || event.index > initCount || skipped.has(event.index) || typeof event.file !== 'string' || typeof event.action !== 'string') throw new Error('invalid OSG initialization checkpoint');
     active = event;
     break;
    case 'initialization': {
     const r = event.record;
     if (active?.kind !== 'init' || !r || r.file !== active.file || r.action !== active.action
       || !['failed','trapped'].includes(r.status) || typeof r.message !== 'string') throw new Error('invalid OSG initialization result');
     const prior = initialization.get(active.index);
     if (prior && JSON.stringify(prior) !== JSON.stringify(r)) throw new Error('unstable OSG initialization result');
     initialization.set(active.index, r); invocationInit.push(r);
     break;
    }
    case 'init-done':
     if (active?.kind !== 'init') throw new Error('OSG initialization completed without checkpoint');
     active = undefined;
     break;
    case 'registry':
     if (!planSeen || active || registrySeen || !Array.isArray(event.titles) || event.titles.some(t => typeof t !== 'string')
       || (titles && JSON.stringify(titles) !== JSON.stringify(event.titles))) throw new Error('unstable OSG registrations');
     titles = event.titles; registrySeen = true;
     break;
    case 'test':
     if (!registrySeen || active || event.index !== results.length + 1 || event.index > titles.length
       || typeof event.title !== 'string' || event.title !== titles[event.index - 1]) throw new Error('unstable OSG test checkpoint');
     active = event;
     break;
    case 'result':
     if (active?.kind !== 'test' || event.index !== active.index || !validRecord(event.record)
       || event.record.title !== active.title) throw new Error('invalid OSG test result');
     results.push(event.record); invocationResults.push(event.record); active = undefined;
     break;
    default: throw new Error('unknown OSG checkpoint kind: ' + event.kind);
   }
  }
  let final;
  try { final = JSON.parse(run.stdout ?? ''); } catch { /* a host dump has no final report */ }
  if (final) {
   if (run.error || ![0,1].includes(run.status) || active || !registrySeen || results.length !== titles.length
     || JSON.stringify(final.results) !== JSON.stringify(invocationResults)
     || JSON.stringify(final.initialization) !== JSON.stringify(invocationInit)) throw new Error('incomplete OSG final report');
   const report = {asyncPending:0, passed:0, failed:0, trapped:0, notRun:0, results,
    initialization:[...initialization].sort(([a],[b]) => a-b).map(([,r]) => r)};
   const keys = {'passed':'passed','failed':'failed','trapped':'trapped','not-run':'notRun','async-pending':'asyncPending'};
   for (const r of results) report[keys[r.status]]++;
   return {report, status:reportStatus(report)};
  }
  // Buffer overflow, failure to spawn, and a normal exit without JSON are not
  // evidence that the active test dumped. Never manufacture a verdict for them.
  if (run.error || run.status === 0 || !active) throw new Error('OSG host failed without a resumable checkpoint: ' + dumpMessage(run));
  const message = dumpMessage(run);
  if (active.kind === 'test') {
   results.push({title:active.title, passed:false, message, status:'trapped'});
   start = active.index + 1;
  } else {
   if (skipped.has(active.index)) throw new Error('OSG initialization retry made no progress');
   initialization.set(active.index, {file:active.file, action:active.action, message, status:'trapped'});
   skipped.add(active.index);
  }
 }
}
