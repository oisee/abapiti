#!/usr/bin/env node
// Fresh pinned upstream, never the translated harness or an existing build.
import {readFileSync, writeFileSync} from "node:fs";
import {createRequire} from "node:module";
import {createHash} from "node:crypto";
import {statementsDump} from "./statements-dump.mjs";
import {buildUpstream} from "./statements-upstream.mjs";
const [casesPath, outPath, upstream] = process.argv.slice(2);
if (!casesPath || !outPath) throw new Error("usage: structures-oracle.mjs cases.json dumps.json [upstream]");
const {core, dispose} = buildUpstream(upstream);
try {
 const require = createRequire(import.meta.url);
 const {Lexer} = require(core + "/abap/1_lexer/lexer.js");
 const {StatementParser} = require(core + "/abap/2_statements/statement_parser.js");
 const {StructureParser} = require(core + "/abap/3_structures/structure_parser.js");
 const {TokenNode} = require(core + "/abap/nodes/token_node.js");
 const {StatementNode} = require(core + "/abap/nodes/statement_node.js");
 const {MemoryFile} = require(core + "/files/memory_file.js");
 const {Release} = require(core + "/version.js");
 const structures = require(core + "/abap/3_structures/structures/index.js");
 const covered = new Set();
 const out = JSON.parse(readFileSync(casesPath, "utf8")).map(c => {
  const file = new MemoryFile(c.filename || "ztest.prog.abap", c.abap);
  const start = performance.now();
  const lexed = new Lexer().run(file);
  const afterLex = performance.now();
  const parsed = new StatementParser(Release.v758).run([{file, tokens:lexed.tokens}], [])[0];
  const afterStatements = performance.now();
  const refs = new Map(parsed.statements.map((s,i) => [s,i]));
  const result = StructureParser.run(parsed);
  const afterStructures = performance.now();
  let count = 0;
  function node(n, depth) {
   if (n instanceof StatementNode) {
    if (!refs.has(n)) throw new Error("unknown statement reference");
    return `\n${depth}|T${refs.get(n)}|${n.get().constructor.name}`;
   }
   count++; covered.add(n.get().constructor.name);
   return `\n${depth}|S${n.get().constructor.name}|${n.getChildren().length}` + n.getChildren().map(s => node(s,depth+1)).join("");
  }
  let dump = `${lexed.tokens.length}|${parsed.statements.length}|${result.issues.length}`;
  for (const i of result.issues) dump += `\nI|${i.getKey()}|${i.getSeverity()}|${i.getFilename()}|${i.getStart().getRow()}:${i.getStart().getCol()}|${i.getEnd().getRow()}:${i.getEnd().getCol()}|${i.getMessage()}`;
  if (result.node) dump += node(result.node, 0);
  const sha = text => createHash("sha256").update(text).digest("hex");
  const lexerDump = lexed.tokens.map(t=>`${t.constructor.name}|${t.getStr()}|${t.getRow()}|${t.getCol()}`).join("\n");
  return {lexerSHA256:sha(lexerDump), statementsSHA256:sha(statementsDump(parsed.statements,TokenNode)), lex_ms:afterLex-start, statements_ms:afterStatements-afterLex, structures_ms:afterStructures-afterStatements, name:c.name, dump, tokens:lexed.tokens.length, statements:parsed.statements.length, structures:count, issues:result.issues.length, sha256:createHash("sha256").update(dump).digest("hex"), ms:Math.round(performance.now()-start)};
 });
 writeFileSync(outPath, JSON.stringify(out,null,1)+"\n");
 const missing = Object.keys(structures).filter(n => !covered.has(n));
 console.error(JSON.stringify({cases:out.length,covered:[...covered].sort(),missing}));
} finally { dispose(); }
