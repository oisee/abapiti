// Adds "not executed" coverage spans for whole files whose functions V8 never ran in the
// DEPLOYMENT, NEGATIVE or OBSERVATION workloads (mapping ambiguity left them unmapped and live).
// Strict: a file qualifies only if no function in its generated JS executed in those workloads.
// usage: node tools/registry-manual-unexecuted.mjs <reachability.json> <closure-dir> <v8-dir> <upstream> src/a.ts [src/b.ts ...]
import {readFileSync, writeFileSync, readdirSync} from 'node:fs';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {join} from 'node:path';
import {verifyUpstream} from './statements-upstream.mjs';
const [manifestPath, closure, v8, upstream, ...files] = process.argv.slice(2);
const ts = createRequire(import.meta.url)(verifyUpstream(upstream));
const hash = text => createHash('sha256').update(text).digest('hex');
const executed = new Map();
for (const w of ['DEPLOYMENT', 'NEGATIVE', 'OBSERVATION']) for (const f of readdirSync(join(v8, w))) for (const s of JSON.parse(readFileSync(join(v8, w, f), 'utf8')).result) {
  const m = /\/build\/src\/(.*)\.js$/.exec(s.url);
  if (!m) continue;
  const key = 'src/' + m[1] + '.ts';
  const ran = s.functions.slice(1).filter(fn => fn.ranges[0].count > 0).length;
  executed.set(key, (executed.get(key) ?? 0) + ran);
}
const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
const have = new Set(manifest.spans.map(s => s.file + ':' + s.start));
let added = 0;
for (const file of files) {
  if (!executed.has(file)) throw new Error(`${file}: not loaded in the workloads; refuse`);
  if (executed.get(file) !== 0) throw new Error(`${file}: ${executed.get(file)} executed functions; refuse`);
  const source = readFileSync(join(closure, file), 'utf8');
  const sf = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true);
  const visit = n => {
    if ((ts.isMethodDeclaration(n) || ts.isFunctionDeclaration(n) || ts.isConstructorDeclaration(n) || ts.isGetAccessor(n) || ts.isSetAccessor(n)) && n.body) {
      const start = n.getStart(sf), end = n.end;
      const span = {file, start: Buffer.byteLength(source.slice(0, start)), end: Buffer.byteLength(source.slice(0, end)),
        kind: ts.isConstructorDeclaration(n) ? 'Constructor' : ts.SyntaxKind[n.kind],
        symbol: ts.isMethodDeclaration(n) || ts.isGetAccessor(n) || ts.isSetAccessor(n) ? n.parent.name?.text + '.' + n.name?.text : ts.isConstructorDeclaration(n) ? n.parent.name?.text : n.name?.text,
        line: sf.getLineAndCharacterOfPosition(start).line + 1, sha256: hash(source.slice(start, end)), executed: false};
      if (!have.has(file + ':' + span.start)) { manifest.spans.push(span); added++; }
    }
    ts.forEachChild(n, visit);
  };
  visit(sf);
}
manifest.spans.sort((a, b) => (a.file < b.file ? -1 : a.file > b.file ? 1 : 0) || a.start - b.start);
writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n');
console.log(JSON.stringify({files: files.length, added}));
