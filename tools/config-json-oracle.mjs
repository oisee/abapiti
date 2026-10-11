#!/usr/bin/env node
// Original pinned TypeScript, plus a proof of default rule own-property order.
import {createRequire} from "node:module";
import {readFileSync, readdirSync, writeFileSync} from "node:fs";
import {join, resolve} from "node:path";
import {buildUpstream, upstreamPin} from "./statements-upstream.mjs";

const [upstream, outputPrefix] = process.argv.slice(2);
if (!upstream || !outputPrefix) throw new Error("usage: config-json-oracle.mjs UPSTREAM OUTPUT_PREFIX");
const require = createRequire(import.meta.url);
const build = buildUpstream(upstream);
try {
  const {Config} = require(join(build.core, "config.js"));
  const {ArtifactsRules} = require(join(build.core, "artifacts_rules.js"));
  const defaultJSON = JSON.stringify(Config.getDefault().get()) + "\n";
  const ts = require(join(resolve(upstream), "node_modules/typescript"));
  const root = join(resolve(upstream), "packages/core");
  const classes = new Map();
  for (const file of readdirSync(join(root, "src/rules")).filter(f => f.endsWith(".ts"))) {
    const ast = ts.createSourceFile(file, readFileSync(join(root, "src/rules", file), "utf8"), ts.ScriptTarget.Latest, true);
    for (const c of ast.statements) {
      if (!ts.isClassDeclaration(c) || !c.name) continue;
      const parent = c.heritageClauses?.find(h => h.token === ts.SyntaxKind.ExtendsKeyword)?.types[0]?.expression.getText(ast);
      classes.set(c.name.text, {
        source: "src/rules/" + file, parent,
        constructors: c.members.filter(ts.isConstructorDeclaration).length,
        fields: c.members.filter(ts.isPropertyDeclaration)
          .filter(p => !p.modifiers?.some(m => m.kind === ts.SyntaxKind.StaticKeyword))
          .map(p => ({name: p.name.getText(ast), initialized: !!p.initializer})),
      });
    }
  }
  const proof = ArtifactsRules.getRules().map(rule => {
    const config = rule.getConfig(), name = config.constructor.name;
    const chain = [];
    for (let c = classes.get(name); c; c = classes.get(c.parent)) chain.unshift(c);
    if (!chain.length) throw new Error("configuration source not found: " + name);
    const keys = [];
    for (const c of chain) {
      if (c.constructors) throw new Error("constructor order needs review: " + name);
      for (const field of c.fields) {
        if (field.initialized && !keys.includes(field.name)) keys.push(field.name);
      }
    }
    if (JSON.stringify(keys) !== JSON.stringify(Object.keys(config))) throw new Error("own-key order mismatch: " + name);
    return {key: rule.getMetadata().key, constructor: name, source: classes.get(name).source, keys};
  });
  writeFileSync(outputPrefix + ".default.json", defaultJSON);
  writeFileSync(outputPrefix + ".orders.json", JSON.stringify({upstreamPin, configs: proof.length, explicitConstructors: 0, allDeclarationOrdersMatchV8: true, proof}, null, 2) + "\n");
  console.log(JSON.stringify({upstreamPin, configs: proof.length, bytes: Buffer.byteLength(defaultJSON), allDeclarationOrdersMatchV8: true}));
} finally {
  build.dispose();
}
