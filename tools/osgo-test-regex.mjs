// The emitted ABAP keeps JS-compatible absolute assertions: the pinned JS
// transpiler cannot read PCRE \A/\z. Only OSGO's invocation-local IR gets RE2's
// exactly equivalent assertions. Never rewrite source patterns, other classes,
// arbitrary lookarounds, or the shared OSG checkout.
export function adaptTestRegex(program, names) {
 const runtimeNames = new Set(Object.entries(names).filter(([,entry]) => entry.id === 'runtime.regexp_js').map(([name]) => name));
 let changed = 0;
 function constant(node) {
  if (!node || typeof node !== 'object') return undefined;
  if (['str','chars'].includes(node.e)) return node.value;
  if (node.e === 'conv' && node.kind === 'c2s') return constant(node.x);
  if (node.e === 'concat') {
   const a=constant(node.l), b=constant(node.r);
   if (a !== undefined && b !== undefined) return a+b;
  }
 }
 function walk(node) {
  if (!node || typeof node !== 'object') return;
  const text = constant(node);
  if (typeof text === 'string') {
   const converted = text.replaceAll('(?<![\\s\\S])','\\A').replaceAll('(?![\\s\\S])','\\z')
    .replaceAll('(?s:.)','(?:[^\\n]|\\n)');
   if (converted !== text) {
    // Folding a constant concat also covers assertions split by ABAP's literal
    // chunking. Dynamic concat operands and all unrelated literals stay intact.
    for (const key of Object.keys(node)) delete node[key];
    Object.assign(node,{e:'str',value:converted,type:{k:'string'}});
    changed++;
   }
   return;
  }
  for (const value of Object.values(node)) {
   if (Array.isArray(value)) value.forEach(walk);
   else walk(value);
  }
 }
 for (const cls of program.classes) {
  if (!runtimeNames.has(cls.name)) continue;
  for (const method of new Set([...(cls.methods ?? []),cls.constructor].filter(Boolean))) walk(method.body);
 }
 return {literalSites:changed};
}
