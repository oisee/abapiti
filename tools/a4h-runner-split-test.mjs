#!/usr/bin/env node
// Optional real archive proof: BEFORE_BUILD AFTER_BUILD PARTS_DIRECTORY.
import assert from 'node:assert/strict';
import {readFileSync, readdirSync} from 'node:fs';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';

function methods(source) {
 return new Map([...source.matchAll(/^\s*METHOD (\w+)\.\n([\s\S]*?)^\s*ENDMETHOD\./gm)].map(m => [m[1], m[2]]));
}
function flatten(source) {
 const bodies = methods(source);
 const visit = (name, stack = []) => {
  assert(!stack.includes(name), 'recursive helper');
  assert(bodies.has(name), `missing ${name}`);
  return bodies.get(name).replace(/^\s*(?:result = )?((?:run_|init_)\w+)\( (?:start_index = start_index )?skip_init = skip_init \)\.\n/gm,
   (_, callee) => visit(callee, [...stack, name]));
 };
 return visit('run');
}
// Ignore layout only outside ABAP literals. New helper indentation changes
// wrapping; literal contents, checkpoint JSON, indices and ordering stay exact.
function tokens(source) {
 return source.match(/`(?:``|[^`])*`|'(?:''|[^'])*'|\|(?:\\.|[^|])*\||[^\s`'|]+/g) ?? [];
}
function equivalent(before, after) {
 let flat = flatten(after);
 // Undo the five explicitly relocated local declarations, at their original
 // positions. This is the only storage plumbing ignored by the proof.
 for (const m of after.matchAll(/^\s*CLASS-DATA (init_error|init_assertion|init_trap|error_class|error_text) (TYPE [^\n]+)\./gm)) {
  flat = flat.replace(new RegExp(`CLEAR ${m[1]}\\.`), `DATA ${m[1]} ${m[2]}.`);
 }
 assert.deepEqual(tokens(flat), tokens(methods(before).get('run')), 'flattened RUN differs');
}
const old = 'METHOD run.\nCASE shard.\nWHEN `a`.\ncheckpoint( `x.y` ).\nENDCASE.\nENDMETHOD.\n';
const split = 'METHOD run.\nCASE shard.\nWHEN `a`.\nresult = run_a( start_index = start_index skip_init = skip_init ).\nENDCASE.\nENDMETHOD.\nMETHOD run_a.\ninit_a( skip_init = skip_init ).\nENDMETHOD.\nMETHOD init_a.\ncheckpoint( `x.y` ).\nENDMETHOD.\n';
equivalent(old, split);
assert.throws(() => equivalent(old, split.replace('x.y', 'x.z')), /flattened RUN differs/);
assert.throws(() => equivalent(old, split.replace('init_a( skip_init', 'init_missing( skip_init')), /missing/);

const [before, after, parts] = process.argv.slice(2);
if (before || after || parts) {
 assert(before && after && parts, 'usage: a4h-runner-split-test.mjs BEFORE AFTER PARTS');
 const runner = build => readFileSync(join(build, 'tests/zcl_lnt_test_runner.clas.abap'), 'utf8');
 equivalent(runner(before), runner(after));
 const baseZip = JSON.parse(readFileSync(join(parts, 'parts.json'), 'utf8')).parts.base.zip;
 const partRunner = execFileSync('python3', ['-c', "import sys,zipfile; print(zipfile.ZipFile(sys.argv[1]).read('src/zcl_lnt_test_runner.clas.abap').decode(),end='')", join(parts,baseZip)], {encoding:'utf8',maxBuffer:16*1024*1024});
 equivalent(runner(before), partRunner);
 const shards = [...runner(before).matchAll(/^\s*WHEN `([^`]+)`\./gm)].length;
 // Reopen every emitted test ZIP, including the monolithic archive; count
 // actual METHOD/ENDMETHOD lines in every class, not just the runner.
 const archives = [join(after, 'zlnt_tests.zip'), ...readdirSync(parts).filter(f => f.endsWith('.zip')).map(f => join(parts,f))];
 const sizes = JSON.parse(execFileSync('python3', ['-c', `import sys,zipfile,re,json
out=[]
for archive in sys.argv[1:]:
 with zipfile.ZipFile(archive) as z:
  for file in z.namelist():
   if not file.endswith('.clas.abap'): continue
   source=z.read(file).decode()
   for m in re.finditer(r'^\\s*METHOD ([\\w~]+)\\.\\n(.*?)^\\s*ENDMETHOD\\.',source,re.M|re.S):
    out.append([m[0].count('\\n')+1,archive,file,m[1]])
print(json.dumps(sorted(out,reverse=True)[:10]))`, ...archives], {encoding:'utf8',maxBuffer:1024*1024}));
 assert(sizes[0][0] <= 5000, JSON.stringify(sizes));
 const previous = Math.max(...[...methods(runner(before)).values()].map(b => b.split('\n').length + 1));
 console.log(JSON.stringify({shards, equivalentRunners:2, archives:archives.length, before:previous, maxMethod:sizes[0]}));
}
console.log('a4h-runner-split: recursive flattening, literal/order mutations, method limits OK');
