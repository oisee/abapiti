#!/usr/bin/env node
// Compile-only train bridge: use the pinned tools/gogen frontend and emitter,
// isolated registries/packages for overlapping runtime names, one Go build.
import {cpSync, mkdirSync, readFileSync, readdirSync, writeFileSync} from 'node:fs';
import {basename, join, relative, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {execFileSync, spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';

const [inputArg, outArg, osgArg] = process.argv.slice(2);
if (!inputArg || !outArg || !osgArg) throw new Error('usage: abap-fixture-compile.mjs FIXTURES OUT OSG');
const input = resolve(inputArg), out = resolve(outArg), osg = resolve(osgArg);
const pin = readFileSync(new URL('osgo.ref', import.meta.url), 'utf8').trim();
const git = (...args) => execFileSync('git', ['-C', osg, ...args], {encoding: 'utf8'}).trim();
if (git('rev-parse', 'HEAD') !== pin || git('status', '--porcelain')) throw new Error('OSG must be clean at ' + pin);
const tool = name => import(pathToFileURL(join(osg, 'tools/gogen', name)));
const {compileProgram} = await tool('frontend.mjs');
const {emitGo, referencedClasses} = await tool('emit-go.mjs');
const {unitInputs} = await tool('unit-inputs.mjs');
const config = JSON.parse(readFileSync(join(osg, 'abap_transpile.json'), 'utf8'));
const walk = dir => readdirSync(dir, {withFileTypes: true}).sort((a,b) => a.name.localeCompare(b.name))
  .flatMap(e => e.isDirectory() ? walk(join(dir, e.name)) : [join(dir, e.name)]);
const suites = dir => {
  const entries = readdirSync(dir, {withFileTypes: true});
  // Recurse even when this directory has ABAP (e.g. nested split fixtures).
  return [...(entries.some(e => /\.abap$/i.test(e.name)) ? [dir] : []),
    ...entries.filter(e => e.isDirectory()).sort((a,b) => a.name.localeCompare(b.name)).flatMap(e => suites(join(dir,e.name)))];
};
const fixtures = suites(input);
if (!fixtures.length) throw new Error('no emitted ABAP fixtures collected');
const go = join(out, 'go');
mkdirSync(out, {recursive: true});
cpSync(join(osg, 'tools/gogen/go'), go, {recursive: true, filter: src => !/(?:^|\/)zz_[^/]+\.go$/.test(src)});
const manifest = {}, packages = [], seen = new Map(), folderFiles = new Map(), contents = new Map();
const filesIn = folder => {
  if (!folderFiles.has(folder)) folderFiles.set(folder, walk(folder));
  return folderFiles.get(folder);
};
const source = file => {
  if (!contents.has(file)) contents.set(file, readFileSync(file, 'utf8'));
  return contents.get(file);
};
const objectName = file => basename(file).split('.')[0].replaceAll('#', '/').toUpperCase();
let failures = 0;
for (const [index, fixture] of fixtures.entries()) {
  const name = relative(input, fixture);
  const hash = createHash('sha256');
  const fixtureFiles = readdirSync(fixture).filter(f => /\.(abap|xml)$/i.test(f)).sort();
  for (const file of fixtureFiles) hash.update(JSON.stringify([file, source(join(fixture, file))]));
  const digest = hash.digest('hex');
  if (seen.has(digest)) {
    manifest[seen.get(digest)].push(name);
    continue;
  }
  const pkg = 'fixture' + String(index).padStart(4, '0');
  seen.set(digest, pkg);
  manifest[pkg] = [name];
  try {
    // The old LLVM fixtures are reports containing local classes, not global
    // class files. Gogen's compileProgram accepts class pools: preserve their
    // declarations/implementations in a locals include of an empty host pool.
    // Only the REPORT header is replaced; fixture bodies are not rewritten.
    let pool = fixture;
    if (fixtureFiles.some(f => /^REPORT\s/i.test(source(join(fixture, f))))) {
      pool = join(out, 'abap', pkg);
      mkdirSync(pool, {recursive: true});
      for (const file of fixtureFiles) {
        const text = source(join(fixture, file));
        if (/^REPORT\s/i.test(text)) {
          if (!/\bCLASS\s+\w+\s+DEFINITION\b/i.test(text)) throw new Error('standalone report is not a gogen class fixture: ' + file);
          writeFileSync(join(pool, 'zcl_ci_report.clas.abap'), 'CLASS zcl_ci_report DEFINITION PUBLIC CREATE PUBLIC.\nENDCLASS.\nCLASS zcl_ci_report IMPLEMENTATION.\nENDCLASS.\n');
          writeFileSync(join(pool, 'zcl_ci_report.clas.locals_imp.abap'), text.replace(/^REPORT[^\n]*\n/i, ''));
        } else writeFileSync(join(pool, file), text);
      }
    }
    const poolFiles = readdirSync(pool).filter(f => /\.(abap|xml)$/i.test(f));
    const {folders, skip: excluded} = unitInputs({home: osg, config, fixture: pool});
    const objects = poolFiles.filter(f => /\.(clas|intf)\.abap$/i.test(f)).map(objectName);
    if (!objects.length) throw new Error('fixture has no class/interface objects');
    const allFiles = folders.flatMap(filesIn).filter(f => !excluded(f));
    const sources = new Map(allFiles.filter(f => /\.clas\.abap$/i.test(f)).map(f => [objectName(f), f]));
    const byObject = new Map();
    for (const file of allFiles.filter(f => /\.(abap|xml)$/i.test(f))) {
      const key = objectName(file);
      if (!byObject.has(key)) byObject.set(key, []);
      byObject.get(key).push(file);
      // Type-pool symbols can be used without an explicit TYPE-POOLS line.
      // Their prefixed names resolve to the pool, not a same-named DDIC file.
      if (/\.type\.abap$/i.test(file)) {
        for (const match of source(file).matchAll(/[A-Z_][A-Z0-9_]*/gi)) {
          const symbol = match[0].toUpperCase();
          if (symbol.startsWith(key + '_')) byObject.set(symbol, [file]);
        }
      }
    }
    // Load the transitive lexical dependency closure, not thousands of unrelated
    // library objects for each tiny fixture. Include interfaces and DDIC/XML as
    // well as ABAP implementations; no fixture file is filtered out.
    const visible = new Set(), pending = poolFiles.map(f => join(pool, f));
    const addObject = name => { for (const f of byObject.get(name) ?? []) if (!visible.has(f)) pending.push(f); };
    while (pending.length) {
      const file = pending.pop();
      if (visible.has(file)) continue;
      visible.add(file);
      for (const match of source(file).matchAll(/[A-Z_/#][A-Z0-9_/#]*/gi)) addObject(match[0].toUpperCase());
      // f557e12's native RTTI emitter has an implicit allocator dependency
      // absent from referencedClasses (DESCRIBE_BY_DATA's reference branch).
      if (objectName(file) === 'CL_ABAP_TYPEDESCR') addObject('CL_ABAP_REFDESCR');
    }
    const skip = file => excluded(file) || !visible.has(file);
    // Match the train's native class closure. Registry-only DDIC references
    // must not turn every visible library class into a compilation target.
    const wanted = new Set(objects), queue = [...objects];
    while (queue.length) {
      const ref = queue.shift(), file = sources.get(ref);
      if (!file) continue;
      const refs = [...source(file).matchAll(/\b(?:CL|CX|ZCL|ZCX)_[A-Z0-9_]+\b/gi)].map(m => m[0].toUpperCase());
      if (ref === 'CL_ABAP_TYPEDESCR') refs.push('CL_ABAP_REFDESCR');
      for (const dep of refs) if (sources.has(dep) && !wanted.has(dep)) { wanted.add(dep); queue.push(dep); }
    }
    let program, registry;
    const session = {};
    for (let round = 0; round < 12; round++) {
      program = compileProgram({folders, objects: [...wanted], tolerant: true, includeTests: new Set(objects), skip, registry, session});
      registry = program.reg;
      const refs = new Set([...referencedClasses(program), ...program.missing]);
      if (program.classes.some(c => c.name === 'CL_ABAP_TYPEDESCR')) refs.add('CL_ABAP_REFDESCR');
      for (const ref of wanted) {
        const sup = registry.getObject('CLAS', ref)?.getDefinition()?.getSuperClass();
        if (sup) refs.add(sup.toUpperCase());
      }
      const more = [...refs].filter(ref => sources.has(ref) && !wanted.has(ref));
      if (!more.length) break;
      for (const ref of more) wanted.add(ref);
      if (round === 11) throw new Error('OSG dependency closure did not settle');
    }
    // Tolerant mode must not silently drop a syntactically broken fixture.
    // Named runtime refusals for unsupported statements remain compile-only:
    // this is not an execution/subset-coverage gate.
    if (program.diagnostics.length) throw new Error(JSON.stringify(program.diagnostics, null, 2));
    const dir = join(go, 'cmd', pkg);
    mkdirSync(dir, {recursive: true});
    writeFileSync(join(dir, 'zz_generated.go'), emitGo(program, pkg, null, true));
    packages.push('./cmd/' + pkg);
    console.log('gogen: ' + name);
  } catch (error) {
    console.error(`ABAP fixture ${name}: ${error.stack ?? error}`);
    failures++;
  }
}
writeFileSync(join(out, 'fixtures.json'), JSON.stringify(manifest, null, 2) + '\n');
if (!packages.length) throw new Error('no fixture generated Go');
const build = spawnSync('go', ['build', '-p', '2', '-trimpath', ...packages], {cwd: go, env: process.env, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024});
const log = (build.stdout ?? '') + (build.stderr ?? build.error?.message ?? '');
writeFileSync(join(out, 'build.log'), log);
console.error(log.replace(/fixture\d{4}/g, pkg => `${pkg} [${manifest[pkg]?.join(', ') ?? 'unknown fixture'}]`));
if (failures || build.status !== 0) process.exitCode = 1;
else console.log(`Compiled ${fixtures.length} emitted ABAP fixtures (${packages.length} unique programs) on OSG ${pin}`);
