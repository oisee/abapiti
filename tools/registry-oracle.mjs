#!/usr/bin/env node
// Original Registry integration oracle. Never imports a pre-existing build.
import {readFileSync, readdirSync, writeFileSync, mkdirSync} from "node:fs";
import {join, resolve, relative} from "node:path";
import {createRequire} from "node:module";
import {createHash} from "node:crypto";
import {execFileSync} from "node:child_process";
import {buildUpstream, upstreamPin} from "./statements-upstream.mjs";
import {statementsDump} from "./statements-dump.mjs";

const [inputDir, depsDir, configPath, outputDir, upstream] = process.argv.slice(2);
if (!outputDir) throw new Error("usage: registry-oracle.mjs input-dir deps-src config.json output-dir [upstream]");
const sha = s => createHash("sha256").update(s).digest("hex");
// Sort record keys only. Array/file/child order remains observable.
function normalize(v) {
 if (Array.isArray(v)) return v.map(normalize);
 if (v && typeof v === "object") return Object.fromEntries(Object.keys(v).sort().map(k => [k, normalize(v[k])]));
 return v;
}
const canonical = v => JSON.stringify(normalize(v));
function files(root) {
 const out = [];
 function walk(dir) {
  for (const entry of readdirSync(dir, {withFileTypes:true}).sort((a,b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0)) {
   const path = join(dir, entry.name);
   if (entry.isDirectory()) walk(path);
   else if (entry.isFile()) out.push({filename:relative(root,path).split("\\").join("/"), raw:readFileSync(path,"utf8")});
   else throw new Error(`unsupported input entry ${path}`);
  }
 }
 walk(resolve(root));
 return out;
}
const mainInput = files(inputDir);
// depsDir is the configured /src subtree, not a repository-wide recursive walk.
const dependencies = files(depsDir);
const configRaw = readFileSync(configPath,"utf8");
const depsPin = execFileSync("git", ["-C", depsDir, "rev-parse", "HEAD"], {encoding:"utf8"}).trim();
if (depsPin !== "d003df932d11177c98d41c32d729d40368b23fc1") throw new Error(`unexpected deps pin ${depsPin}`);
if (execFileSync("git", ["-C", depsDir, "status", "--porcelain", "--untracked-files=all"], {encoding:"utf8"}).trim()) throw new Error("deps checkout is dirty");
if (dependencies.length !== 360) throw new Error(`expected 360 deps files, found ${dependencies.length}`);
const {core, dispose} = buildUpstream(upstream);
try {
 const require = createRequire(import.meta.url);
 const {Registry} = require(core + "/registry.js");
 const {Config} = require(core + "/config.js");
 const {MemoryFile} = require(core + "/files/memory_file.js");
 const {ABAPObject} = require(core + "/objects/_abap_object.js");
 const {Lexer} = require(core + "/abap/1_lexer/lexer.js");
 const {StatementParser} = require(core + "/abap/2_statements/statement_parser.js");
 const {StructureParser} = require(core + "/abap/3_structures/structure_parser.js");
 const {StatementNode} = require(core + "/abap/nodes/statement_node.js");
 const {TokenNode} = require(core + "/abap/nodes/token_node.js");
 const toFiles = input => input.map(f => new MemoryFile(f.filename,f.raw));
 const conf = new Config(configRaw);
 const reg = new Registry(conf).addFiles(toFiles(mainInput)).addDependencies(toFiles(dependencies));
 const timing = {};
 function inventory(registry) {
  return [...registry.getObjects()].map(o => ({name:o.getName(), type:o.getType(), dependency:registry.isDependency(o),
   files:o.getFiles().map(f => f.getFilename()), xml:normalize(o.parseRaw2() ?? null), description:o.getDescription() ?? null,
   ...(o.getType() === "PROG" ? {include:o.isInclude(), modulePool:o.isModulePool()} : {})}));
 }
 const start = performance.now();
 const objects = inventory(reg);
 timing.inventory_ms = performance.now()-start;
 const parseStart = performance.now();
 reg.parse();
 timing.parse_ms = performance.now()-parseStart;
 function dump(file, tokens, statements, result) {
  const refs = new Map(statements.map((s,i) => [s,i]));
  let structures = 0;
  function node(n, depth) {
   if (n instanceof StatementNode) {
    if (!refs.has(n)) throw new Error(`unknown statement identity in ${file.getFilename()}`);
    return `\n${depth}|T${refs.get(n)}|${n.get().constructor.name}`;
   }
   structures++;
   return `\n${depth}|S${n.get().constructor.name}|${n.getChildren().length}` + n.getChildren().map(c => node(c,depth+1)).join("");
  }
  let text = `${tokens.length}|${statements.length}|${result.issues.length}`;
  for (const i of result.issues) text += `\nI|${i.getKey()}|${i.getSeverity()}|${i.getFilename()}|${i.getStart().getRow()}:${i.getStart().getCol()}|${i.getEnd().getRow()}:${i.getEnd().getCol()}|${i.getMessage()}`;
  if (result.node) text += node(result.node,0);
  return {filename:file.getFilename(), tokens:tokens.length, statements:statements.length, structures, issues:result.issues.length,
   lexerSHA256:sha(tokens.map(t => `${t.constructor.name}|${t.getStr()}|${t.getRow()}|${t.getCol()}`).join("\n")),
   statementsSHA256:sha(statementsDump(statements,TokenNode)), structuresSHA256:sha(text)};
 }
 const parsedFiles = new Map();
 for (const o of reg.getObjects()) if (o instanceof ABAPObject) {
  for (const f of o.getABAPFiles()) parsedFiles.set(f.getFilename(), f);
 }
 const dumps = [];
 const dumpStart = performance.now();
 for (const input of [...mainInput,...dependencies].filter(f => f.filename.endsWith(".abap"))) {
  const f = parsedFiles.get(input.filename);
  if (f) {
   // Re-run the structure stage on the Registry's actual expanded statements:
   // this preserves Registry/include/macro handling, not just standalone lexing.
   const result = StructureParser.run({file:f, tokens:f.getTokens(), statements:f.getStatements()});
   dumps.push({...dump(f,f.getTokens(),f.getStatements(),result), registered:true});
  } else {
   // Non-object *.abap files are ignored by Registry, but still have an oracle.
   const file = new MemoryFile(input.filename,input.raw);
   const lexed = new Lexer().run(file);
   const parsed = new StatementParser(conf.getRelease(),reg,conf.getLanguageVersion()).run([lexed], conf.getSyntaxSetttings().globalMacros)[0];
   dumps.push({...dump(file,lexed.tokens,parsed.statements,StructureParser.run(parsed)), registered:false});
  }
 }
 timing.dumps_ms = performance.now()-dumpStart;
 const resolved = {config:conf.get(), release:conf.getRelease(), language:conf.getLanguageVersion(),
  rules:conf.getEnabledRules().map(r => ({key:r.getMetadata().key, config:r.getConfig()}))};
 const negative = {};
 function capture(name, run) {
  try { negative[name] = {outcome:"return", value:normalize(run())}; }
  catch (e) { negative[name] = {outcome:"throw", error:e.name, message:e.message}; }
 }
 const small = entries => new Registry(new Config(configRaw)).addFiles(toFiles(entries));
 const f = (filename,raw) => ({filename,raw});
 capture("duplicate_default", () => inventory(small([f("a/zdup.prog.abap","REPORT zdup."),f("b/ZDUP.prog.abap","REPORT zdup.")])));
 capture("duplicate_strict", () => {
  const c = JSON.parse(configRaw); c.global.errorOnDuplicateFilenames = true;
  return inventory(new Registry(new Config(JSON.stringify(c))).addFiles(toFiles([f("a/zdup.prog.abap","REPORT zdup."),f("b/ZDUP.prog.abap","REPORT zdup.")])));
 });
 capture("missing_xml", () => inventory(small([f("zmissing.prog.abap","REPORT zmissing.")])));
 capture("malformed_xml", () => inventory(small([f("zbroken.prog.xml","<abapGit><broken></abapGit>"),f("zbroken.prog.abap","REPORT zbroken.")])));
 capture("malformed_config", () => new Config("{syntax:").get());
 capture("json5", () => new Config("{ // comment\n syntax: { version: 'v702', }, rules: {}, }").get());
 capture("case_sensitivity", () => {
  const r = small([f("ZCASE.PROG.ABAP","REPORT zcase.")]);
  r.parse(); const o = r.getFirstObject();
  return {objects:inventory(r), lowerType:r.getObject("prog","zcase")?.getName() ?? null,
   upperType:r.getObject("PROG","zcase")?.getName() ?? null,
   file:r.getFileByName("zcase.prog.abap")?.getFilename() ?? null, parsed:o.getABAPFiles().length};
 });
 capture("dependency_replaced", () => {
  const r = new Registry(new Config(configRaw)).addDependencies(toFiles([f("zreplace.prog.abap","REPORT zreplace.")]));
  r.addFile(new MemoryFile("zreplace.prog.abap","REPORT zreplace. WRITE 'main'."));
  return inventory(r);
 });
 capture("include", () => {
  const r = small([f("zmain.prog.abap","REPORT zmain. INCLUDE zinc."),f("zinc.prog.abap","WRITE 'include'."),
   f("zinc.prog.xml",'<abapGit><asx:abap><asx:values><PROGDIR><SUBC>I</SUBC></PROGDIR></asx:values></asx:abap></abapGit>')]);
  r.parse();
  return {objects:inventory(r), files:[...r.getObjects()].flatMap(o => o.getABAPFiles().map(a => ({filename:a.getFilename(),
   statements:a.getStatements().map(s => ({kind:s.get().constructor.name, text:s.concatTokens()}))})))};
 });
 const counts = {};
 for (const o of objects) counts[o.type] = (counts[o.type] ?? 0)+1;
 const manifest = {upstreamPin, depsPin, configSHA256:sha(configRaw),
  main:mainInput.map(f => ({filename:f.filename,bytes:Buffer.byteLength(f.raw),sha256:sha(f.raw)})),
  dependencies:dependencies.map(f => ({filename:f.filename,bytes:Buffer.byteLength(f.raw),sha256:sha(f.raw)})),
  types:counts, objectCount:reg.getObjectCount(), abapFiles:dumps.length, registeredABAPFiles:parsedFiles.size};
 mkdirSync(outputDir,{recursive:true});
 for (const [name,value] of Object.entries({manifest,inventory:objects,config:resolved,dumps,negative,timing})) {
  writeFileSync(join(outputDir,name+".json"),JSON.stringify(value,null,2)+"\n");
 }
 writeFileSync(join(outputDir,"checksums.json"),JSON.stringify({inventory:sha(canonical(objects)),config:sha(canonical(resolved)),dumps:sha(canonical(dumps)),negative:sha(canonical(negative))},null,2)+"\n");
 console.error(JSON.stringify({...manifest,main:manifest.main.length,dependencies:manifest.dependencies.length,timing}));
} finally { dispose(); }
