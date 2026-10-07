#!/usr/bin/env node
// Differential oracle for the phase-1 lexer translation: runs the ORIGINAL
// abaplint Lexer over the corpus and writes the tokens as JSON, one entry
// per case, in exactly the format the lowered ABAP driver produces:
// "Type|str|row|col" lines joined by "\n" (row and col as the lexer reports
// them, 0-based).
//
//   node lexer-oracle.mjs <cases.json> <out.json> [path-to-abaplint-build]
//
// The abaplint core build output is required (~/dev/abaplint/packages/core/
// build). Nothing is installed into the abaplint checkout.
import {readFileSync, writeFileSync} from "node:fs";
import {createRequire} from "node:module";
import {resolve} from "node:path";

const [casesPath, outPath, coreArg] = process.argv.slice(2);
if (!casesPath || !outPath) {
  console.error("usage: node lexer-oracle.mjs <cases.json> <out.json> [abaplint-core-build]");
  process.exit(2);
}
const core = resolve(coreArg || "/home/alice/dev/abaplint/packages/core/build/src");
const require = createRequire(import.meta.url);
const {Lexer} = require(core + "/abap/1_lexer/lexer.js");
const {MemoryFile} = require(core + "/files/memory_file.js");

const cases = JSON.parse(readFileSync(casesPath, "utf8"));
const out = [];
for (const c of cases) {
  const file = new MemoryFile("zcorpus.prog.abap", c.abap);
  const tokens = new Lexer().run(file).tokens;
  const lines = [];
  for (const t of tokens) {
    lines.push(`${t.constructor.name}|${t.getStr()}|${t.getRow()}|${t.getCol()}`);
  }
  out.push({name: c.name, dump: lines.join("\n"), tokens: tokens.length});
  console.error(`${c.name}: ${tokens.length} tokens`);
}
writeFileSync(outPath, JSON.stringify(out, null, 1));
console.error(`wrote ${outPath}`);
