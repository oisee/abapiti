#!/usr/bin/env node
// Execute a shared binary with fresh, shard-local checkpoint/resume state.
import {mkdirSync, writeFileSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {performance} from 'node:perf_hooks';
import {resumeRunner} from './osg-runner-resume.mjs';
import {externalPath} from './osg-harness-cache.mjs';

const [binary, names, output, shard] = process.argv.slice(2);
if (!binary || !names || !output || !shard) throw new Error('usage: osg-runner-execute.mjs BINARY NAMES OUT SHARD');
const out = process.env.ABAPITI_OSG_HOME ? externalPath(output, process.env.ABAPITI_OSG_HOME) : resolve(output);
mkdirSync(out, {recursive:true});
const started = performance.now();
let status = 2;
try {
 const run = resumeRunner(resolve(binary), {cwd:out, args:[resolve(names)],
  env:{...process.env, ABAPITI_OSG_SHARD:shard}, diagnostic:text => process.stderr.write(text)});
 process.stdout.write(JSON.stringify(run.report) + '\n');
 status = run.status;
} catch (error) { console.error(String(error)); }
writeFileSync(join(out, 'timing.json'), JSON.stringify({status, timingMs:{run:Math.round(performance.now()-started)}, sharedBuild:true}, null, 2) + '\n');
process.exitCode = status;
