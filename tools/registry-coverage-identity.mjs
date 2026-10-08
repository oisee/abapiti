// Resolve all eligible source declarations; ambiguity must never authorize a trap.
export function declarationCandidates(ts, sf, candidates, mappings, fn, kind) {
 const range = fn.ranges[0];
 const found = new Map();
 for (const m of mappings) {
  if (m.offset < range.startOffset || m.offset >= range.endOffset) continue;
  const pos = sf.getPositionOfLineAndCharacter(m.originalLine-1,m.originalColumn);
  for (const n of candidates) {
   const ctor = ts.isConstructorDeclaration(n);
   const name = ctor ? n.parent.name?.text : n.name?.text;
   if (ts.SyntaxKind[n.kind] !== kind || (fn.functionName !== name && !(ctor && fn.functionName === 'constructor'))) continue;
   // Class headers often precede the TS constructor in emitted JS. Its body
   // mappings identify the constructor, while method headers identify methods.
   if (n.getStart(sf) <= pos && pos < (ctor ? n.end : n.body.getStart(sf))) found.set(`${n.pos}:${n.end}:${n.kind}`,n);
  }
 }
 return [...found.values()];
}
