#!/usr/bin/env node
// Small original-Node oracles for the rules initializer representation boundaries.
// These are regressions, not replacement registrations or suite pass credits.
import {execFileSync} from 'node:child_process';
import {dirname, join, resolve} from 'node:path';
import {homedir} from 'node:os';
import {buildUpstream} from './statements-upstream.mjs';

const upstream=resolve(process.argv[2] ?? join(homedir(),'dev/abaplint'));
const build=buildUpstream(upstream);
try {
 const script=String.raw`
const assert=require('node:assert/strict');
const path=require('node:path');
const root=process.argv[1];
const {LineLength,LineLengthConf}=require(path.join(root,'src/rules/line_length.js'));
const {NoDynamicStuffConf}=require(path.join(root,'src/rules/no_dynamic_stuff.js'));
const {KeywordCase,KeywordCaseConf,KeywordCaseStyle}=require(path.join(root,'src/rules/keyword_case.js'));

// Erasing an array element type does not change or copy the original row.
const rows=[{description:'one',config:{flag:true}},{description:'two',config:{flag:false}}];
const seen=[];
rows.forEach(test=>{seen.push(test);test.config.flag=!test.config.flag;});
assert.equal(seen[0],rows[0]);assert.equal(seen[1],rows[1]);
assert.deepEqual(rows.map(test=>test.config.flag),[false,true]);

// Object-spread config stays the setter's original reference, with no constructor.
const source=new KeywordCaseConf();
const config={...source,style:KeywordCaseStyle.Lower};
const keyword=new KeywordCase();keyword.setConfig(config);
assert.equal(keyword.getConfig(),config);
config.ignoreGlobalClassBoundaries=true;
assert.equal(keyword.getConfig().ignoreGlobalClassBoundaries,true);
assert.notEqual(config,source);

// Assignment expressions return the RHS Number, NOT the newly allocated config.
const allocated=new LineLengthConf();
const value=allocated.length=400;
assert.equal(value,400);assert.equal(typeof value,'number');
const line=new LineLength();line.setConfig(value);
assert.equal(line.getConfig(),400);
const missing=['length','maxIssuesPerFile','severity'].map(key=>line.getConfig()[key]);
assert.deepEqual(missing,[undefined,undefined,undefined]);
assert.equal(10 > line.getConfig().length,false);

// Object.keys includes constructor-created own fields, including superclass fields.
const native=new NoDynamicStuffConf();
const keys=Object.keys(native);
const flags=keys.filter(key=>typeof native[key]==='boolean');
assert.deepEqual(flags,['callMethod','callFunction','callDatabaseProcedure','callTransformation','callTransaction','perform','submit','createObject','createData','getBadi','assign','internalTable','exportImport']);
const nonBoolean=keys.filter(key=>typeof native[key]!=='boolean').map(key=>[key,native[key]]);
for(const enable of ['callMethod','internalTable']) {
 const config=new NoDynamicStuffConf();
 for(const key of Object.keys(config)) if(typeof config[key]==='boolean') config[key]=false;
 config[enable]=true;
 assert.deepEqual(flags.filter(key=>config[key]),[enable]);
 assert.deepEqual(nonBoolean.map(([key])=>[key,config[key]]),nonBoolean);
 assert.deepEqual(Object.keys(config),keys);
}
console.log(JSON.stringify({erasedRows:seen.length,spreadConfigIdentity:true,primitiveConfig:{value,missingProperties:['length','maxIssuesPerFile','severity']},nativeKeys:keys,booleanFlags:flags,enabledCases:['callMethod','internalTable']}));
`;
 process.stdout.write(execFileSync(process.execPath,['-e',script,dirname(build.core)],{encoding:'utf8'}));
} finally {build.dispose();}
