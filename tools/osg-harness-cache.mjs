// Caller-owned, content-addressed snapshots of frontend output and executables.
// Never cache verdicts: every run executes the binary in a fresh process.
import {createHash} from 'node:crypto';
import {copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, renameSync, rmSync, statSync, writeFileSync} from 'node:fs';
import {join,dirname,relative,resolve,isAbsolute,sep} from 'node:path';

// Resolve symlinks in the existing ancestor before creating caller output.
export function externalPath(path, protectedRoot) {
 const absolute=resolve(path);
 let ancestor=absolute;
 while (!existsSync(ancestor)) ancestor=dirname(ancestor);
 const canonical=resolve(realpathSync(ancestor),relative(ancestor,absolute));
 const inside=relative(realpathSync(protectedRoot),canonical);
 if (!inside || (!inside.startsWith('..'+sep) && inside!=='..' && !isAbsolute(inside))) throw new Error('OSG output/cache must be outside protected inputs');
 return absolute;
}

const digest = bytes => createHash('sha256').update(bytes).digest('hex');
export function snapshotKey(trees, metadata) {
 const hash = createHash('sha256');
 const add = value => { const bytes=Buffer.from(value); hash.update(String(bytes.length)+':'); hash.update(bytes); };
 add(JSON.stringify({schema:1,metadata}));
 function walk(path, name, ancestors) {
  const real=realpathSync(path), stat=statSync(path);
  if (stat.isDirectory()) {
   if (ancestors.has(real)) throw new Error('cache input symlink cycle: '+name);
   const next=new Set([...ancestors,real]);
   add('directory:'+name);
   for (const entry of readdirSync(path).sort()) {
    if (['.git','.out','.bin'].includes(entry)) continue;
    walk(join(path,entry),name+'/'+entry,next);
   }
  } else if (stat.isFile()) { add('file:'+name); add(readFileSync(path)); }
  else throw new Error('unsupported cache input: '+name);
 }
 trees.forEach((path,i)=>walk(path,String(i),new Set()));
 return hash.digest('hex');
}
// A snapshot is a set of named files plus JSON evidence. Missing, partial or
// corrupt entries are misses; nothing is ever served as a verdict.
export function readSnapshot(root, key, targets) {
 const entry=join(root,key);
 if (!existsSync(join(entry,'complete.json'))) return null;
 try {
  const meta=JSON.parse(readFileSync(join(entry,'complete.json'),'utf8'));
  if (meta.schema!==2 || meta.key!==key || meta.evidenceSHA256!==digest(JSON.stringify(meta.evidence))) return null;
  const names=Object.keys(targets).sort();
  if (JSON.stringify(names)!==JSON.stringify(Object.keys(meta.files).sort())) return null;
  for (const name of names) if (meta.files[name]!==digest(readFileSync(join(entry,name)))) return null;
  for (const name of names) { mkdirSync(dirname(targets[name]),{recursive:true}); copyFileSync(join(entry,name),targets[name]); }
  return meta.evidence;
 } catch { return null; }
}
export function writeSnapshot(root, key, sources, evidence) {
 mkdirSync(root,{recursive:true});
 const entry=join(root,key), temp=mkdtempSync(join(root,'.pending-'));
 try {
  const files={};
  for (const [name,source] of Object.entries(sources)) {
   if (!/^[a-z0-9][a-z0-9._-]*$/.test(name)) throw new Error('invalid snapshot file name: '+name);
   copyFileSync(source,join(temp,name)); files[name]=digest(readFileSync(join(temp,name)));
  }
  writeFileSync(join(temp,'complete.json'),JSON.stringify({schema:2,key,files,evidenceSHA256:digest(JSON.stringify(evidence)),evidence}));
  // Publish once. Parallel writers cannot expose a partial snapshot.
  try { renameSync(temp,entry); }
  catch (error) { if (!['EEXIST','ENOTEMPTY'].includes(error.code)) throw error; }
 } finally { rmSync(temp,{recursive:true,force:true}); }
}
