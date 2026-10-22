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

// V8 starts a static method range at its name, excluding the static modifier.
// Keep both AST positions; source-map candidate ambiguity still refuses pruning.
export function generatedDeclarationKinds(ts, source) {
 const kinds = new Map();
 function visit(n) {
  if (ts.isClassDeclaration(n) || ts.isClassExpression(n)) kinds.set(n.getStart(source), 'Constructor');
  else if (ts.isMethodDeclaration(n) || ts.isFunctionDeclaration(n) || ts.isGetAccessor(n) || ts.isSetAccessor(n)) {
   kinds.set(n.getStart(source), ts.SyntaxKind[n.kind]);
   if (n.name && ts.isMethodDeclaration(n)) kinds.set(n.name.getStart(source), ts.SyntaxKind[n.kind]);
  }
  ts.forEachChild(n, visit);
 }
 visit(source);
 return kinds;
}
