import assert from 'node:assert/strict';
import {test} from 'node:test';
import {mkdtempSync, mkdirSync, readFileSync, rmSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {materializeShardedClosure, buildGroups} from './abaplint-tests-sharded-closure.mjs';

test('union sources retain dependency-first per-shard registration orders, including borrowed tests', () => {
 const dir = mkdtempSync(join(tmpdir(), 'sharded-closure-'));
 const shards = [{name:'a',id:'a',roots:['test/a.ts']}, {name:'b',id:'b',roots:['test/b.ts']}];
 try {
  const orders = [['runtime/mocha.ts','test/shared.ts','test/a.ts'], ['runtime/mocha.ts','test/b.ts','test/shared.ts']];
  const calls = [];
  const materialize = (out, upstream, {roots}) => {
   calls.push(roots);
   mkdirSync(out);
   const initialization = roots.length === 2 ? [...new Set(orders.flat())] : orders[roots[0] === 'test/a.ts' ? 0 : 1];
   return {roots, initialization, sources:initialization.map(file => ({file}))};
  };
  const out = join(dir,'union');
  const {manifest,manifests} = materializeShardedClosure(out,'upstream',shards,'runtime',materialize);
  assert.deepEqual(calls,[['test/a.ts'],['test/b.ts'],['test/a.ts','test/b.ts']]);
  assert.deepEqual(manifest.shards.map(s => s.initialization),orders);
  assert.deepEqual(manifests.get('a').initialization,orders[0]);
  assert.deepEqual(JSON.parse(readFileSync(join(out,'closure.json'))),manifest);
  assert.deepEqual(buildGroups(shards),[shards]);
  assert.deepEqual(buildGroups(shards,1),[[shards[0]],[shards[1]]]);
  for (const size of [0,18,NaN,1.5]) assert.throws(() => buildGroups(shards,size),/integer/);
 } finally {rmSync(dir,{recursive:true,force:true});}
});
