#!/usr/bin/env node
// Compact, reproducible four-shard evidence; never estimates missing tests.
import {readFileSync, writeFileSync, existsSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {execFileSync} from 'node:child_process';
const [directory, output, prefix = ''] = process.argv.slice(2);
if (!output) throw new Error('usage: abaplint-tests-summary.mjs RUN OUTPUT.json [artifact-prefix]');
const read = name => JSON.parse(readFileSync(join(directory, name), 'utf8'));
const shards = [];
const traps = new Map();
const totals = {passed:0, failed:0, trapped:0, notRun:0, registered:0, total:0, asyncOrDone:0, unregistered:0};
let pin;
for (const name of ['rules','lexer','statements','structures']) {
 const report = read(`${prefix}report-${name}.json`);
 const run = read(`${prefix}go-${name}.json`);
 const node = read(`node-${name}.json`);
 const closure = read(`${prefix}${name}/closure.json`);
 if (pin && pin !== closure.upstreamPin) throw new Error('mixed upstream pins');
 pin = closure.upstreamPin;
 const clean = text => text.replaceAll(resolve(directory, `${prefix}${name}`) + '/', '');
 const row = {name:name === 'rules' ? name : `abap/${name}`, files:closure.roots.length,
  passed:report.passed, failed:report.failed, trapped:report.trapped, notRun:report.notRun,
  registered:report.registered, total:report.total, node:{passed:node.passed, failed:node.failed, pending:node.pending},
  notRunReasons:report.notRunReasons, unmatched:report.unmatched.length,
  initialization:(run.initialization ?? []).map(r => ({...r, action:clean(r.action), message:clean(r.message)})),
  failures:run.results.filter(r => r.status === 'failed').map(r => ({title:r.title, message:clean(r.message).replace(/0x[0-9a-f]+/g,'<address>')}))};
 if (row.total !== node.total) throw new Error('oracle total mismatch');
 shards.push(row);
 for (const key of ['passed','failed','trapped','notRun','registered','total']) totals[key] += row[key];
 totals.asyncOrDone += report.notRunReasons.asyncOrDone;
 totals.unregistered += report.notRunReasons.unregistered;
 for (const r of run.results.filter(r => r.status === 'trapped')) {
  const message = clean(r.message).replace(/^.*?lowering trap: /s,'').replace(/(?:\/[^\s:;]+\/)?((?:src|test|runtime)\/[^\s:;]+):\d+:\d+/g,'$1').replace(/node \d+ \([^)]*\): /g,'');
  const item = traps.get(message) ?? {message,count:0,shards:{},example:clean(r.message)};
  item.count++; item.shards[name] = (item.shards[name] ?? 0) + 1; traps.set(message,item);
 }
}
const result = {upstreamPin:pin, codeCommit:execFileSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim(),
 mainCommit:execFileSync('git',['rev-parse','origin/main'],{encoding:'utf8'}).trim(),
 artifacts:directory, artifactPrefix:prefix, shards, uniqueTotal:totals,
 asyncInitializationFailures:shards.flatMap(s => s.initialization).filter(r => r.message.includes('unsupported-async')).length,
 registrationParity:shards.every(s => s.registered === s.total && s.unmatched === 0),
 top20Traps:[...traps.values()].sort((a,b) => b.count-a.count || a.message.localeCompare(b.message)).slice(0,20)};
if (existsSync(join(directory,'verification.json'))) result.verification = read('verification.json');
writeFileSync(output,JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(totals));
