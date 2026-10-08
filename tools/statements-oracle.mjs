#!/usr/bin/env node
// Differential oracle for the phase-2 statement-parser translation: runs the
// ORIGINAL abaplint Lexer + StatementParser over the corpus and writes one
// dump per case in exactly the format the lowered ABAP driver produces
// (harness/statements_dump.ts):
//   <statementCount>
//   <StatementClass>|<colon row:col or ->|<first row:col>|<last row:col>|<tree>
// with tree nodes `T<TokenClass>[str]` and `E<ExpressionClass>(children...)`.
// Rows and columns are 0-based, as the lexer reports them.
//
//   node statements-oracle.mjs <cases.json> <out.json> [abaplint-core-build]
//
// Nothing is installed into the abaplint checkout; the existing build output
// is loaded through createRequire.
import {readFileSync, writeFileSync} from "node:fs";
import {createRequire} from "node:module";
import {resolve} from "node:path";

const [casesPath, outPath, coreArg] = process.argv.slice(2);
if (!casesPath || !outPath) {
  console.error("usage: node statements-oracle.mjs <cases.json> <out.json> [abaplint-core-build]");
  process.exit(2);
}
const core = resolve(coreArg || "/home/alice/dev/abaplint/packages/core/build/src");
const require = createRequire(import.meta.url);
const {Lexer} = require(core + "/abap/1_lexer/lexer.js");
const {StatementParser} = require(core + "/abap/2_statements/statement_parser.js");
const {Release} = require(core + "/version.js");
const {MemoryFile} = require(core + "/files/memory_file.js");
const {TokenNode} = require(core + "/abap/nodes/token_node.js");

function node(n) {
  if (n instanceof TokenNode) {
    const t = n.get();
    return `T${t.constructor.name}[${t.getStr()}]`;
  }
  const inner = n.getChildren().map(node).join(",");
  return `E${n.get().constructor.name}(${inner})`;
}

function dump(abap) {
  const file = new MemoryFile("zcorpus.prog.abap", abap);
  const lexed = new Lexer().run(file);
  const parsed = new StatementParser(Release.v758).run([{file, tokens: lexed.tokens}], []);
  const lines = [String(parsed[0].statements.length)];
  for (const st of parsed[0].statements) {
    const first = st.getFirstToken();
    const last = st.getLastToken();
    const colon = st.getColon() === undefined ? "-" : `${st.getColon().getRow()}:${st.getColon().getCol()}`;
    const tree = st.getChildren().map(node).join(",");
    lines.push(
      `${st.get().constructor.name}|${colon}` +
      `|${first.getRow()}:${first.getCol()}|${last.getRow()}:${last.getCol()}` +
      `|${tree}`);
  }
  return {dump: lines.join("\n"), tokens: lexed.tokens.length, statements: parsed[0].statements.length};
}

const cases = JSON.parse(readFileSync(casesPath, "utf8"));
const out = [];
for (const c of cases) {
  let d;
  try {
    d = dump(c.abap);
  } catch (err) {
    d = {tokens: -1, statements: -1, dump: `EXCEPTION|${err && err.constructor ? err.constructor.name : "Error"}|${String(err && err.message ? err.message : err)}`};
  }
  out.push({name: c.name, ...d});
  console.error(`${c.name}: ${d.statements} statements`);
}
writeFileSync(outPath, JSON.stringify(out, null, 1));
console.error(`wrote ${outPath}`);
