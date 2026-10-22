#!/usr/bin/env node
// Original @abaplint/core oracle. No translated implementation is imported.
// Usage: node tools/library-tree-reference.mjs CORE_DIR MANIFEST > reference.jsonl
// MANIFEST: {Files: [{Filename,Raw}], Config: string}. Files retain input order.
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {createRequire} from 'node:module';
const require = createRequire(import.meta.url);
const corePath = resolve(process.argv[2]);
if (require(resolve(corePath, "package.json")).version !== "2.120.73") throw new Error("expected @abaplint/core 2.120.73");
const core = require(corePath);
const {Registry, Config, MemoryFile, ABAPObject, Nodes, Tokens} = core;
const input = JSON.parse(readFileSync(process.argv[3], 'utf8'));
const registry = new Registry(new Config(input.Config));
for (const file of input.Files) registry.addFile(new MemoryFile(file.Filename, file.Raw));
registry.parse();
const pos = p => ({Row:p.getRow(), Col:p.getCol()});
const token = t => ({Str:t.getStr(), Start:pos(t.getStart()), End:pos(t.getEnd()), Pragma:t instanceof Tokens.Pragma});
const statement = n => ({Kind:n.get().constructor.name, Statement:true,
 Start:pos(n.getFirstToken().getStart()), End:pos(n.getLastToken().getEnd()),
 Tokens:n.getTokens().map(token), Children:null, FirstStatement:null});
function firstDFS(n) {
 for (const c of n.getChildren()) {
  if (c instanceof Nodes.StatementNode) return c;
  const found = firstDFS(c);
  if (found) return found;
 }
}
function structure(n) {
 const first = n.getFirstStatement();
 const start = n.getChildren().length ? pos(n.getFirstToken().getStart()) : {Row:0,Col:0};
 const end = n.getChildren().length ? n.getLastToken()?.getEnd() : undefined;
 return {Kind:n.get().constructor.name, Statement:false, Start:start, End:end ? pos(end) : start,
  Tokens:null, Children:n.getChildren().map(c => c instanceof Nodes.StatementNode ? statement(c) : structure(c)),
  FirstStatement:first && first !== firstDFS(n) ? statement(first) : null};
}
const issue = i => ({Filename:i.getFilename(), Key:i.getKey(), Severity:i.getSeverity(), Message:i.getMessage(),
 Row:i.getStart().getRow(), Column:i.getStart().getCol(), EndRow:i.getEnd().getRow(), EndColumn:i.getEnd().getCol()});
const counts = {Objects:0, ABAPObjects:0, Files:input.Files.length, ABAPFiles:0, FileStatements:0, FileTokens:0, PragmaTokens:0, StructureStatements:0, Structures:0, StructureTokens:0, Issues:0};
function count(n) {
 if (n.Statement) { counts.StructureStatements++; counts.StructureTokens += n.Tokens.length; }
 else { counts.Structures++; for (const c of n.Children) count(c); }
}
// Match encoding/json's string escaping, including JS line separators.
const json = value => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, c => '\\u'+c.charCodeAt(0).toString(16).padStart(4,'0'));
for (const obj of [...registry.getObjects()].sort((a,b) => (a.getType()+'\0'+a.getName()) < (b.getType()+'\0'+b.getName()) ? -1 : 1)) {
 const typ = obj.getType(), name = obj.getName();
 let files = [];
 counts.Objects++;
 if (obj instanceof ABAPObject) {
  counts.ABAPObjects++;
  if (typ === 'CLAS' || typ === 'INTF') files = obj.getSequencedFiles();
  else {
   const main = obj.getMainABAPFile();
   files = [...(main ? [main] : []), ...obj.getABAPFiles().filter(f => f !== main)];
  }
 }
 const trees = files.map(f => ({Filename:f.getFilename(), Structure:f.getStructure() ? structure(f.getStructure()) : null,
  Statements:f.getStatements().map(statement)}));
 const concat = files.map(f => f.getStatements().map(s => s.concatTokens()));
 const issues = registry.findIssuesObject(registry.getObject(typ,name)).map(issue);
 for (const f of trees) {
  counts.ABAPFiles++;
  counts.FileStatements += f.Statements.length;
  for (const s of f.Statements) { counts.FileTokens += s.Tokens.length; counts.PragmaTokens += s.Tokens.filter(t => t.Pragma).length; }
  if (f.Structure) count(f.Structure);
 }
 counts.Issues += issues.length;
 process.stdout.write(json({Type:typ, Name:name, Found:true, Files:trees, Concat:concat, Issues:issues})+'\n');
}
console.error(JSON.stringify(counts));
