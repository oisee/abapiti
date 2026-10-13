// Union sources are lowered once. Initialization is NOT the union order:
// retain each original shard's dependency-first order, including borrowed tests.
import {writeFileSync, rmSync} from 'node:fs';
import {join} from 'node:path';
import {materializeRegistryClosure} from './registry-closure.mjs';

export function materializeShardedClosure(out, upstream, shards, runtime, materialize = materializeRegistryClosure) {
 const manifests = new Map();
 for (const shard of shards) {
  const scratch = `${out}-manifest-${shard.id}`;
  try { manifests.set(shard.name, materialize(scratch, upstream, {roots:shard.roots, runtime})); }
  finally { rmSync(scratch, {recursive:true, force:true}); }
 }
 const manifest = materialize(out, upstream, {roots:shards.flatMap(s => s.roots), runtime});
 manifest.shards = shards.map(({name}) => ({name, initialization:manifests.get(name).initialization}));
 const available = new Set(manifest.sources.map(s => s.file));
 for (const shard of manifest.shards) {
  if (new Set(shard.initialization).size !== shard.initialization.length || shard.initialization.some(f => !available.has(f))) throw new Error(`invalid initialization closure: ${shard.name}`);
 }
 writeFileSync(join(out, 'closure.json'), JSON.stringify(manifest, null, 2) + '\n');
 return {manifest, manifests};
}

export function buildGroups(shards, size = shards.length) {
 if (!Number.isInteger(size) || size < 1 || size > 17) throw new Error('ABAPITI_OSG_SHARDS_PER_BUILD must be an integer from 1 to 17');
 return Array.from({length:Math.ceil(shards.length / size)}, (_, i) => shards.slice(i * size, (i + 1) * size));
}
