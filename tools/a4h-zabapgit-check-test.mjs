import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,writeFileSync,chmodSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {goBuildShim} from './a4h-zabapgit-check.mjs';
test('native build flags are bounded; non-build calls pass through unchanged',()=>{
 const dir=mkdtempSync(join(tmpdir(),'a4h-shim-'));
 try {
  const go=join(dir,"real go's launcher"),shim=join(dir,'go');
  writeFileSync(go,'#!/usr/bin/env node\nconsole.log(JSON.stringify(process.argv.slice(2)))\n');chmodSync(go,0o755);
  writeFileSync(shim,goBuildShim(go));chmodSync(shim,0o755);
  const invoke=args=>JSON.parse(execFileSync(shim,args,{encoding:'utf8'}));
  assert.deepEqual(invoke(['version']),['version']);
  assert.deepEqual(invoke(['build','-o','output with spaces','./cmd/osabap']),['build','-p','1','-gcflags=osg/gogen/cmd/osabap=-N -l -dwarf=false','-o','output with spaces','./cmd/osabap']);
 } finally {rmSync(dir,{recursive:true,force:true})}
});
