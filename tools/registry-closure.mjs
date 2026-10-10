#!/usr/bin/env node
// Materialize the complete, unmodified static closure for compiler exploration.
import {readFileSync, existsSync, mkdirSync, copyFileSync, writeFileSync, symlinkSync, readdirSync} from "node:fs";
import {join, dirname, relative, resolve} from "node:path";
import {pathToFileURL} from "node:url";
import {createRequire} from "node:module";
import {createHash} from "node:crypto";
import {verifyUpstream, upstreamPin} from "./statements-upstream.mjs";

export function materializeRegistryClosure(outArg, upstreamArg, options = {}) {
 const out = resolve(outArg), upstream = resolve(upstreamArg);
 if (existsSync(out)) throw new Error("output directory must not already exist");
 const require = createRequire(import.meta.url);
 const ts = require(verifyUpstream(upstream));
 const root = join(upstream, "packages/core");
 const roots = options.roots || ["src/registry.ts", "src/config.ts", "src/files/memory_file.ts"];
 const runtime = options.runtime;
 const seen = new Map(), external = new Set(), initialization = [];
 function walk(name) {
  if (seen.has(name)) return;
  if (name.startsWith("../") || name.startsWith("/") || !name.endsWith(".ts")) {
   throw new Error(`source escapes core or is not TypeScript: ${name}`);
  }
  const path = name.startsWith("runtime/") ? join(runtime, name.slice(8)) : join(root, name);
  const raw = readFileSync(path, "utf8");
  seen.set(name, {file:name, sha256:createHash("sha256").update(raw).digest("hex")});
  const source = ts.createSourceFile(name, raw, ts.ScriptTarget.Latest, true);
  function visit(n) {
   if ((ts.isImportDeclaration(n) || ts.isExportDeclaration(n)) && n.moduleSpecifier && ts.isStringLiteral(n.moduleSpecifier)) {
    const spec = n.moduleSpecifier.text;
    if (!spec.startsWith(".")) {
     if (runtime && (spec === "chai" || spec === "mocha")) walk(`runtime/${spec}.ts`);
     else external.add(spec);
    } else {
     const base = resolve(dirname(path), spec);
     const next = [base+".ts", join(base, "index.ts")].find(p => existsSync(p));
     if (!next) throw new Error(`unresolved relative import ${name}: ${spec}`);
     const baseDir = name.startsWith("runtime/") ? runtime : root;
     const rel = relative(baseDir, next).split("\\").join("/");
     if (rel.startsWith("../")) throw new Error(`import escapes source root: ${name}: ${spec}`);
     walk(name.startsWith("runtime/") ? "runtime/"+rel : rel);
    }
   }
   ts.forEachChild(n, visit);
  }
  visit(source);
  initialization.push(name);
 }
 for (const name of roots) walk(name);
 // mocha globals must be in the checker program even with no explicit import.
 if (runtime) for (const name of readdirSync(runtime).filter(n => n.endsWith(".ts"))) walk("runtime/"+name);
 const sources = [...seen.values()].sort((a,b) => a.file < b.file ? -1 : a.file > b.file ? 1 : 0);
 for (const source of sources) {
  const dst = join(out, source.file);
  mkdirSync(dirname(dst), {recursive:true});
  copyFileSync(source.file.startsWith("runtime/") ? join(runtime, source.file.slice(8)) : join(root, source.file), dst);
 }
 symlinkSync(join(upstream, "packages/core/node_modules"), join(out, "node_modules"), "dir");
 const compilerOptions = {module:"commonjs",target:"es2020",lib:["es2020"],noEmit:true,skipLibCheck:true,
  strictNullChecks:true,strictFunctionTypes:true,noImplicitAny:true,strictPropertyInitialization:false};
 if (runtime) Object.assign(compilerOptions, {types:[],baseUrl:".", paths:{chai:["runtime/chai.ts"],mocha:["runtime/mocha.ts"]}});
 writeFileSync(join(out, "tsconfig.json"), JSON.stringify({compilerOptions,include:runtime ? ["src/**/*.ts","test/**/*.ts","runtime/**/*.ts"] : ["src/**/*.ts"]},null,2)+"\n");
 // Preserve the original registry manifest's src-relative roots for old callers.
 const manifest = {upstreamPin,roots:options.roots || roots.map(n => n.slice(4)),external:[...external].sort(),sources};
 if (runtime) manifest.initialization = initialization;
 writeFileSync(join(out, "closure.json"), JSON.stringify(manifest,null,2)+"\n");
 console.error(JSON.stringify({upstreamPin,files:sources.length,external:manifest.external}));
 return manifest;
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
 const [out, upstream = "/home/alice/dev/abaplint"] = process.argv.slice(2);
 if (!out) throw new Error("usage: registry-closure.mjs output-dir [upstream]");
 materializeRegistryClosure(out, upstream);
}
