// Imported test-bearing modules execute in their importers too. Count them only
// in the shard containing their root file. Preserve duplicate titles as multisets;
// refuse ambiguous own/borrowed title collisions, rather than guessing ownership.
export function ownedResults(oracle, run, borrowed, files) {
 if (!borrowed.total) return {oracle, run};
 const counts = new Map();
 for (const title of borrowed.titles) counts.set(title, (counts.get(title) ?? 0) + 1);
 const remaining = new Map(counts);
 const titles = oracle.titles.filter(title => {
  const n = remaining.get(title) ?? 0;
  if (!n) return true;
  remaining.set(title, n - 1); return false;
 });
 if ([...remaining.values()].some(n => n)) throw new Error('borrowed Node titles missing from shard');
 if (titles.some(t => counts.has(t))) throw new Error('ambiguous own/borrowed title collision');
 const node = {...oracle, titles, borrowed:{files, total:borrowed.total, passed:borrowed.passed, failed:borrowed.failed, pending:borrowed.pending}};
 for (const key of ['total','passed','failed','pending']) node[key] -= borrowed[key];
 for (const key of ['passedTitles','failedTitles','pendingTitles']) {
  const remove = new Map();
  for (const title of borrowed[key]) remove.set(title, (remove.get(title) ?? 0) + 1);
  node[key] = oracle[key].filter(title => {
   const n = remove.get(title) ?? 0;
   if (!n) return true;
   remove.set(title,n-1); return false;
  });
  if ([...remove.values()].some(n => n)) throw new Error(`borrowed ${key} mismatch`);
 }
 node.failures = oracle.failures.filter(t => !counts.has(t.title));
 const available = new Map(counts);
 const results = run.results.filter(r => {
  const n = available.get(r.title) ?? 0;
  if (!n) return true;
  available.set(r.title,n-1); return false;
 });
 return {oracle:node, run:{...run, results, rawRegistered:run.results.length, borrowedRegistered:run.results.length-results.length}};
}
