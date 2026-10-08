#!/usr/bin/env node
import {readFileSync,writeFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import {resolve} from 'node:path';
import {verifyUpstream} from './statements-upstream.mjs';
const require=createRequire(import.meta.url);
const ts=require(verifyUpstream(process.argv[2] ?? '/home/alice/dev/abaplint'));
const file=resolve('tsfront/testdata/registryfeatures/probe.ts');
const compiled=ts.transpileModule(readFileSync(file,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
const module={exports:{}};
new Function('exports','module',compiled)(module.exports,module);
const inputs=[['abc','b',7,true],['abc','x',-3,false],['','',0,false],['a  ',' ',42,true],['€😀','😀',9007199254740991,true],['a\n\t`|','\t',-9007199254740991,false]];
const cases=inputs.map(([raw,needle,n,flag])=>({raw,needle,n,flag,expected:module.exports.Probe.run(raw,needle,n,flag)}));
writeFileSync('tsfront/testdata/registryfeatures/oracle.json',JSON.stringify(cases,null,2)+'\n');

const xmlFile=resolve('tsfront/testdata/registryfeatures/xml.ts');
const xmlCompiled=ts.transpileModule(readFileSync(xmlFile,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
const xmlModule={exports:{}};
new Function('exports','module',xmlCompiled)(xmlModule.exports,xmlModule);
writeFileSync('tsfront/testdata/registryfeatures/tagged-oracle.json',JSON.stringify([true,false].map(flag=>({flag,expected:xmlModule.exports.XMLProbe.tagged(flag)})),null,2)+'\n');
