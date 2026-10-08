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

const arrayFile=resolve('tsfront/testdata/registryfeatures/arrays.ts');
const arrayCompiled=ts.transpileModule(readFileSync(arrayFile,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
const arrayModule={exports:{}};
new Function('exports','module',arrayCompiled)(arrayModule.exports,arrayModule);
writeFileSync('tsfront/testdata/registryfeatures/arrays-oracle.json',JSON.stringify({shift:arrayModule.exports.ArrayProbe.shift(),optionalIndex:arrayModule.exports.ArrayProbe.optionalIndex(),staticCollections:arrayModule.exports.ArrayProbe.staticCollections(),find:arrayModule.exports.ArrayProbe.find(),unionViews:arrayModule.exports.ArrayProbe.unionViews(),namedRecord:arrayModule.exports.ArrayProbe.namedRecord()},null,2)+'\n');

const iteratorFile=resolve('tsfront/testdata/registryfeatures/iterators.ts');
const iteratorCompiled=ts.transpileModule(readFileSync(iteratorFile,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
const iteratorModule={exports:{}};
new Function('exports','module',iteratorCompiled)(iteratorModule.exports,iteratorModule);
writeFileSync('tsfront/testdata/registryfeatures/iterators-oracle.json',JSON.stringify({run:iteratorModule.exports.IteratorProbe.run()},null,2)+'\n');

const jsonFile=resolve('tsfront/testdata/registryfeatures/json.ts');
const jsonCompiled=ts.transpileModule(readFileSync(jsonFile,'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS}}).outputText;
const jsonModule={exports:{}};
new Function('exports','module',jsonCompiled)(jsonModule.exports,jsonModule);
jsonModule.exports.JSONProbe.config=JSON.parse;
const configCases=[{global:{files:'/src/**/*.abap'},syntax:{version:'v702',errorNamespace:'^Z',globalConstants:['B','A','B'],ambigiousVoids:['Z','Z']},rules:{unknown_rule:false},targetRules:null,extra:{retained:[1,true,null]}},{global:{files:'x',skipIncludesWithoutMain:true,errorOnDuplicateFilenames:true},syntax:{errorNamespace:'^Y'},rules:{}}];
writeFileSync('tsfront/testdata/registryfeatures/config-oracle.json',JSON.stringify(configCases.map(input=>({input:JSON.stringify(input),expected:jsonModule.exports.JSONProbe.defaults(JSON.stringify(input))})),null,2)+'\n');
