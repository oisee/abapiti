// Run from the abapiti repository root. Frozen clones stay source-clean;
// pack fetches and build generators run in an untracked archive copy.
import {existsSync, mkdirSync, readFileSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {execFileSync} from 'node:child_process';
const sha = '3ee404ca1260464221f5eba7d86a06a38eabc6c4';
const root = resolve('.osg'), build = resolve('.tmp/selfhost/osg');
const run = (cmd, args, cwd) => execFileSync(cmd, args, {cwd, stdio: 'inherit'});
function clone(repo, ref, dir) {
  if (!existsSync(join(dir, '.git'))) {
    mkdirSync(dir, {recursive:true});
    run('git', ['init', '-q'], dir);
    run('git', ['remote', 'add', 'origin', `https://github.com/${repo}`], dir);
    run('git', ['fetch', '--depth', '1', 'origin', ref], dir);
    run('git', ['checkout', '--detach', ref], dir);
  }
  const actual = execFileSync('git', ['rev-parse', 'HEAD'], {cwd:dir, encoding:'utf8'}).trim();
  if (actual !== ref) throw new Error(`wrong pin for ${repo}: ${actual}`);
  if (execFileSync('git', ['status', '--porcelain', '--untracked-files=no'], {cwd:dir, encoding:'utf8'}).trim()) throw new Error(`dirty source: ${repo}`);
}
clone('oisee/open-steamgate', sha, root);
const lock = JSON.parse(readFileSync(join(root,'libs.lock.json')));
for (const lib of lock.libraries) clone(lib.repo, lib.ref, join(root,'.local/lars',lib.folder));
if (existsSync(build)) throw new Error('remove .tmp/selfhost/osg before preparing a fresh build view');
mkdirSync(build,{recursive:true});
const archive = execFileSync('git',['archive',sha],{cwd:root,maxBuffer:128*1024*1024});
execFileSync('tar',['-x','-C',build],{input:archive});
// Archive library sources into the build view so converter imports resolve
// its locked node_modules without writing dependencies into frozen clones.
for (const lib of lock.libraries) {
  const target = join(build,'.local/lars',lib.folder);
  mkdirSync(target,{recursive:true});
  const bytes = execFileSync('git',['archive',lib.ref],{cwd:join(root,'.local/lars',lib.folder),maxBuffer:128*1024*1024});
  execFileSync('tar',['-x','-C',target],{input:bytes});
}
run('npm',['ci','--ignore-scripts','--no-audit','--no-fund'],build);
run(process.execPath,['tools/osd-fetch.mjs'],build);
// The first generator reports transpiler provenance; it generates no ABAP.
// Run the build's full sequence using the pinned package-lock dependencies.
run(process.execPath,['--input-type=module','-e',
  `import {runGenerators} from './tools/osd-build.mjs'; console.log(runGenerators(process.cwd(), console.log));`],build);
console.log('Prepared .tmp/selfhost/osg; frozen .osg sources untouched.');
