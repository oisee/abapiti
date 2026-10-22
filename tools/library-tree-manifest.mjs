#!/usr/bin/env node
// Preserve the frozen .73 M1 workload's logical names, file order and bytes.
// Usage: node tools/library-tree-manifest.mjs KIT > manifest.json
//        node tools/library-tree-manifest.mjs --fixtures > small.json
import {readFileSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {createHash} from 'node:crypto';
const sha = b => createHash('sha256').update(b).digest('hex');
const fixture = new URL('../tsfront/testdata/library-tree/files.json', import.meta.url);
if (process.argv[2] === '--fixtures') {
 process.stdout.write(JSON.stringify({Files:JSON.parse(readFileSync(fixture,'utf8')),
  Config:JSON.stringify({syntax:{version:'open-abap',errorNamespace:'.'},rules:{parser_error:true,check_syntax:true}})})+'\n');
} else {
 if (!process.argv[2]) throw new Error('usage: library-tree-manifest.mjs KIT | --fixtures');
 const kit = resolve(process.argv[2]);
 const reference = JSON.parse(readFileSync(new URL('../tsfront/testdata/selfhost-73/m1/reference.json',import.meta.url),'utf8'));
 const fileList = readFileSync(join(kit,'files.txt'));
 const depsList = readFileSync(join(kit,'deps.txt'));
 const config = readFileSync(join(kit,'abaplint.json'));
 for (const [bytes,key] of [[fileList,'filesListSHA256'],[depsList,'depsListSHA256'],[config,'configSHA256']]) {
  if (sha(bytes) !== reference[key]) throw new Error(`M1 provenance mismatch: ${key}`);
 }
 if (depsList.length) throw new Error('M1 must have no dependencies');
 const names = fileList.toString('utf8').split(/\r?\n/).filter(Boolean);
 const inputs = [], files = [];
 for (const filename of names) {
  const bytes = readFileSync(join(kit,filename));
  inputs.push({filename,dependency:false,sha256:sha(bytes)});
  files.push({Filename:filename,Raw:bytes.toString('utf8')});
 }
 if (files.length !== reference.mainFiles || sha(JSON.stringify(inputs)) !== reference.inputsSHA256) throw new Error('M1 source bytes mismatch');
 console.error(JSON.stringify({Files:files.length, InputsSHA256:sha(JSON.stringify(inputs))}));
 process.stdout.write(JSON.stringify({Files:files,Config:config.toString('utf8')})+'\n');
}
