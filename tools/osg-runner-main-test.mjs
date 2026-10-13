import assert from 'node:assert/strict';
import {test} from 'node:test';
import {mkdtempSync, mkdirSync, writeFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
import {runnerMain} from './osg-runner-main.mjs';
import {reportStatus} from './osg-runner-resume.mjs';

test('generated Go executable agrees with reportStatus, including initialization-only refusals', () => {
 const dir = mkdtempSync(join(tmpdir(), 'osg-runner-main-'));
 try {
  mkdirSync(join(dir, 'abap'));
  writeFileSync(join(dir, 'go.mod'), 'module osg/gogen\n\ngo 1.26\n');
  writeFileSync(join(dir, 'abap/session.go'), 'package abap\ntype Session struct{}\n');
  writeFileSync(join(dir, 'main.go'), runnerMain);
  writeFileSync(join(dir, 'fixture.go'), `package main
import("os";"osg/gogen/abap")
func ZCL_ABAPITI_TEST_RUNNER_RUN(s *abap.Session, start int32, skip string) string {
 if start != 1 || skip != ",2," {panic("runner inputs lost")}
 return os.Getenv("ABAPITI_FIXTURE_REPORT")
}
`);
  writeFileSync(join(dir, 'names.json'), '{}');
  const binary = join(dir, 'runner');
  const build = spawnSync('go', ['build', '-p', '1', '-o', binary, '.'], {cwd:dir, encoding:'utf8'});
  assert.equal(build.status, 0, build.stderr || build.error?.message);
  const clean = {passed:1, failed:0, trapped:0, asyncPending:0, notRun:0,
   results:[{title:'pass', passed:true, status:'passed', message:''}], initialization:[]};
  const cases = [clean,
   ...['failed','trapped'].map(status => ({...clean, initialization:[{file:'init.ts', action:'host_init_2', status, message:'refused'}]})),
   ...['failed','trapped','asyncPending'].map(key => ({...clean, [key]:1}))];
  for (const report of cases) {
   const run = spawnSync(binary, [join(dir, 'names.json')], {encoding:'utf8', env:{...process.env,
    ABAPITI_OSG_START_INDEX:'1', ABAPITI_OSG_SKIP_INIT:',2,', ABAPITI_FIXTURE_REPORT:JSON.stringify(report)}});
   assert.equal(run.status, reportStatus(report), run.stderr);
   assert.deepEqual(JSON.parse(run.stdout), report);
  }
 } finally { rmSync(dir, {recursive:true, force:true}); }
});
