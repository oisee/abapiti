import assert from 'node:assert/strict';
import test from 'node:test';
import {mkdtempSync, writeFileSync, mkdirSync, rmSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {osgBuildSettings, osgBuildArgs, osgFixtureObjects, osgNodeHeap, osgPinnedCheckout} from './osg-harness-input.mjs';

test('fast build flags stay the TS-HA harness flags, scoped to the cmd package', () => {
  const settings = {debug: false, mode: 'fast', gcflags: ['-N', '-l', '-dwarf=false']};
  assert.deepEqual(osgBuildArgs('abapitests', settings),
    ['build', '-p', '1', '-trimpath', '-ldflags=-s -w', '-gcflags=osg/gogen/cmd/abapitests=-N -l -dwarf=false']);
  assert.deepEqual(osgBuildArgs('step4b', settings),
    ['build', '-p', '1', '-trimpath', '-ldflags=-s -w', '-gcflags=osg/gogen/cmd/step4b=-N -l -dwarf=false']);
});

test('debug keeps symbols, default mode adds nothing to gcflags', () => {
  assert.deepEqual(osgBuildArgs('abapitests', {debug: true, mode: 'fast', gcflags: ['-N', '-l']}),
    ['build', '-p', '1', '-trimpath', '-gcflags=osg/gogen/cmd/abapitests=-N -l']);
  assert.deepEqual(osgBuildArgs('abapitests', {debug: false, mode: 'default', gcflags: []}),
    ['build', '-p', '1', '-trimpath', '-ldflags=-s -w']);
});

test('settings: GOGEN_NOLINE default, debug opt-out, mode validation', () => {
  const fresh = () => ({});
  const env = fresh(); osgBuildSettings(env);
  assert.equal(env.GOGEN_NOLINE, '1'); // before emit-go.mjs is imported
  const preset = fresh(); preset.GOGEN_NOLINE = '0'; osgBuildSettings(preset);
  assert.equal(preset.GOGEN_NOLINE, '0'); // an explicit value is never overwritten
  const debug = fresh(); debug.ABAPITI_OSG_DEBUG = '1';
  assert.deepEqual(osgBuildSettings(debug), {debug: true, mode: 'fast', gcflags: ['-N', '-l']});
  const slow = fresh(); slow.ABAPITI_OSG_BUILD_MODE = 'default';
  assert.deepEqual(osgBuildSettings(slow).gcflags, ['-dwarf=false']);
  assert.throws(() => osgBuildSettings({ABAPITI_OSG_BUILD_MODE: 'turbo'}), /default or fast/);
});

test('fixture objects: only .clas/.intf.abap, uppercased base names', () => {
  const dir = mkdtempSync(join(tmpdir(), 'osg-input-test-'));
  try {
    writeFileSync(join(dir, 'zcl_a.clas.abap'), 'CLASS zcl_a.\nENDCLASS.');
    writeFileSync(join(dir, 'zif_b.intf.abap'), 'INTERFACE zif_b.\nENDINTERFACE.');
    writeFileSync(join(dir, 'zcl_c.clas.testclasses.abap'), 'x');
    writeFileSync(join(dir, 'names.json'), '{}');
    const {abapFiles, objects} = osgFixtureObjects(dir);
    assert.deepEqual(abapFiles.sort(), ['zcl_a.clas.abap', 'zif_b.intf.abap']);
    assert.deepEqual([...objects].sort(), ['ZCL_A', 'ZIF_B']);
  } finally { rmSync(dir, {recursive: true, force: true}); }
});

test('heap/gc re-exec is a no-op once both flags are present', () => {
  const saved = process.execArgv, savedOptions = process.env.NODE_OPTIONS;
  try {
    process.execArgv = ['--expose-gc', '--max-old-space-size=12288'];
    osgNodeHeap(); // returns; a re-exec would exit the test runner first
    process.execArgv = [];
    process.env.NODE_OPTIONS = '--expose-gc --max-old-space-size=8192';
    osgNodeHeap();
    process.env.NODE_OPTIONS = undefined;
  } finally { process.execArgv = saved; process.env.NODE_OPTIONS = savedOptions; }
});

test('pinned checkout refuses a non-repository directory', () => {
  const dir = mkdtempSync(join(tmpdir(), 'osg-pin-test-'));
  try {
    mkdirSync(join(dir, 'sub'));
    assert.throws(() => osgPinnedCheckout(join(dir, 'sub')), /fatal|not a git|ENOTDIR|Unknown revision/);
  } finally { rmSync(dir, {recursive: true, force: true}); }
});
