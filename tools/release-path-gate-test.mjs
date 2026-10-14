#!/usr/bin/env node
import assert from 'node:assert/strict';
import {spawnSync, execFileSync} from 'node:child_process';
import {mkdtempSync, mkdirSync, writeFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join, resolve} from 'node:path';
const work = mkdtempSync(join(tmpdir(), 'release-path-gate-test-'));
const gate = resolve(import.meta.dirname, 'release-path-gate.py');
try {
 const assets = join(work, 'assets'); mkdirSync(assets);
 const run = () => spawnSync('python3', [gate, assets, '--root', '/build/private', '--user', 'builder_name'], {encoding:'utf8'});
 writeFileSync(join(assets, 'clean.json'), '{"source":"src/lsp/help.ts:222:7"}');
 assert.equal(run().status, 0);
 writeFileSync(join(assets, 'unrelated.txt'), 'builder_nameblue currentcolor|aliceblue|antiquewhite');
 assert.equal(run().status, 0, 'unrelated word is not a build-user identity');
 const boundary = Buffer.concat([Buffer.alloc(1024 * 1024 - 'builder_name'.length, 32), Buffer.from('builder_nameblue')]);
 writeFileSync(join(assets, 'boundary.bin'), boundary);
 assert.equal(run().status, 0, 'defer a chunk-boundary username until its following byte');
 writeFileSync(join(assets, 'boundary.bin'), Buffer.concat([boundary, Buffer.from(' /builder_name/') ]));
 assert.equal(run().status, 1, 'a complete identity after a chunk boundary is rejected');
 rmSync(join(assets,'boundary.bin'));
 for (const [i, token] of ['/mnt/private/', '/home/private/', '/tmp/private/', '/build/private', 'builder_name'].entries()) {
  writeFileSync(join(assets, `binary-${i}`), Buffer.concat([Buffer.from('ELF\0'), Buffer.from(token)]));
 }
 const bad = run(); assert.equal(bad.status, 1, bad.stderr);
 assert.equal(JSON.parse(bad.stdout).filter(r => r.leakedEntries).length, 5);
 for (let i=0;i<5;i++) rmSync(join(assets, `binary-${i}`));
 const entries = join(work, 'entries'); mkdirSync(entries);
 writeFileSync(join(entries,'binary'), Buffer.from('ELF\0/tmp/closure/src/a.ts:1:1'));
 execFileSync('zip',['-q',join(assets,'leak.zip'),'binary'],{cwd:entries});
 execFileSync('tar',['-czf',join(assets,'leak.tar.gz'),'-C',entries,'binary']);
 const archives = run(); assert.equal(archives.status,1, archives.stderr);
 assert.equal(JSON.parse(archives.stdout).filter(r => r.leakedEntries).length,2);
 rmSync(join(assets,'leak.zip')); rmSync(join(assets,'leak.tar.gz'));
 // Gate archive entry names too, even when contents are clean.
 writeFileSync(join(entries,'builder_name.txt'),'relative source');
 execFileSync('zip',['-q',join(assets,'name.zip'),'builder_name.txt'],{cwd:entries});
 assert.equal(run().status,1);
 rmSync(join(assets,'name.zip'));
 // Isolate the raw archive-comment check from entry names/content.
 execFileSync('python3',['-c',`import zipfile,sys
with zipfile.ZipFile(sys.argv[1], 'w') as z:
 z.writestr('clean.txt', 'relative source')
 z.comment = b'/tmp/private-build'
`,join(assets,'comment.zip')]);
 assert.equal(run().status,1);
 console.log('release-path-gate: clean files pass; all forbidden tokens, binary strings, zip/tar entries, names and archive comments rejected');
} finally {rmSync(work,{recursive:true,force:true});}
