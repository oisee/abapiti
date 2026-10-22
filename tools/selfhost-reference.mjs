#!/usr/bin/env node
// Regenerate a frozen multifile self-hosting kit's reference from original TS.
import {readFileSync, writeFileSync, mkdirSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {buildUpstream, upstreamPin} from './statements-upstream.mjs';

const [kitArg, outArg, upstreamArg] = process.argv.slice(2);
if (!upstreamArg) throw new Error('usage: selfhost-reference.mjs KIT OUTPUT UPSTREAM');
const kit = resolve(kitArg), out = resolve(outArg), upstream = resolve(upstreamArg);
const require = createRequire(import.meta.url);
const ts = require(join(upstream, 'node_modules/typescript'));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const list = name => readFileSync(join(kit, name), 'utf8').split(/\r?\n/).filter(Boolean);
const main = list('files.txt'), dependencies = list('deps.txt');
mkdirSync(out, {recursive:true});
const built = buildUpstream(upstream);
try {
 const raw = readFileSync(new URL('../tsfront/testdata/registrycorpus/harness/registry_run.ts', import.meta.url), 'utf8');
 const js = ts.transpileModule(raw, {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText
  .replace(/require\("\.\.\/src\/([^" ]+)"\)/g, (_, name) => `require(${JSON.stringify(join(built.core, name + '.js'))})`);
 const harness = join(out, 'node-harness.cjs');
 writeFileSync(harness, js);
 const {RegistryRun} = require(harness);
 const run = new RegistryRun();
 const inputs = [];
 for (const [files, dependency] of [[main,false],[dependencies,true]]) for (const name of files) {
  const bytes = readFileSync(resolve(kit, name));
  const filename = name.replaceAll('\\','/');
  inputs.push({filename, dependency, sha256:sha(bytes)});
  if (dependency) run.addDependency(filename, bytes.toString('utf8'));
  else run.addFile(filename, bytes.toString('utf8'));
 }
 const config = readFileSync(join(kit, 'abaplint.json'));
 const start = performance.now();
 const output = run.run(config.toString('utf8')) + '\n';
 const seconds = (performance.now() - start) / 1000;
 writeFileSync(join(out,'expected-clean.txt'), output);
 const reference = {upstreamPin, coreVersion:JSON.parse(readFileSync(join(upstream,'packages/core/package.json'))).version,
  mainFiles:main.length, dependencyFiles:dependencies.length, issues:Number(output.split('\n')[0]),
  outputSHA256:sha(output), dumpSHA256:sha(output.slice(0,-1)), filesListSHA256:sha(readFileSync(join(kit,'files.txt'))),
  depsListSHA256:sha(readFileSync(join(kit,'deps.txt'))), configSHA256:sha(config),
  inputsSHA256:sha(JSON.stringify(inputs)), harnessSHA256:sha(raw)};
 writeFileSync(join(out,'reference.json'), JSON.stringify(reference,null,2)+'\n');
 console.log(JSON.stringify({...reference, seconds, peakRSSKiB:process.resourceUsage().maxRSS}));
} finally { built.dispose(); }
