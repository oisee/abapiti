#!/usr/bin/env node
// Reproduce the runtime's identifier tables from the installed pinned package.
// No host Unicode-version approximation, source overrides or network access.
import {createRequire} from 'node:module';
import {writeFileSync} from 'node:fs';
import {resolve, join} from 'node:path';
import {homedir} from 'node:os';
const require=createRequire(import.meta.url);
const [pkg=join(homedir(),'dev/abaplint/packages/core/node_modules/json5'), output='hir/golang/json5_unicode.go']=process.argv.slice(2);
if (require(resolve(pkg,'package.json')).version !== '2.2.3') throw new Error('expected json5 2.2.3');
const unicode=require(resolve(pkg,'lib/unicode.js'));
let source='package golang\n\n// Pinned json5 2.2.3 Unicode identifier tables, compressed from lib/unicode.js.\n// Range data avoids depending on the host Go Unicode version. json5 is MIT:\n// Copyright (c) 2012-2018 Aseem Kishore and others (see\n// tsfront/testdata/registrycorpus/node-packages/json5/LICENSE.md).\nconst json5UnicodeSource = `\n';
for (const [name,expression] of [['json5IDStart',unicode.ID_Start],['json5IDContinue',unicode.ID_Continue]]) {
 source+=`var ${name} = [][2]int32{\n`;
 let start;
 for (let cp=0;cp<=0x110000;cp++) {
  const matches=cp<0x110000 && expression.test(String.fromCodePoint(cp));
  if (matches && start === undefined) start=cp;
  if (!matches && start !== undefined) {
   source+=`{0x${start.toString(16)},0x${(cp-1).toString(16)}},`;
   start=undefined;
  }
 }
 source+='\n}\n';
}
writeFileSync(output,source+'`\n');
