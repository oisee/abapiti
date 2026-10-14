#!/usr/bin/env node
// Focused, offline readable-name proof through the existing pinned OSG harness.
// Does not run the all-suite scheduler; no OSG files are edited.
import {discoverShards,selectShards,assertNoFailures} from './abaplint-tests-all.mjs';
import {materializeShardedClosure} from './abaplint-tests-sharded-closure.mjs';
import {buildUpstream} from './statements-upstream.mjs';
import {executeOracle} from './abaplint-tests-oracle.mjs';
import {ownedResults} from './abaplint-tests-owned.mjs';
import {resolve,join} from 'node:path';
import {mkdirSync,writeFileSync,readFileSync} from 'node:fs';
import {execFileSync,spawnSync} from 'node:child_process';
const [upstreamArg,outArg]=process.argv.slice(2);
if(!outArg||!process.env.ABAPITI_OSG_HOME) throw new Error('usage: a4h-tests-osg-check.mjs UPSTREAM OUTPUT (ABAPITI_OSG_HOME required)');
const upstream=resolve(upstreamArg),out=resolve(outArg),root=resolve(import.meta.dirname,'..');
mkdirSync(out,{recursive:true});
const plan=selectShards(discoverShards(upstream),'abap/lexer,abap/types,utils');plan.target='osg';
writeFileSync(join(out,'shards.json'),JSON.stringify(plan));
const {manifests}=materializeShardedClosure(join(out,'closure'),upstream,plan.shards,join(root,'tsfront/testdata/abaplint-tests/runtime'));
const env={...process.env,ABAPITI_NAMES:'readable',ABAPITI_OSG_BUILD_ONLY:'1'};
execFileSync('go',['run','./cmd/abapiti','abaplint-tests',join(out,'closure'),'-o',join(out,'build'),'--target','osg','--osg-home',env.ABAPITI_OSG_HOME],{cwd:root,env,stdio:'inherit'});
const oracle=buildUpstream(upstream);
try {
 for(const {name,id,roots} of plan.shards) {
  let node=executeOracle(oracle,upstream,roots);
  const execution=spawnSync(process.execPath,[join(root,'tools/osg-runner-execute.mjs'),join(out,'build/osg/abaplint-tests-osg'),join(out,'build/abap/names.json'),join(out,id+'-host'),name],{cwd:root,env,encoding:'utf8',maxBuffer:128*1024*1024});
  if(![0,1].includes(execution.status)) throw new Error(execution.stderr ?? 'OSG execution failed');
  let run=JSON.parse(execution.stdout);assertNoFailures(name,run);
  const foreign=manifests.get(name).sources.map(s=>s.file).filter(f=>f.startsWith('test/')&&!roots.includes(f));
  if(foreign.length) ({oracle:node,run}=ownedResults(node,run,executeOracle(oracle,upstream,foreign),foreign));
  writeFileSync(join(out,`node-${id}.json`),JSON.stringify(node));
  writeFileSync(join(out,`go-${id}.json`),JSON.stringify(run));
  mkdirSync(join(out,id),{recursive:true});
  writeFileSync(join(out,id,'closure.json'),JSON.stringify({roots,upstreamPin:plan.upstreamPin}));
 }
 execFileSync(process.execPath,[join(root,'tools/abaplint-tests-summary.mjs'),out,join(out,'summary.json')],{cwd:root,stdio:'inherit'});
 const summary=JSON.parse(readFileSync(join(out,'summary.json'),'utf8'));
 if(summary.nodeActive.notPassing || summary.uniqueTotal.failed || !summary.registrationParity || summary.shards.some(s=>s.initialization.length)) throw new Error('readable OSG parity gate failed');
 console.log(`${summary.nodeActive.passed}/${summary.nodeActive.total} Node-active; FAIL 0`);
} finally {oracle.dispose()}
