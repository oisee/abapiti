#!/usr/bin/env node
// Materialize the complete, unmodified static closure for compiler exploration.
import {readFileSync, existsSync, mkdirSync, copyFileSync, writeFileSync, symlinkSync} from "node:fs";
import {join, dirname, relative, resolve} from "node:path";
import {createRequire} from "node:module";
import {createHash} from "node:crypto";
import {verifyUpstream, upstreamPin} from "./statements-upstream.mjs";

const [outArg, upstreamArg = "/home/alice/dev/abaplint"] = process.argv.slice(2);
if (!outArg) throw new Error("usage: registry-closure.mjs output-dir [upstream]");
const out = resolve(outArg), upstream = resolve(upstreamArg);
if (existsSync(out)) throw new Error("output directory must not already exist");
const require = createRequire(import.meta.url);
const ts = require(verifyUpstream(upstream));
const root = join(upstream,"packages/core/src");
const roots = ["registry.ts","config.ts","files/memory_file.ts"];
const seen = new Map(), external = new Set();
function walk(name) {
 if (seen.has(name)) return;
 const path = join(root,name), raw = readFileSync(path,"utf8");
 seen.set(name,{file:"src/"+name, sha256:createHash("sha256").update(raw).digest("hex")});
 const source = ts.createSourceFile(name,raw,ts.ScriptTarget.Latest,true);
 function visit(n) {
  if ((ts.isImportDeclaration(n) || ts.isExportDeclaration(n)) && n.moduleSpecifier && ts.isStringLiteral(n.moduleSpecifier)) {
   const spec = n.moduleSpecifier.text;
   if (!spec.startsWith(".")) external.add(spec);
   else {
    const base = resolve(dirname(path),spec);
    const next = [base+".ts",join(base,"index.ts")].find(p => existsSync(p));
    if (!next) throw new Error(`unresolved relative import ${name}: ${spec}`);
    const rel = relative(root,next).split("\\").join("/");
    if (rel.startsWith("../")) throw new Error(`import escapes src: ${name}: ${spec}`);
    walk(rel);
   }
  }
  ts.forEachChild(n,visit);
 }
 visit(source);
}
for (const name of roots) walk(name);
const sources = [...seen.values()].sort((a,b) => a.file < b.file ? -1 : a.file > b.file ? 1 : 0);
for (const source of sources) {
 const dst = join(out,source.file);
 mkdirSync(dirname(dst),{recursive:true});
 copyFileSync(join(root,source.file.slice(4)),dst);
}
symlinkSync(join(upstream,"packages/core/node_modules"),join(out,"node_modules"),"dir");
writeFileSync(join(out,"tsconfig.json"),JSON.stringify({compilerOptions:{module:"commonjs",target:"es2020",lib:["es2020"],noEmit:true,skipLibCheck:true,strictNullChecks:true,strictFunctionTypes:true,noImplicitAny:true,strictPropertyInitialization:false},include:["src/**/*.ts"]},null,2)+"\n");
const manifest = {upstreamPin,roots,external:[...external].sort(),sources};
writeFileSync(join(out,"closure.json"),JSON.stringify(manifest,null,2)+"\n");
console.error(JSON.stringify({upstreamPin,files:sources.length,external:manifest.external}));
