#!/usr/bin/env node
// OSGO currently ignores ordinary ABAP WRITE. Surface the registry driver's
// returned report after an unchanged successful unit run; never fail a test
// merely to display timings, or write into the read-only OSG checkout.
import {mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync} from 'node:fs';
import {dirname, join, relative, resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {execFileSync, spawnSync} from 'node:child_process';

const [resultArg, osgArg] = process.argv.slice(2);
if (!resultArg || !osgArg) throw new Error('usage: osgo-registry-report.mjs UNIT_RESULT.json OSG');
const osg = realpathSync(resolve(osgArg));
const pin = readFileSync(new URL('../.github/ci/osgo.ref', import.meta.url), 'utf8').trim();
const git = (...args) => execFileSync('git', ['-C', osg, ...args], {encoding:'utf8'}).trim();
if (git('rev-parse', 'HEAD') !== pin || git('status', '--porcelain')) throw new Error('OSG must be clean at ' + pin);
const unit = JSON.parse(readFileSync(resultArg, 'utf8'));
if (unit.classes !== 1 || unit.compiled !== 1 || unit.rows?.length !== 1 ||
    unit.rows[0].class !== 'ZCL_ABAPITI_REGISTRY_RUN' || unit.rows[0].method !== 'RUN' || unit.rows[0].status !== 'SUCCESS') {
  throw new Error('STOP: registry unit must succeed before measuring its report');
}
const go = realpathSync(resolve(unit.buildDir));
const inside = relative(osg, go);
if (!inside.startsWith('../')) throw new Error('unit build directory must be outside OSG');
// The normal --input unit invocation places the driver in the app layer.
if (!readFileSync(join(go, 'generated/app/zz_generated.go'), 'utf8').includes('func ZCL_ABAPITI_REGISTRY_RUN_RUN(')) {
  throw new Error('expected the normal layered registry unit build');
}
const command = mkdtempSync(join(go, 'cmd/registry-report-'));
const scratch = mkdtempSync(join(tmpdir(), 'abapiti-registry-report-'));
try {
  writeFileSync(join(command, 'main.go'), `package main
import("fmt";"os";"osg/gogen/abap";"osg/gogen/generated/app")
func main(){
 var report string
 ok:=app.ZCL_ABAPITI_REGISTRY_RUN_RUN(&abap.Session{},&report)
 fmt.Println(report)
 if ok!="X" {os.Exit(1)}
}
`);
  const binary = join(scratch, 'report');
  execFileSync('go', ['build', '-trimpath', '-o', binary, './cmd/' + relative(join(go, 'cmd'), command)], {cwd:go, stdio:'inherit'});
  const run = spawnSync(binary, [], {cwd:dirname(binary), stdio:'inherit'});
  if (run.error) throw run.error;
  process.exitCode = run.status ?? 2;
} finally {
  rmSync(command, {recursive:true, force:true});
  rmSync(scratch, {recursive:true, force:true});
}
