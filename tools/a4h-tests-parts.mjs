#!/usr/bin/env node
// Offline-only splitter. Reuses the actual abapGit archive; never emits TS again.
import {readFileSync, writeFileSync, readdirSync, mkdirSync, mkdtempSync, rmSync} from 'node:fs';
import {resolve, join, basename} from 'node:path';
import {tmpdir} from 'node:os';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {pathToFileURL} from 'node:url';

export const groups = {
 statements: ['abap/statements', 'abap', 'abap/expressions', 'abap/lexer', 'abap/nodes', 'abap/structures', 'abap/types'],
 rules_1: ['rules'], rules_2: ['rules'], syntax: ['abap/syntax'],
 objects: ['objects', 'objects/rename', 'lsp'],
 rest: ['root', 'cds', 'ddl', 'pretty_printer', 'utils'],
};
const sha = raw => createHash('sha256').update(raw).digest('hex');
const json = file => JSON.parse(readFileSync(file, 'utf8'));
const saveJSON = (file, value) => writeFileSync(file, JSON.stringify(value, null, 2) + '\n');
const nameOf = key => key.split('.')[0];

// Lexical scan, not a context whitelist: covers inheritance, interfaces, casts,
// exception/type names, static calls, and declarations. Templates recurse into
// embedded expressions; comments/literal test ABAP must not create false edges.
export function identifiers(source) {
 const tokens = [];
 const locals = new Set();
 let i = 0, previousWord = '';
 function code(templateExpression = false) {
  while (i < source.length) {
   const c = source[i];
   if (templateExpression && c === '}') { i++; return; }
   if (c === '"' || c === '*' && (i === 0 || source[i - 1] === '\n')) {
    while (i < source.length && source[i] !== '\n') i++;
   } else if (c === "'" || c === '`') {
    i++;
    while (i < source.length) {
     if (source[i++] === c) { if (source[i] === c) i++; else break; }
    }
   } else if (c === '|') {
    i++;
    while (i < source.length) {
     if (source[i] === '\\') { i += 2; continue; }
     if (source[i] === '|') { i++; break; }
     if (source[i] === '{') { i++; code(true); } else i++;
    }
   } else if (/[a-z_]/i.test(c)) {
    const start = i++;
    while (i < source.length && /[a-z0-9_]/i.test(source[i])) i++;
    const token = source.slice(start, i).toUpperCase();
    const before = source.slice(0, start).slice(-80);
    const declaration = ['DATA', 'METHODS', 'CONSTANTS', 'TYPES'].includes(previousWord);
    if (declaration) locals.add(token);
    const member = /(?:->|=>|~)\s*$/.test(before) || declaration || previousWord === 'METHOD';
    const objectContext = /\b(?:TYPE(?:\s+REF\s+TO)?|NEW|CAST|INHERITING\s+FROM|INTERFACES|RAISING|CATCH|CLASS)\s+$/i.test(before) || /^\s*=>/.test(source.slice(i));
    if (/^Z[A-Z0-9_]*$/.test(token) && !member) tokens.push({token, objectContext});
    previousWord = token;
   } else { if (c === '.') previousWord = ''; i++; }
  }
 }
 code();
 return new Set(tokens.filter(t => t.objectContext || !locals.has(t.token)).map(t => t.token));
}

export function checkIndependence(edges, assignment, core, external = new Set()) {
 const violations = [];
 for (const [from, refs] of edges) for (const to of refs) {
  if (core.has(to) || external.has(to)) continue;
  const owner = assignment.get(to);
  if (!owner || owner !== 'base' && owner !== assignment.get(from)) violations.push({from, to, fromPart: assignment.get(from), toPart: owner ?? 'missing'});
 }
 return violations;
}
export function promote(edges, assignment, core) {
 const moves = [];
 for (;;) {
  const violations = checkIndependence(edges, assignment, core);
  if (!violations.length) return moves;
  for (const v of violations) {
   if (v.toPart === 'missing') throw new Error(`unresolved Z object: ${v.from} -> ${v.to}`);
   if (assignment.get(v.to) !== 'base') {
    moves.push({...v, previousPart: assignment.get(v.to)});
    assignment.set(v.to, 'base');
   }
  }
 }
}

export function initialAssignment(objects, names, closure) {
 const membership = new Map();
 const shardGroup = new Map(Object.entries(groups).flatMap(([part, shards]) => shards.map(s => [s, part])));
 for (const shard of closure.shards) {
  if (!shardGroup.has(shard.name)) throw new Error(`unknown shard ${shard.name}`);
  for (const file of shard.initialization) {
   if (!membership.has(file)) membership.set(file, new Set());
   membership.get(file).add(shard.name);
  }
 }
 const assignment = new Map(), provenance = {};
 const expected = new Set(Object.values(groups).flat());
 if (closure.shards.length !== expected.size || new Set(closure.shards.map(s => s.name)).size !== expected.size) throw new Error('expected complete, unique 17-shard manifest');
 for (const name of objects.keys()) {
  const entry = names[name];
  const files = new Set();
  // SourceEntry.source can be file.ts:line:column or file.ts.module.
  for (const identity of [entry?.source, entry?.id, ...(entry?.declared ?? [])]) {
   for (const match of (identity ?? '').matchAll(/(?:test|runtime|src|node_modules)\/[^@:\s]+?\.ts/g)) files.add(match[0]);
  }
  const shards = new Set([...files].flatMap(f => [...(membership.get(f) ?? [])]));
  // Shared by original shards belongs to base even when those shards are grouped.
  const part = shards.size === 1 ? shardGroup.get([...shards][0]) : 'base';
  assignment.set(name, part);
  provenance[name] = {files: [...files].sort(), shards: [...shards].sort(), initialPart: part};
 }
 return {assignment, provenance};
}

// Rules is the largest original shard. Balance its indivisible root modules by
// emitted bytes; dependencies shared by the two halves are promoted afterwards.
export function balanceRules(assignment, provenance, sources) {
 const files = new Map();
 for (const [name, info] of Object.entries(provenance)) {
  if (info.shards.length !== 1 || info.shards[0] !== 'rules') continue;
  const file = info.files.find(f => f.startsWith('test/rules/'));
  if (!file || /\/_/.test(file)) { assignment.set(name, 'base'); provenance[name].initialPart = 'base'; continue; }
  if (!files.has(file)) files.set(file, {names: [], bytes: 0});
  files.get(file).names.push(name); files.get(file).bytes += Buffer.byteLength(sources.get(name));
 }
 const sizes = {rules_1: 0, rules_2: 0}, roots = {rules_1: [], rules_2: []};
 for (const [file, info] of [...files].sort((a, b) => b[1].bytes - a[1].bytes || a[0].localeCompare(b[0]))) {
  const part = sizes.rules_1 <= sizes.rules_2 ? 'rules_1' : 'rules_2';
  sizes[part] += info.bytes; roots[part].push(file);
  for (const name of info.names) { assignment.set(name, part); provenance[name].initialPart = part; }
 }
 return roots;
}

export function adaptRunner(source, objects) {
 const calls = [];
 const result = source.replace(/(METHOD host_init_\d+\.\n\s*)(z[a-z0-9_]+)=>([a-z0-9_]+)\( \)\.(\nENDMETHOD\.)/gi, (text, indent, target, method, end) => {
  if (!objects.has(target.toUpperCase())) return text;
  // Only zero-argument host_init wrappers are eligible. Other runner calls fail
  // closed through the final independence scan rather than silently changing ABI.
  calls.push({target: target.toUpperCase(), method: method.toUpperCase()});
  return `${indent}CALL METHOD ('${target.toUpperCase()}')=>('${method.toUpperCase()}').${end}`;
 });
 return {source: result, calls};
}

// Split only generated leaf bodies; retain the factory's original routing tree,
// constructor conversions, exception paths, and deletion's first-match order.
export function dispatchPlan(factory, deletion) {
 const branches = [];
 for (const m of factory.matchAll(/  METHOD ((?:new|args)_part_\d+)\.\n([\s\S]*?)  ENDMETHOD\./g)) {
  if (!m[2].trimStart().startsWith('CASE class_name.')) continue;
  for (const b of m[2].matchAll(/      WHEN `(Z[A-Z0-9_]+)`\.\n([\s\S]*?)(?=      WHEN |    ENDCASE\.)/g)) {
   branches.push({kind: 'factory', method: m[1], target: b[1], text: b[0], body: b[2]});
  }
 }
 const blocks = [...deletion.matchAll(/    IF p0 IS INSTANCE OF (z[a-z0-9_]+)\.\n[\s\S]*?    ENDIF\.\n/g)].map((m, i) => ({kind: 'delete', method: `d_${i}`, target: m[1].toUpperCase(), text: m[0]}));
 if (!branches.length || !blocks.length) throw new Error('unsupported generated dispatch layout');
 return {branches, blocks};
}
const helper = (kind, part) => `ZCL_LNT_P_${part.toUpperCase()}_${{factory: 'FAC', delete: 'DEL', chai: 'CHAI'}[kind]}`;
const ancestors = receiver => `    DATA ancestors TYPE STANDARD TABLE OF string WITH EMPTY KEY.
    DATA descr TYPE REF TO cl_abap_classdescr.
    IF ${receiver} IS BOUND.
      descr ?= cl_abap_typedescr=>describe_by_object_ref( ${receiver} ).
      WHILE descr IS BOUND.
        APPEND descr->absolute_name TO ancestors.
        CALL METHOD descr->get_super_class_type
          RECEIVING p_descr_ref = descr EXCEPTIONS super_class_not_found = 1.
        IF sy-subrc <> 0.
          CLEAR descr.
        ENDIF.
      ENDWHILE.
    ENDIF.
`;
export function chaiPlan(source) {
 const blocks = [];
 for (const m of source.matchAll(/  METHOD (entries|invoke)\.\n([\s\S]*?)  ENDMETHOD\./g)) {
  const lines = m[2].split('\n');
  for (let i = 0; i < lines.length; i++) {
   const match = lines[i].match(/^\s+IF p0->oval IS INSTANCE OF (z[a-z0-9_]+)\.$/);
   if (!match) continue;
   const start = i;
   let depth = 1;
   while (++i < lines.length && depth) {
    if (/^\s*IF\b/.test(lines[i])) depth++;
    if (/^\s*ENDIF\./.test(lines[i])) depth--;
   }
   if (depth) throw new Error('unbalanced chai dispatch block');
   i--;
   blocks.push({kind: 'chai', method: `c_${blocks.length}`, operation: m[1], target: match[1].toUpperCase(), text: lines.slice(start, i + 1).join('\n') + '\n'});
  }
 }
 if (!blocks.length) throw new Error('unsupported chai layout');
 return blocks;
}
export function adaptChai(source, blocks, assignment) {
 const helpers = new Map();
 const initialized = new Set();
 for (const b of blocks) {
  const part = assignment.get(b.target) ?? 'base', name = helper('chai', part);
  if (!helpers.has(name)) helpers.set(name, {part, declarations: [], methods: []});
  const h = helpers.get(name), returns = b.operation === 'entries';
  h.declarations.push(`    CLASS-METHODS ${b.method} IMPORTING p0 TYPE REF TO zlnt_runtime_dyna_a429568d5be8${returns ? ' RETURNING VALUE(result) TYPE zlnt_runtime_dyna_a429568d5be8=>entries_type' : ''}.`);
  h.methods.push(`  METHOD ${b.method}.\n${b.text}  ENDMETHOD.\n`);
  const setup = initialized.has(b.operation) ? '' : ancestors('p0->oval');
  initialized.add(b.operation);
  source = source.replace(b.text, `${setup}    IF line_exists( ancestors[ table_line = '\\CLASS=${b.target}' ] ).\n      CALL METHOD ('${name}')=>('${b.method.toUpperCase()}')\n        EXPORTING p0 = p0${returns ? ' RECEIVING result = result' : ''}.\n      RETURN.\n    ENDIF.\n`);
 }
 return {source, helpers: new Map([...helpers].map(([name, h]) => [name, {part: h.part, source: `CLASS ${name.toLowerCase()} DEFINITION PUBLIC CREATE PRIVATE.\n  PUBLIC SECTION.\n${h.declarations.join('\n')}\nENDCLASS.\nCLASS ${name.toLowerCase()} IMPLEMENTATION.\n${h.methods.join('')}ENDCLASS.\n` }]))};
}
const signature = (kind, method) => kind === 'delete'
 ? `${method} IMPORTING p0 TYPE REF TO object key TYPE string EXPORTING handled TYPE abap_bool result TYPE abap_bool`
 : `${method} IMPORTING class_name TYPE string p0 TYPE REF TO zlnt_runtime_clas_a39833db3295${method.startsWith('args_') ? ' p1 TYPE REF TO zlnt_runtime_arra_c22708faf2e3' : ''} RETURNING VALUE(result) TYPE REF TO object`;
export function adaptDispatch(factory, deletion, plan, assignment) {
 const helpers = new Map();
 function add(branch) {
  const part = assignment.get(branch.target);
  if (!part) throw new Error(`missing dispatch owner ${branch.target}`);
  const name = helper(branch.kind, part);
  if (!helpers.has(name)) helpers.set(name, {part, kind: branch.kind, methods: new Map()});
  const methods = helpers.get(name).methods;
  if (!methods.has(branch.method)) methods.set(branch.method, []);
  methods.get(branch.method).push(branch.text);
  return name;
 }
 for (const b of plan.branches) {
  const provider = add(b);
  factory = factory.replace(b.text, `      WHEN \`${b.target}\`.\n        CALL METHOD ('${provider}')=>('${b.method.toUpperCase()}')\n          EXPORTING class_name = class_name p0 = p0${b.method.startsWith('args_') ? ' p1 = p1' : ''}\n          RECEIVING result = result.\n`);
 }
 // RTTI ancestor membership implements the original IS INSTANCE OF conditions,
 // in precisely their original order (not regrouped by package).
 deletion = deletion.replace('    IF p0 IS INSTANCE OF', ancestors('p0') + '    IF p0 IS INSTANCE OF');
 for (const b of plan.blocks) {
  const provider = add(b);
  deletion = deletion.replace(b.text, `    IF line_exists( ancestors[ table_line = '\\CLASS=${b.target}' ] ).\n      CALL METHOD ('${provider}')=>('${b.method.toUpperCase()}')\n        EXPORTING p0 = p0 key = key IMPORTING handled = handled result = result.\n      RETURN.\n    ENDIF.\n`);
 }
 const sources = new Map();
 for (const [name, h] of helpers) {
  const declarations = [...h.methods.keys()].map(m => `    CLASS-METHODS ${signature(h.kind, m)}.`).join('\n');
  const methods = [...h.methods].map(([m, bodies]) => `  METHOD ${m}.\n${h.kind === 'factory' ? '    CASE class_name.\n' : ''}${bodies.join('')}${h.kind === 'factory' ? '    ENDCASE.\n' : ''}  ENDMETHOD.\n`).join('');
  sources.set(name, {part: h.part, source: `CLASS ${name.toLowerCase()} DEFINITION PUBLIC CREATE PRIVATE.\n  PUBLIC SECTION.\n${declarations}\nENDCLASS.\nCLASS ${name.toLowerCase()} IMPLEMENTATION.\n${methods}ENDCLASS.\n`});
 }
 return {factory, deletion, helpers: sources};
}

// Python stdlib supplies deterministic ZIP I/O, avoiding npm/vendor changes.
function unpack(archive, directory) {
 execFileSync('python3', ['-c', `import sys,zipfile,pathlib
with zipfile.ZipFile(sys.argv[1]) as z:
 for f in z.infolist():
  p=pathlib.PurePosixPath(f.filename)
  if p.is_absolute() or '..' in p.parts: raise ValueError('unsafe zip path')
 z.extractall(sys.argv[2])`, archive, directory]);
}
function pack(directory, archive) {
 execFileSync('python3', ['-c', `import sys,zipfile,pathlib
root=pathlib.Path(sys.argv[1])
with zipfile.ZipFile(sys.argv[2],'w',compression=zipfile.ZIP_DEFLATED,compresslevel=6) as z:
 for p in sorted(root.rglob('*')):
  if not p.is_file(): continue
  f=zipfile.ZipInfo(p.relative_to(root).as_posix(),(2026,1,1,0,0,0))
  f.compress_type=zipfile.ZIP_DEFLATED
  f.external_attr=0o100644<<16
  z.writestr(f,p.read_bytes())`, directory, archive]);
}
function manifestObjects(directory) {
 const objects = {};
 for (const file of readdirSync(join(directory, 'src')).sort()) {
  const key = file.split('.').slice(0, 2).join('.').toUpperCase();
  (objects[key] ??= {files: []}).files.push(`src/${file}`);
 }
 for (const o of Object.values(objects)) {
  const h = createHash('sha256');
  for (const file of o.files) {
   const raw = readFileSync(join(directory, file));
   h.update(`${Buffer.byteLength(file)}:${file}${raw.length}:`); h.update(raw);
  }
  o.sha256 = h.digest('hex');
 }
 return objects;
}
function verifyManifest(directory, manifest) {
 const actual = manifestObjects(directory);
 if (JSON.stringify(Object.keys(actual).sort()) !== JSON.stringify(Object.keys(manifest.objects).sort())) throw new Error('manifest object set mismatch');
 for (const [key, obj] of Object.entries(actual)) if (obj.sha256 !== manifest.objects[key].sha256) throw new Error(`manifest hash mismatch: ${key}`);
}
function classXML(name, template) {
 return template.replace(/<CLSNAME>[^<]+<\/CLSNAME>/, `<CLSNAME>${name}</CLSNAME>`);
}

// Recheck emitted archives, not just the in-memory graph used to assign them.
// Also usable independently after transfer (requires the original core manifest).
export function verifyParts(build, output) {
 const index = json(join(output, 'parts.json'));
 const core = new Set(Object.keys(json(join(build, 'zlnt.manifest.json')).objects).filter(k => k !== 'PACKAGE.DEVC').map(nameOf));
 const assignment = new Map(), edges = new Map(), hashes = new Map();
 const scratch = mkdtempSync(join(tmpdir(), 'a4h-parts-verify-'));
 try {
  for (const [part, info] of Object.entries(index.parts)) {
   if (basename(info.zip) !== info.zip) throw new Error('unsafe part archive path');
   const archive = join(output, info.zip), raw = readFileSync(archive);
   if (sha(raw) !== info.sha256 || raw.length !== info.bytes) throw new Error(`zip hash/size mismatch: ${part}`);
   const directory = join(scratch, part); unpack(archive, directory);
   if (!readFileSync(join(directory, '.abapgit.xml'), 'utf8').includes('<STARTING_FOLDER>/src/</STARTING_FOLDER>')) throw new Error('invalid offline repo');
   const manifest = json(join(output, info.zip.replace(/\.zip$/, '.manifest.json')));
   if (manifest.package !== info.package || Object.keys(manifest.objects).length !== info.objectCount + 1) throw new Error(`part manifest mismatch: ${part}`);
   verifyManifest(directory, manifest);
   for (const [key, object] of Object.entries(manifest.objects)) {
    if (key === 'PACKAGE.DEVC') continue;
    const name = nameOf(key);
    if (assignment.has(name) || core.has(name)) throw new Error(`duplicate object: ${name}`);
    assignment.set(name, part); hashes.set(key, object.sha256);
    edges.set(name, new Set(object.files.filter(f => f.endsWith('.abap')).flatMap(f => [...identifiers(readFileSync(join(directory, f), 'utf8'))])));
   }
  }
  const original = json(join(build, 'zlnt_tests.manifest.json'));
  const adapted = new Set(index.adaptedObjects);
  for (const [key, object] of Object.entries(original.objects)) {
   if (key === 'PACKAGE.DEVC') continue;
   if (!hashes.has(key)) throw new Error(`lost original object: ${key}`);
   if (!adapted.has(nameOf(key)) && hashes.get(key) !== object.sha256) throw new Error(`unexpected object change: ${key}`);
  }
  const violations = checkIndependence(edges, assignment, core);
  if (violations.length) throw new Error(`emitted independence failed: ${JSON.stringify(violations.slice(0, 20))}`);
  for (const [name, info] of Object.entries(index.assignments)) if (assignment.get(name) !== info.part) throw new Error(`assignment mismatch: ${name}`);
  return {checkedObjects: edges.size, violations: 0};
 } finally { rmSync(scratch, {recursive: true, force: true}); }
}

export function split(build, output) {
 build = resolve(build); output = resolve(output);
 if (output === build || output.startsWith(build + '/')) throw new Error('output must not overwrite build inputs');
 mkdirSync(output, {recursive: true});
 const scratch = mkdtempSync(join(tmpdir(), 'a4h-parts-'));
 try {
  const input = join(scratch, 'input'); unpack(join(build, 'zlnt_tests.zip'), input);
  const original = json(join(build, 'zlnt_tests.manifest.json'));
  verifyManifest(input, original);
  const core = new Set(Object.keys(json(join(build, 'zlnt.manifest.json')).objects).filter(k => k !== 'PACKAGE.DEVC').map(nameOf));
  const objects = new Map(), sources = new Map();
  for (const [key, obj] of Object.entries(original.objects)) {
   if (key === 'PACKAGE.DEVC') continue;
   const name = nameOf(key); objects.set(name, obj.files);
   sources.set(name, obj.files.filter(f => f.endsWith('.abap')).map(f => readFileSync(join(input, f), 'utf8')).join('\n'));
  }
  const {assignment, provenance} = initialAssignment(objects, json(join(build, 'names.json')), json(join(build, 'closure/closure.json')));
  const ruleRoots = balanceRules(assignment, provenance, sources);
  const runnerName = 'ZCL_LNT_TEST_RUNNER', factoryName = 'ZCL_LNT_TEST_FACTORY', deleteName = 'ZCL_LNT_TEST_DELETE';
  for (const name of [runnerName, factoryName, deleteName, 'ZCL_LNT_TEST_EXTENSION', 'ZCL_LNT_TEST_LOG', 'ZLNT_TESTS_RUN', 'ZLNT_TESTRUN']) if (objects.has(name)) assignment.set(name, 'base');
  const runner = adaptRunner(sources.get(runnerName), objects);
  sources.set(runnerName, runner.source);
  const factory = sources.get(factoryName), deletion = sources.get(deleteName), plan = dispatchPlan(factory, deletion);
  const chaiName = 'ZLNT_RUNTIME_CHAI_63495E43F558', chai = sources.get(chaiName), chaiBlocks = chaiPlan(chai);
  let neutralChai = chai;
  for (const b of chaiBlocks) neutralChai = neutralChai.replace(b.text, '');
  sources.set(chaiName, neutralChai);
  const edges = new Map([...sources].filter(([name]) => name !== factoryName && name !== deleteName).map(([name, source]) => [name, identifiers(source)]));
  // Dispatch leaf dependencies are attributed to the constructed/deleted class,
  // not to the base router. Include original public signatures/refusal paths.
  for (const b of [...plan.branches, ...plan.blocks, ...chaiBlocks]) {
   if (core.has(b.target)) continue;
   if (!edges.has(b.target)) throw new Error(`unknown dispatch target ${b.target}`);
   for (const ref of identifiers(b.text)) edges.get(b.target).add(ref);
  }
  // Retain the neutral routers' runtime dependencies as well as leaf edges.
  const neutralFactory = plan.branches.reduce((s, b) => s.replace(b.text, ''), factory);
  const neutralDelete = plan.blocks.reduce((s, b) => s.replace(b.text, ''), deletion);
  edges.set(factoryName, identifiers(neutralFactory)); edges.set(deleteName, identifiers(neutralDelete));
  const initialCounts = Object.fromEntries(['base', ...Object.keys(groups)].map(part => [part, [...assignment.values()].filter(p => p === part).length]));
  const moves = promote(edges, assignment, core);
  const adapted = adaptDispatch(factory, deletion, plan, assignment);
  sources.set(factoryName, adapted.factory); sources.set(deleteName, adapted.deletion);
  const adaptedChai = adaptChai(chai, chaiBlocks, assignment);
  sources.set(chaiName, adaptedChai.source);
  for (const [name, h] of adaptedChai.helpers) adapted.helpers.set(name, h);
  for (const [name, h] of adapted.helpers) {
   if (objects.has(name) || core.has(name)) throw new Error(`helper name collision ${name}`);
   assignment.set(name, h.part); sources.set(name, h.source);
  }
  const finalEdges = new Map([...sources].map(([name, source]) => [name, identifiers(source)]));
  const remaining = checkIndependence(finalEdges, assignment, core);
  if (remaining.length) throw new Error(`independence failed: ${JSON.stringify(remaining.slice(0, 20))}`);
  for (const [name, source] of sources) for (const line of source.split('\n')) if (Buffer.byteLength(line) > 255) throw new Error(`overlong line in ${name}`);
  const parts = {};
  for (const part of ['base', ...Object.keys(groups)]) {
   const pkg = `$ZLNT_TESTS_${part.toUpperCase()}`, stem = `zlnt_tests_${part}`;
   const directory = join(scratch, part); mkdirSync(join(directory, 'src'), {recursive: true});
   writeFileSync(join(directory, '.abapgit.xml'), readFileSync(join(input, '.abapgit.xml')));
   // Root DEVC: no PARENTCL, no requirement to create $ZLNT_TESTS.
   const devc = readFileSync(join(input, 'src/package.devc.xml'), 'utf8').replace(/<CTEXT>[^<]*<\/CTEXT>/, `<CTEXT>abaplint tests (${pkg})</CTEXT>`);
   writeFileSync(join(directory, 'src/package.devc.xml'), devc);
   for (const [name, files] of objects) if (assignment.get(name) === part) for (const file of files) {
    writeFileSync(join(directory, file), file.endsWith('.clas.abap') && [runnerName, factoryName, deleteName, chaiName].includes(name) ? sources.get(name) : readFileSync(join(input, file)));
   }
   for (const [name, h] of adapted.helpers) if (h.part === part) {
    writeFileSync(join(directory, 'src', `${name.toLowerCase()}.clas.abap`), h.source);
    writeFileSync(join(directory, 'src', `${name.toLowerCase()}.clas.xml`), classXML(name, readFileSync(join(input, 'src/zcl_lnt_test_factory.clas.xml'), 'utf8')));
   }
   const manifest = {package: pkg, algorithm: original.algorithm, objects: manifestObjects(directory)};
   saveJSON(join(output, `${stem}.manifest.json`), manifest);
   const zip = join(output, `${stem}.zip`); pack(directory, zip);
   // Independently reopen emitted bytes and recompute hashes (not staging only).
   const reopened = join(scratch, `${part}-check`); unpack(zip, reopened); verifyManifest(reopened, manifest);
   parts[part] = {package: pkg, objectCount: Object.keys(manifest.objects).length - 1, originalObjectCount: [...objects.keys()].filter(n => assignment.get(n) === part).length, zip: basename(zip), bytes: readFileSync(zip).length, sha256: sha(readFileSync(zip)), dependsOn: part === 'base' ? ['$ZLNT'] : ['$ZLNT', '$ZLNT_TESTS_BASE'], shards: groups[part] ?? [], ...(ruleRoots[part] ? {roots: ruleRoots[part].sort()} : {})};
  }
  saveJSON(join(output, 'parts.json'), {schemaVersion: 1, sourceZipSHA256: sha(readFileSync(join(build, 'zlnt_tests.zip'))), adaptedObjects: [runnerName, factoryName, deleteName, chaiName], parts, independence: {originalObjects: objects.size, helperObjects: adapted.helpers.size, initialCounts, promotedObjects: moves.length, moves, checkedObjects: finalEdges.size, referencedEdges: [...finalEdges.values()].reduce((n, refs) => n + refs.size, 0), violations: 0}, dynamicInitialization: runner.calls, assignments: Object.fromEntries([...assignment].sort().map(([name, part]) => [name, {part, ...provenance[name]}]))});
  verifyParts(build, output);
  return parts;
 } finally { rmSync(scratch, {recursive: true, force: true}); }
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
 const [build, output, mode] = process.argv.slice(2);
 if (!build || !output || mode && mode !== '--verify') throw new Error('usage: a4h-tests-parts.mjs BUILD OUTPUT [--verify]');
 console.log(JSON.stringify(mode ? verifyParts(resolve(build), resolve(output)) : split(build, output), null, 2));
}
