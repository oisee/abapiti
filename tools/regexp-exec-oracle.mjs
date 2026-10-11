#!/usr/bin/env node
// Independent Node observations for the checked RegExp subset.
import {readFileSync,writeFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import {verifyUpstream} from './statements-upstream.mjs';
const require=createRequire(import.meta.url);
const ts=require(verifyUpstream(process.argv[2] ?? '/home/alice/dev/abaplint'));
const dir='tsfront/testdata/regexp-exec';
const compiled=ts.transpileModule(readFileSync(`${dir}/probe.ts`,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
const module={exports:{}};
new Function('exports','module',compiled)(module.exports,module);
writeFileSync(`${dir}/oracle.json`,JSON.stringify({run:module.exports.Probe.run()},null,2)+'\n');
