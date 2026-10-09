#!/usr/bin/env node
// Original-JS observations for tsfront/testdata/syntaxfeatures/probe.ts: the
// pinned upstream TypeScript compiler runs the fixture, the ABAP side must
// reproduce every string exactly.
import {readFileSync,writeFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import {resolve} from 'node:path';
import {verifyUpstream} from './statements-upstream.mjs';
const require=createRequire(import.meta.url);
const ts=require(verifyUpstream(process.argv[2] ?? '/home/alice/dev/abaplint'));
const file=resolve('tsfront/testdata/syntaxfeatures/probe.ts');
const compiled=ts.transpileModule(readFileSync(file,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
const module={exports:{}};
new Function('exports','module',compiled)(module.exports,module);
const inputs=[['abcdef','b',7,true],['hello','x',-3,false],['Z','',0,false],['a  ',' ',42,true],['€😀xy','😀',9007199254740991,true]];
const cases=inputs.map(([raw,needle,n,flag])=>({raw,needle,n,flag,expected:new module.exports.Probe().run(raw,needle,n,flag)}));
writeFileSync('tsfront/testdata/syntaxfeatures/oracle.json',JSON.stringify(cases,null,2)+'\n');
