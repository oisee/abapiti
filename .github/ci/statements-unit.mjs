// Complete statement differentials and fail-closed aggregate mutations, on the
// same clean runtime pin used by lexer-unit.mjs. No runtime source patches.
import {execFileSync, spawnSync} from 'node:child_process';
import {cpSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
const [workArg] = process.argv.slice(2);
if (!workArg) throw new Error('usage: statements-unit.mjs WORK');
const work = resolve(workArg), osg = join(work, 'open-steamgate');
const root = resolve(import.meta.dirname, '../..');
const pin = readFileSync(join(root, '.github/ci/osgo.ref'), 'utf8').trim();
function clean() {
  const git = (...args) => execFileSync('git', args, {cwd: osg, encoding: 'utf8'}).trim();
  if (git('rev-parse', 'HEAD') !== pin || git('status', '--porcelain', '--untracked-files=all')) throw new Error('runtime must be clean at CI pin');
}
clean();
const generated = join(work, 'statements');
rmSync(generated, {recursive: true, force: true});
execFileSync('go', ['test', './tsfront', '-run', '^TestEmitStatementsClosure$', '-count=1'], {
  cwd: root, env: {...process.env, STMTS_EMIT: '1', ABAPITI_TEST_OUT: generated}, stdio: 'inherit'
});
execFileSync('node', [join(root, '.github/ci/hir-lint.mjs'), generated, osg], {stdio: 'inherit'});
const testfile = readdirSync(generated).find(f => f.endsWith('.clas.testclasses.abap'));
const original = readFileSync(join(generated, testfile), 'utf8');
if (!original.includes('act = completed exp = 64') || !original.includes('act = equal_statements exp = 926')) throw new Error('64/926 aggregate assertions missing');
function mutate(which) {
  const calls = [...original.matchAll(/tokens = (\d+) statements = (\d+)\./g)];
  const index = which === 'tokens' ? 1 : 2;
  const selected = calls.find(m => Number(m[index]) > 0);
  if (!selected) throw new Error('no positive oracle count to mutate');
  const replacement = selected[0].replace(`${which} = ${selected[index]}`, `${which} = 0`);
  return original.slice(0, selected.index) + replacement + original.slice(selected.index + selected[0].length);
}
for (const runtime of ['osgo', 'osgjs']) {
  for (const label of ['baseline', 'tokens', 'statements']) {
    const dir = label === 'baseline' ? generated : join(work, `statements-mutation-${label}`);
    if (label !== 'baseline') {
      rmSync(dir, {recursive: true, force: true}); mkdirSync(dir, {recursive: true});
      cpSync(generated, dir, {recursive: true}); writeFileSync(join(dir, testfile), mutate(label));
    }
    const result = spawnSync('timeout', ['--foreground', '--kill-after=5s', '1800s', 'npm', 'run', '-s', `${runtime}:unit`, '--', dir, '--json'], {
      cwd: osg, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024,
      env: {...process.env, GOTOOLCHAIN: 'go1.26.0', GOFLAGS: '-buildvcs=false'}
    });
    writeFileSync(join(work, `statements-${runtime}-${label}.json`), result.stdout || '');
    writeFileSync(join(work, `statements-${runtime}-${label}.err`), result.stderr || '');
    if (result.error) throw result.error;
    const data = JSON.parse(result.stdout), t = data.totals;
    if (t.tests !== 2 || data.rows?.length !== 2 || t.error || t.not_compiled || t.skipped || data.overrides?.length) throw new Error(`${runtime} ${label}: invalid runtime report`);
    const pass = label === 'baseline';
    if (pass ? result.status !== 0 || t.success !== 2 || data.rows.some(r => r.status !== 'SUCCESS') : result.status !== 1 || t.failure !== 1 || t.success !== 1 || data.rows.some(r => !['SUCCESS', 'FAILURE'].includes(r.status))) throw new Error(`${runtime} ${label}: unexpected verdict ${JSON.stringify(data.rows)}`);
    console.log(`${runtime} statements ${label}: ${pass ? 'PASS (64/64 cases, 926/926 statements, plus regressions)' : 'expected FAIL'}`);
  }
}
clean();
