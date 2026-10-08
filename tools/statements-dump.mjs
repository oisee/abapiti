// Shared serialization of original upstream StatementNodes. References and
// full expression/token trees are stable across the Node and ABAP harnesses.
export function statementsDump(statements, TokenNode) {
 function node(n) {
  if(n instanceof TokenNode){const t=n.get();return `T${t.constructor.name}[${t.getStr()}]`;}
  return `E${n.get().constructor.name}(${n.getChildren().map(node).join(",")})`;
 }
 const lines=[String(statements.length)];
 for(const s of statements){const f=s.getFirstToken(),l=s.getLastToken(),c=s.getColon();lines.push(`${s.get().constructor.name}|${c===undefined?"-":`${c.getRow()}:${c.getCol()}`}|${f.getRow()}:${f.getCol()}|${l.getRow()}:${l.getCol()}|${s.getChildren().map(node).join(",")}`);}
 return lines.join("\n");
}
