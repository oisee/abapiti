#!/usr/bin/env node
// Original upstream mocha/chai, never our runtime. A runner may share a fresh
// upstream compilation; every oracle invocation is a separate mocha process.
import {execFileSync} from 'node:child_process';
import {writeFileSync, existsSync} from 'node:fs';
import {join, dirname, resolve} from 'node:path';
import {homedir} from 'node:os';
import {pathToFileURL} from 'node:url';
import {buildUpstream} from './statements-upstream.mjs';

export function executeOracle(build, upstream, files) {
 for (const file of files) if (!/^test\/.+\.ts$/.test(file) || file.includes('..')) throw new Error(`invalid test path: ${file}`);
 const mocha = [join(upstream,'node_modules/mocha/bin/mocha.js'), join(upstream,'packages/core/node_modules/mocha/bin/mocha.js')].find(existsSync);
 if (!mocha) throw new Error('upstream mocha is not installed');
 let raw;
 try {
  raw = execFileSync(process.execPath, [mocha,'--reporter','json', ...files.map(f => join(dirname(build.core), f.replace(/\.ts$/, '.js')))], {encoding:'utf8', maxBuffer:128*1024*1024});
 } catch (e) { if (!e.stdout) throw e; raw = e.stdout; }
 const result = JSON.parse(raw);
 const titles = tests => tests.map(t => t.fullTitle);
 return {total:result.stats.tests, passed:result.stats.passes, failed:result.stats.failures, pending:result.stats.pending,
  failures:result.failures.map(t => ({title:t.fullTitle,message:t.err.message})), titles:titles(result.tests),
  passedTitles:titles(result.passes), failedTitles:titles(result.failures), pendingTitles:titles(result.pending)};
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
 const [output, upstream = join(homedir(), 'dev/abaplint'), ...files] = process.argv.slice(2);
 if (!output || !files.length) throw new Error('usage: abaplint-tests-oracle.mjs OUTPUT.json UPSTREAM test/rules/7bit_ascii.ts [...files]');
 const build = buildUpstream(upstream);
 try {
  const summary = executeOracle(build, upstream, files);
  writeFileSync(resolve(output), JSON.stringify(summary,null,2)+'\n');
  console.log(JSON.stringify(summary));
 } finally { build.dispose(); }
}
