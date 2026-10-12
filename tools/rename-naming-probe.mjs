#!/usr/bin/env node
// Exercise the original object_naming rule and default Registry config without
// a rename. The same body runs against fresh pinned Node source and TS-HG@Go.
import {createRequire, stripTypeScriptTypes} from 'node:module';
import {mkdirSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {buildUpstream} from './statements-upstream.mjs';
import {materializeRegistryClosure} from './registry-closure.mjs';

const [upstream, compiler, output] = process.argv.slice(2);
if (!upstream || !compiler || !output) throw Error('usage: rename-naming-probe.mjs upstream compiler output-directory');
const out = resolve(output);
mkdirSync(out, {recursive:true});
const body = `function decision(): string {
 const reg = new Registry();
 reg.addFile(new MemoryFile("#foo#cl_bar.clas.abap", "CLASS /foo/cl_bar DEFINITION PUBLIC CREATE PUBLIC.\\nENDCLASS.\\nCLASS /foo/cl_bar IMPLEMENTATION.\\nENDCLASS."));
 reg.addFile(new MemoryFile("#foo#cl_bar.clas.xml", '\uFEFF<?xml version="1.0" encoding="utf-8"?><abapGit version="v1.0.0" serializer="LCL_OBJECT_CLAS"><asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0"><asx:values><VSEOCLASS><CLSNAME>/FOO/CL_BAR</CLSNAME><LANGU>E</LANGU><DESCRIPT>Description</DESCRIPT><STATE>1</STATE><CLSCCINCL>X</CLSCCINCL><FIXPT>X</FIXPT><UNICODE>X</UNICODE></VSEOCLASS></asx:values></asx:abap></abapGit>'));
 reg.parse();
 const direct = new ObjectNaming().initialize(reg).run(reg.getFirstObject()!);
 const issues = reg.findIssues();
 return reg.getFirstObject()!.getName()+"|"+direct.length+"|"+(direct[0]?.getKey() || "missing")+"|"+issues.length+"|"+(issues[0]?.getKey() || "missing");
}`;
const oracle = buildUpstream(upstream);
let expected;
try {
 const require = createRequire(import.meta.url);
 const {Registry} = require(join(oracle.core, 'registry.js'));
 const {MemoryFile} = require(join(oracle.core, 'files/memory_file.js'));
 const {ObjectNaming} = require(join(oracle.core, 'rules/object_naming.js'));
 expected = Function('Registry','MemoryFile','ObjectNaming',stripTypeScriptTypes(body)+'\nreturn decision();')(Registry,MemoryFile,ObjectNaming);
} finally { oracle.dispose(); }
if (expected !== '/FOO/CL_BAR|1|object_naming|1|object_naming') throw Error(`unexpected Node oracle: ${expected}`);
writeFileSync(join(out,'node.json'), JSON.stringify({expected},null,2)+'\n');
const closure = join(out,'closure');
const manifest = materializeRegistryClosure(closure, upstream, {roots:['src/registry.ts','src/rules/object_naming.ts'],runtime:resolve('tsfront/testdata/abaplint-tests/runtime')});
const file = 'test/naming_probe.ts';
const source = `import {Registry} from "../src/registry";\nimport {MemoryFile} from "../src/files/memory_file";\nimport {ObjectNaming} from "../src/rules/object_naming";\nimport {expect} from "chai";\n${body}\nit("namespace object_naming without rename",()=>expect(decision()).to.equal(${JSON.stringify(expected)}));\n`;
mkdirSync(join(closure,'test'));
writeFileSync(join(closure,file),source);
manifest.roots = [file];
manifest.sources.push({file,sha256:createHash('sha256').update(source).digest('hex')});
manifest.initialization.push(file);
writeFileSync(join(closure,'closure.json'),JSON.stringify(manifest,null,2)+'\n');
let result;
try { result = execFileSync(resolve(compiler),['abaplint-tests',closure,'-o',join(out,'compiled')],{encoding:'utf8',maxBuffer:128*1024*1024,env:{...process.env,GOFLAGS:`${process.env.GOFLAGS ?? ''} -p=2 -buildvcs=false`}}); }
catch (error) { if (!error.stdout) throw error; result = error.stdout; }
writeFileSync(join(out,'go.json'),result);
const run = JSON.parse(result);
console.log(JSON.stringify({expected,passed:run.passed,failed:run.failed,trapped:run.trapped,results:run.results},null,2));
if (run.passed !== 1 || run.failed || run.trapped || run.initialization.length) process.exitCode = 1;
