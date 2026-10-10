#!/usr/bin/env node
// Run under flock /tmp/abapiti-heavy.lock. Builds only the original upstream
// oracle. Writes a reproducible, explicitly representative abapGit source kit.
import {execFileSync, spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {createRequire} from 'node:module';
import {basename, join, resolve} from 'node:path';
import {copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync} from 'node:fs';
import {buildUpstream} from './statements-upstream.mjs';

const args = Object.fromEntries(process.argv.slice(2).map(arg => {
 const at = arg.indexOf('=');
 if (!arg.startsWith('--') || at < 3) throw new Error('use --name=value');
 return [arg.slice(2, at), arg.slice(at + 1)];
}));
for (const key of ['pilot', 'vanilla', 'upstream', 'abapgit', 'zabapgit', 'out']) {
 if (!args[key]) throw new Error('missing --' + key);
 args[key] = resolve(args[key]);
}
const started = Date.now();
const pin = '3b6485b5d0b09ef3861006b966e68542446721ce';
const git = (...a) => execFileSync('git', ['-C', args.abapgit, ...a], {encoding: 'utf8'}).trim();
if (git('rev-parse', 'HEAD') !== pin || git('status', '--porcelain', '--untracked-files=all')) {
 throw new Error('abapGit source must be clean at ' + pin);
}
mkdirSync(args.out, {recursive: true});
const sourceKit = join(args.out, 'abapgit-src');
mkdirSync(sourceKit, {recursive: true});
function walk(dir) {
 return readdirSync(dir).sort().flatMap(name => {
  const p = join(dir, name);
  return statSync(p).isDirectory() ? walk(p) : [p];
 });
}
// An independent, deterministic selection; no selection by observed outcome.
const interfaces = walk(join(args.abapgit, 'src')).filter(p => p.endsWith('.intf.abap'))
 .sort((a, b) => statSync(a).size - statSync(b).size || (a < b ? -1 : a > b ? 1 : 0)).slice(0, 5);
const sourceInputs = interfaces.flatMap(p => [p, p.replace(/\.abap$/, '.xml')].filter(existsSync)).sort();
for (const p of sourceInputs) copyFileSync(p, join(sourceKit, basename(p)));
const config = readFileSync(join(args.zabapgit, 'abaplint.json'), 'utf8');
const deps = readFileSync(join(args.zabapgit, 'deps.txt'), 'utf8').split(/\r?\n/).filter(Boolean)
 .map(p => resolve(args.zabapgit, p));
const depList = join(args.out, 'deps.txt');
const configPath = join(args.out, 'abaplint.json');
writeFileSync(depList, deps.join('\n') + '\n');
writeFileSync(configPath, config);
const sha = raw => createHash('sha256').update(raw).digest('hex');
const manifest = {
 schema: 1, abapgit_pin: pin,
 selection: 'five smallest *.intf.abap sources by byte size/path, with their original XML; representative, not the full source tree',
 inputs: sourceInputs.map(p => ({file: basename(p), sha256: sha(readFileSync(p))})),
 config_sha256: sha(config), dependencies: deps.map(p => ({file: basename(p), sha256: sha(readFileSync(p))})),
};
writeFileSync(join(args.out, 'kit-manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
const build = buildUpstream(args.upstream);
const require = createRequire(import.meta.url);
const {Registry} = require(join(build.core, 'registry.js'));
const {Config} = require(join(build.core, 'config.js'));
const {MemoryFile} = require(join(build.core, 'files/memory_file.js'));
function oracle(input) {
 const reg = new Registry(new Config(config));
 for (const p of input) reg.addFile(new MemoryFile(basename(p), readFileSync(p, 'utf8')));
 for (const p of deps) reg.addDependency(new MemoryFile(basename(p), readFileSync(p, 'utf8')));
 reg.parse();
 const issues = reg.findIssues();
 return String(issues.length) + issues.map(i => `\n${i.getKey()}|${i.getSeverity()}|${i.getFilename()}|${i.getStart().getRow()}:${i.getStart().getCol()}|${i.getEnd().getRow()}:${i.getEnd().getCol()}|${i.getMessage()}`).join('') + '\n';
}
const results = {schema: 1, differentials: {}, prestore_negatives: {}, seconds: 0};
function run(binary, input, guarded, late = '') {
 const list = join(args.out, 'input-files.txt');
 writeFileSync(list, input.join('\n') + '\n');
 const flags = ['--files-list', list, '--config', configPath, '--deps', depList];
 if (guarded) flags.unshift('--cert-pilot');
 return spawnSync(binary, flags, {encoding: 'utf8', maxBuffer: 64 * 1024 * 1024,
  env: {...process.env, ABAPITI_CERT_LATE_WRITE: late, ABAPITI_CERT_TRACE: guarded ? '1' : ''}});
}
try {
 const variants = {
  clean: [join(args.zabapgit, 'zabapgit_standalone.prog.abap')],
  seeded: [join(args.zabapgit, 'seeded/zabapgit_standalone.prog.abap')],
  'abapgit-src': sourceInputs.map(p => join(sourceKit, basename(p))),
 };
 for (const [label, input] of Object.entries(variants)) {
  const expected = oracle(input);
  writeFileSync(join(args.out, label + '-node.txt'), expected);
  if (label !== 'abapgit-src' && readFileSync(join(args.zabapgit, 'expected-' + label + '.txt'), 'utf8') !== expected) {
   throw new Error('fresh Node oracle differs from supplied kit golden: ' + label);
  }
  for (const [name, binary, guarded] of [['vanilla', args.vanilla, false], ['pilot', args.pilot, true]]) {
   const actual = run(binary, input, guarded);
   writeFileSync(join(args.out, label + '-' + name + '.txt'), actual.stdout ?? '');
   writeFileSync(join(args.out, label + '-' + name + '.err'), actual.stderr ?? '');
   if (actual.error || actual.status !== 0 || actual.stdout !== expected) throw new Error(label + ' ' + name + ' differs/refuses: ' + actual.stderr);
  }
  results.differentials[label] = 'PASS';
  console.log(label + ': Node = vanilla = pilot (' + input.length + ' files)');
 }
 for (const cache of ['StructureParser.singletons', 'Alternative.map', 'SubStructure.matcher', 'sub.singletons']) {
  const actual = run(args.pilot, variants.clean, true, cache);
  const expected = readFileSync(join(args.out, 'clean-node.txt'), 'utf8');
  writeFileSync(join(args.out, 'late-' + cache + '.txt'), actual.stdout ?? '');
  writeFileSync(join(args.out, 'late-' + cache + '.err'), actual.stderr ?? '');
  if (actual.error || actual.status !== 0 || actual.stdout !== expected || !actual.stderr.includes('certificate quarantine:') || !actual.stderr.includes('blocked before store: ' + cache)) {
   throw new Error('missing caught pre-store fallback/equality: ' + cache);
  }
  results.prestore_negatives[cache] = 'PASS';
  console.log(cache + ': blocked before store; quarantine; byte-identical sequential output');
 }
} finally {
 build.dispose();
 results.seconds = (Date.now() - started) / 1000;
 writeFileSync(join(args.out, 'results.json'), JSON.stringify(results, null, 2) + '\n');
}
