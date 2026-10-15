#!/usr/bin/env node
// Syntax-check translated core/tests and the matching readable registry driver.
// Corpus/log runtime remains a dependency; its optional BAL adapter needs SAP.
import {createRequire} from 'node:module';
import {readFileSync,readdirSync} from 'node:fs';
import {resolve,join} from 'node:path';
const [buildArg,osgArg=process.env.ABAPITI_OSG_HOME]=process.argv.slice(2);
if(!buildArg||!osgArg) throw new Error('usage: a4h-tests-lint.mjs BUILD OSG');
const build=resolve(buildArg),osg=resolve(osgArg);
const lint=createRequire(join(osg,'package.json'))('@abaplint/core');
const config=lint.Config.getDefault();
// Existing narrow release exception: PCRE needs 7.55, other emitted constructs
// remain 7.50-style. Do not pretend PCRE parses as REGEX on a 7.50 kernel.
config.get().syntax.version=lint.Version.v755;
config.get().syntax.errorNamespace='.*';
config.get().rules={parser_error:true,check_syntax:true};
const reg=new lint.Registry(config);
let objects=0;
for(const part of ['core','tests','registry']) for(const name of readdirSync(join(build,part))) {
 if(!/\.(?:abap|xml)$/.test(name)) continue;
 if(part==='registry'&&!/^(?:zcl_lnt_registry_a4h|zlnt_registry_(?:run|clean|neg))\./.test(name)) continue;
 const source=readFileSync(join(build,part,name),'utf8');
 if(name.endsWith('.abap')) {
  if(Buffer.byteLength(name.split('.')[0])>30) throw new Error(`overlong name ${name}`);
  for(const line of source.split('\n')) if(Buffer.byteLength(line)>255) throw new Error(`overlong line ${name}`);
 }
 reg.addFile(new lint.MemoryFile(name,source));objects++;
}
function dependencies(dir) {
 for(const entry of readdirSync(dir,{withFileTypes:true})) {
  const file=join(dir,entry.name);
  if(entry.isDirectory()) dependencies(file);
  else if(/\.(?:abap|xml)$/.test(entry.name)&&!entry.name.includes('testclasses')) reg.addDependency(new lint.MemoryFile(file,readFileSync(file,'utf8')));
 }
}
dependencies(join(osg,'.local/lars/open-abap-core/src'));
dependencies(resolve(import.meta.dirname,'allbackends/corpus-abap/src'));
reg.parse();
const issues=reg.findIssues();
for(const issue of issues) console.error(`${issue.getFilename()}:${issue.getStart().getRow()}: ${issue.getMessage()}`);
console.log(`A4H core/tests/registry driver ABAP v755 (PCRE exception): ${objects} files, ${issues.length} issues`);
process.exitCode=issues.length?1:0;
