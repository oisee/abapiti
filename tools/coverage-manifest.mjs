#!/usr/bin/env node
// Builds a schema 1 reachability manifest for any TS/JS project from V8
// coverage (NODE_V8_COVERAGE). Coverage is evidence for one workload only:
// bodies it never entered become traps, so a different input fails loudly.
// usage: coverage-manifest.mjs <root> <v8-dir> <files-list> <workload> <out.json>
import {readFileSync, readdirSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';

const [rootArg, v8Dir, filesList, workload, out] = process.argv.slice(2);
if (!out) throw new Error('usage: coverage-manifest.mjs root v8-dir files-list workload out.json');
const root = resolve(rootArg);
const ts = createRequire(join(root, 'package.json'))('typescript');
const hash = text => createHash('sha256').update(text).digest('hex');

// Function ranges per file, in UTF-16 offsets, merged over all coverage dumps.
const ranges = new Map();
for (const dump of readdirSync(v8Dir).sort()) {
  for (const script of JSON.parse(readFileSync(join(v8Dir, dump), 'utf8')).result) {
    if (!script.url.startsWith('file:')) continue;
    const path = fileURLToPath(script.url);
    if (!path.startsWith(root + '/')) continue;
    const list = ranges.get(path) ?? [];
    for (const fn of script.functions) list.push(fn.ranges[0]);
    ranges.set(path, list);
  }
}

const files = readFileSync(filesList, 'utf8').split('\n').filter(Boolean);
const spans = [];
const stats = {files: files.length, loaded: 0, bodies: 0, executed: 0, unmatched: 0};
for (const rel of files) {
  const path = join(root, rel), source = readFileSync(path, 'utf8');
  const fns = ranges.get(path);
  if (fns) stats.loaded++;
  const sf = ts.createSourceFile(rel, source, ts.ScriptTarget.Latest, true);
  const visit = n => {
    if ((ts.isMethodDeclaration(n) || ts.isFunctionDeclaration(n) || ts.isConstructorDeclaration(n) || ts.isGetAccessor(n) || ts.isSetAccessor(n)) && n.body) {
      const start = n.getStart(sf), bodyStart = n.body.getStart(sf);
      // V8 starts a function at its name or keyword and ends it at the body's
      // closing brace; a class constructor may report the whole class range.
      const hits = (fns ?? []).filter(r => (r.endOffset === n.end && r.startOffset >= start && r.startOffset <= bodyStart) ||
        (ts.isConstructorDeclaration(n) && r.startOffset === n.parent.getStart(sf) && r.endOffset === n.parent.end));
      if (fns && hits.length === 0) stats.unmatched++;
      const executed = hits.some(r => r.count > 0);
      const name = ts.isConstructorDeclaration(n) ? n.parent.name?.text : n.name?.getText(sf);
      const owner = ts.isFunctionDeclaration(n) ? '' : (n.parent.name?.text ?? '') + '.';
      spans.push({
        file: rel, start: Buffer.byteLength(source.slice(0, start)), end: Buffer.byteLength(source.slice(0, n.end)),
        kind: ts.isConstructorDeclaration(n) ? 'Constructor' : ts.SyntaxKind[n.kind],
        symbol: ts.isConstructorDeclaration(n) ? name ?? '' : owner + name,
        line: sf.getLineAndCharacterOfPosition(start).line + 1, sha256: hash(source.slice(start, n.end)), executed,
      });
      stats.bodies++;
      if (executed) stats.executed++;
    }
    ts.forEachChild(n, visit);
  };
  visit(sf);
}
writeFileSync(out, JSON.stringify({schema: 1, upstreamPin: '', workloads: [workload], inputs: [], spans}, null, 1) + '\n');
console.log(JSON.stringify(stats));
