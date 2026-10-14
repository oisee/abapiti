#!/usr/bin/env node
// Build only: never connects to SAP. Run under the shared heavy-job lock.
import {discoverShards} from './abaplint-tests-all.mjs';
import {materializeShardedClosure} from './abaplint-tests-sharded-closure.mjs';
import {resolve, join} from 'node:path';
import {mkdirSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
const [upstream, output] = process.argv.slice(2);
if (!output) throw new Error('usage: a4h-tests-build.mjs UPSTREAM OUTPUT');
const out=resolve(output), root=resolve(import.meta.dirname,'..');
mkdirSync(out,{recursive:true});
const plan=discoverShards(resolve(upstream));
if(plan.shards.length!==17) throw new Error('expected pinned 17-shard union');
materializeShardedClosure(join(out,'closure'),resolve(upstream),plan.shards,join(root,'tsfront/testdata/abaplint-tests/runtime'));
execFileSync('go',['run','./cmd/abapiti','abaplint-tests',join(out,'closure'),'-o',out,'--target','a4h'],{cwd:root,stdio:'inherit'});
