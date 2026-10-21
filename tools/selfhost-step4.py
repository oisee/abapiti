#!/usr/bin/env python3
"""Emit, build and compare TS-HG gogen against the pinned Node step-4 driver.

Usage: tools/selfhost-step4.py [--inputs PATH] [--expect-ref <sha256-prefix>] [--list-traps] [--abap] <abapiti-bin> <osg> <abaplint> <transpiler> <out-dir>

--inputs stages an alternative inputs JSON for Node, replay and Go, restoring the
captured inputs on exit, and prints its Node reference SHA256. --expect-ref is optional.

--list-traps skips go build/run/cmp: from the one emission it lists every trap
site whose source lies in code the Node reference executed (V8 block coverage)
in <out-dir>/reached-traps.tsv, ordered by first execution in a breakpoint
replay of the reference, and prints `step4-traps: N reached (A/G unknown) M unreached`.

--abap (step 4b, implies --list-traps) emits ABAP with `emit-abap --keep-going` into
<out-dir>/abap instead of Go: TS-HA refusals trap whole owning bodies, ownerless
type/declaration refusals are recorded (evidence/unattributable-refusals.json) and
omitted so emission finishes, and the reached list covers the body traps together with
the lowering traps (`step4b-traps: ...`); unattributable counts and a trap breakdown
by operation/message go to <out-dir>/step4b-summary.txt.

Environment: STEP4_NO_SYSTEMD=1 (or no systemd-run on PATH) runs steps directly under the
caller's memory cap; GOCACHE defaults to <out-dir>/go-build when unset.
Requires prepared OSG node_modules, osd-libs and .local/step4 inputs. OUT must
be empty.
Each compiler, Node and Go process runs in a memory-limited scope when systemd-run
is available. Only the checkout's driver is temporarily adapted for the primitive host ABI;
its exact original bytes are restored, including on failure. No gogen edits.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

# The Node reference depends on the OSG commit's testdata, so it is computed
# from the driver on the same checkout; --expect-ref pins it for a final run.
EMPTY_SHA256 = 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'
REPO = Path(__file__).resolve().parent.parent
# Match the pilot kit's syntactic relative-import closure. Package imports are
# supplied explicitly by the abaplint and rearranger roots and tsconfig paths.
CLOSURE = r'''
import {readFileSync, writeFileSync, existsSync, statSync} from 'node:fs';
import {dirname, join, relative, resolve} from 'node:path';
import {createRequire} from 'node:module';
const root = resolve(process.argv[2]);
const ts = createRequire(join(root, 'package.json'))('typescript');
const seen = new Set(), graph = new Map();
function imports(file, text, runtime) {
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true);
  const deps = [];
  function visit(n) {
    let spec;
    if ((ts.isImportDeclaration(n) || ts.isExportDeclaration(n)) && n.moduleSpecifier && ts.isStringLiteral(n.moduleSpecifier)) spec = n.moduleSpecifier.text;
    if (!runtime && ts.isCallExpression(n) && n.expression.kind === ts.SyntaxKind.ImportKeyword && n.arguments[0] && ts.isStringLiteral(n.arguments[0])) spec = n.arguments[0].text;
    // A lazy require("<relative literal>") (gogen requires through frontend-base.mjs's
    // createRequire) is lowered like a dynamic import: in the closure, not in the
    // eager initialization graph, because Node loads it at the call.
    if (!runtime && ts.isCallExpression(n) && ts.isIdentifier(n.expression) && n.expression.text === 'require' && n.arguments.length === 1 && ts.isStringLiteral(n.arguments[0])) spec = n.arguments[0].text;
    if (spec?.startsWith('.')) {
      const base = resolve(dirname(file), spec);
      const found = [base, base.replace(/\.js$/, '.ts'), base + '.ts', base + '/index.ts'].find(f => existsSync(f) && statSync(f).isFile());
      if (!found) throw new Error(`unresolved import ${spec} in ${file}`);
      deps.push(found);
    }
    ts.forEachChild(n, visit);
  }
  visit(source);
  return deps;
}
function collect(file) {
  if (seen.has(file)) return;
  seen.add(file);
  const text = readFileSync(file, 'utf8');
  const runtime = ts.transpileModule(text, {compilerOptions: {target: ts.ScriptTarget.ESNext, module: ts.ModuleKind.ESNext}, fileName: file}).outputText;
  graph.set(file, imports(file, runtime, true));
  for (const dep of imports(file, text, false)) collect(dep);
}
const roots = process.argv.slice(3).map(f => resolve(root, f));
for (const file of roots) collect(file);
const initialized = new Set(), order = [];
function initialize(file) {
  if (initialized.has(file)) return;
  initialized.add(file);
  for (const dep of graph.get(file) ?? []) initialize(dep);
  order.push(relative(root, file));
}
for (const file of roots) initialize(file);
writeFileSync(process.env.STEP4_INIT_OUT, JSON.stringify(order, null, 1) + '\n');
console.log([...seen].map(f => relative(root, f)).sort().join('\n'));
'''


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def first_difference(ref, actual):
    a, b = ref.read_bytes(), actual.read_bytes()
    pos = next((i for i, (x, y) in enumerate(zip(a, b)) if x != y), min(len(a), len(b)))
    line = a[:pos].count(b'\n') + 1
    col = pos - a.rfind(b'\n', 0, pos)
    return f'differs at line {line} byte {pos + 1} (column {col}; ref={a[pos:pos+1].hex() or "EOF"}, out={b[pos:pos+1].hex() or "EOF"})'


def adapt_driver(source):
    changes = {
        "import {readFileSync} from 'node:fs';": "import {readText, writeStdout} from './step4-host.ts';",
        "readFileSync(new URL('./step4-inputs.json', import.meta.url), 'utf8')": "readText('.local/step4/step4-inputs.json')",
        'process.stdout.write(': 'writeStdout(',
    }
    for before, after in changes.items():
        if source.count(before) != 1:
            raise ValueError(f'driver host adapter expected exactly one {before!r}')
        source = source.replace(before, after)
    return source


SOURCE = re.compile(r'^(.+?):(\d+)(?::(\d+))?$')
LOCATED = re.compile(r'([^\s;()]+\.(?:ts|mjs|js)):(\d+)(?::(\d+))?')


def site_location(site):
    """Return (file, line, column-or-None) of a trap site, preferring its node."""
    for text in (site.get('Source', ''), site.get('Location', ''), site.get('Message', '')):
        m = SOURCE.match(text or '') or LOCATED.search(text or '')
        if m and re.search(r'\.(?:ts|mjs|js|tsx|cjs)$', m.group(1)):
            return m.group(1), int(m.group(2)), int(m.group(3)) if m.group(3) else None
    return None


def utf16_offset(text, line, column):
    """1-based line and UTF-16 column (None: first non-blank) to a UTF-16 offset."""
    lines = text.split('\n')
    if line < 1 or line > len(lines):
        return None
    before = sum(len(l.encode('utf-16-le')) // 2 + 1 for l in lines[:line - 1])
    current = lines[line - 1]
    if column is None:
        column = len(current) - len(current.lstrip()) + 1
    return before + column - 1


def utf16_line_column(text, offset):
    """UTF-16 offset to the 0-based line and column V8 breakpoints take."""
    units = text.encode('utf-16-le')[:offset * 2].decode('utf-16-le', errors='ignore')
    return units.count('\n'), len(units) - units.rfind('\n') - 1


def body_probe(text, start):
    """UTF-16 offset just inside the body of the function that starts at start."""
    units = text.encode('utf-16-le')
    chars = [units[i:i + 2].decode('utf-16-le', errors='replace') for i in range(0, len(units), 2)]
    depth = 0
    for i in range(start, min(len(chars), start + 4096)):
        c = chars[i]
        if c in '([':
            depth += 1
        elif c in ')]':
            depth -= 1
        elif depth == 0 and c == '{':
            return i + 1
        elif depth == 0 and c == '=' and i + 1 < len(chars) and chars[i + 1] == '>':
            j = i + 2
            while j < len(chars) and chars[j].isspace():
                j += 1
            return j
    return start


def covered_hits(functions, offset, body=False, text=None, owner_offset=None):
    """Execution count of the code at a UTF-16 offset in one V8 script.

    Block coverage nests: the innermost range containing the offset decides,
    so a count-0 branch inside an executed function is not executed. A body
    trap fires on entry, so it takes the count of the function V8 starts at
    or just after MethodSource (keyword, modifiers and name come first).
    That owner's range must also contain the trap node; an inner expression
    can belong to an uncalled nested arrow without changing the owner count.
    Returns (count, evidence, probe offset for an entry breakpoint).
    """
    if body:
        best = None
        start = owner_offset if owner_offset is not None else offset
        script_length = len(text.encode('utf-16-le')) // 2 if text is not None else None
        for fn in functions:
            r = fn['ranges'][0]
            # V8 also records the module itself as an unnamed function. It
            # is not the HIR method, even when its declaration starts at 0.
            if owner_offset is not None and not fn.get('functionName') and r['startOffset'] == 0 and r['endOffset'] == script_length:
                continue
            gap = r['startOffset'] - start
            # Source may itself be the declaration, including an export or
            # modifier prefix that V8 excludes from the function range.
            if owner_offset is not None and not start <= offset < r['endOffset']:
                continue
            # Nearest start wins; at equal starts the innermost (shortest) range.
            if 0 <= gap < 512 and r['endOffset'] > r['startOffset'] and (best is None or (r['startOffset'], r['endOffset']) < (best['startOffset'], best['endOffset'])):
                if text is None or not re.search(r'[{};]', text.encode('utf-16-le')[start * 2:r['startOffset'] * 2].decode('utf-16-le', errors='ignore')):
                    best = r
        if best is not None:
            return best['count'], 'function', body_probe(text, best['startOffset']) if text is not None else best['startOffset']
        if owner_offset is not None:
            return None, 'owner-unmapped', None
    inner = None
    for fn in functions:
        for r in fn['ranges']:
            if r['startOffset'] <= offset < r['endOffset'] and (inner is None or r['endOffset'] - r['startOffset'] <= inner['endOffset'] - inner['startOffset']):
                inner = r
    return (inner['count'] if inner else 0), 'block', offset


def load_coverage(v8):
    """Merge V8 coverage dumps into {absolute path: [functions]}."""
    from urllib.parse import unquote, urlparse
    scripts = {}
    for dump in sorted(Path(v8).glob('*.json')):
        for script in json.loads(dump.read_text())['result']:
            if script['url'].startswith('file:'):
                scripts.setdefault(unquote(urlparse(script['url']).path), []).extend(script['functions'])
    return scripts


def compiled_path(path, lint, transpiler):
    """The node_modules build file Node executes for an abaplint/transpiler source."""
    for root, package in ((lint / 'packages/core', '@abaplint/core'), (transpiler / 'packages/transpiler', '@abaplint/transpiler')):
        try:
            rel = path.relative_to(root)
        except ValueError:
            continue
        if rel.parts and rel.parts[0] == 'src':
            return package, rel.with_suffix('.js')
    return None


# A member that is unresolved because its own definition was refused (a
# trapped method or module function of that name) is a follow-on, not a root.
CASCADE = re.compile(r'unresolved (?:field|method) (\w+)')


def classify_traps(sites, osg, lint, transpiler, scripts):
    """Attach coverage evidence to trap sites; returns (rows, unknown_a, unknown_g)."""
    texts, rows, unknown = {}, [], {'A': 0, 'G': 0}
    trapped = {s['Method'] for s in sites if s.get('Kind') == 'body' and 'lowering trap' in s.get('Message', '')}
    for site in sites:
        loc = site_location(site)
        side = 'A' if loc is None and not site.get('Class', '').startswith('tools/') else 'G'
        row = dict(site, file=None, line=0, col=0, hits=None, evidence='none', url=None, probe=None)
        if loc:
            path = (osg / loc[0]).resolve()
            row.update(file=os.path.relpath(path, osg), line=loc[1], col=loc[2] or 0)
            compiled = compiled_path(path, lint, transpiler)
            side = 'A' if compiled or not str(path).startswith(str(osg) + os.sep) else 'G'
            if compiled:
                build = osg / 'node_modules' / compiled[0] / 'build' / compiled[1]
                functions = scripts.get(str(build.resolve())) or scripts.get(str(build))
                name = site['Method'] if site['Method'] != 'constructor' else site['Class'].rsplit('.', 1)[-1]
                matches = [fn for fn in functions or [] if fn.get('functionName') == name]
                if site.get('Kind') == 'body' and matches:
                    best = max(matches, key=lambda fn: fn['ranges'][0]['count'])
                    if build not in texts:
                        texts[build] = build.read_text(errors='replace') if build.exists() else ''
                    row.update(hits=best['ranges'][0]['count'], evidence='name', url=build.resolve().as_uri(),
                               probe=body_probe(texts[build], best['ranges'][0]['startOffset']) if texts[build] else None)
                    if row['probe'] is not None:
                        row['text'] = build
            elif path.exists():
                functions = scripts.get(str(path))
                if path not in texts:
                    texts[path] = path.read_text(errors='replace')
                offset = utf16_offset(texts[path], loc[1], loc[2])
                if functions is None:
                    row.update(hits=0, evidence='unloaded')
                elif offset is not None:
                    # TrapSites writes m.Source as MethodSource, separately
                    # from the refused expression's Source. Class/Method own
                    # this location even for anonymous lifted closures.
                    owner = site_location({'Source': site.get('MethodSource', '')}) if site.get('Kind') == 'body' else None
                    owner_offset = utf16_offset(texts[path], owner[1], owner[2]) if owner and (osg / owner[0]).resolve() == path else None
                    hits, evidence, probe = covered_hits(functions, offset, site.get('Kind') == 'body', texts[path], owner_offset)
                    row.update(hits=hits, evidence=evidence, url=path.as_uri(), probe=probe, text=path)
        if row['hits'] is None and site.get('Kind') != 'guard':
            unknown[side] += 1
        message = site.get('Message', '')
        m = CASCADE.search(message)
        row['root'] = 'cascade' if m and m.group(1) in trapped - {site['Method']} else 'root'
        if row.get('text') is not None and row.get('probe') is not None:
            row['bp'] = utf16_line_column(texts[row['text']], row['probe'])
        rows.append(row)
    return rows, unknown['A'], unknown['G']


def order_rows(rows, hits_order):
    """Reached rows by first execution; rows the replay missed follow by file:line:col."""
    rank = {i: n for n, i in enumerate(hits_order)}
    return sorted(rows, key=lambda r: (rank.get(r['index'], len(rank)), r['file'] or '', r['line'], r['col']))


def write_traps(out, rows, unknown_a, unknown_g):
    """Write reached traps; runtime-check guards go to reached-guards.tsv and are not counted."""
    certain = [r for r in rows if r.get('Kind') != 'guard']
    reached = [r for r in certain if r['hits']]
    unreached = sum(1 for r in certain if r['hits'] == 0)
    for name, selected in (('reached-traps.tsv', reached), ('reached-guards.tsv', [r for r in rows if r.get('Kind') == 'guard' and r['hits']])):
        with (out / name).open('w') as f:
            f.write('location\tkind: message\thits\troot-or-cascade\tmethod\tevidence\n')
            for r in selected:
                location = f"{r['file']}:{r['line']}:{r['col']}"
                message = ' '.join(r.get('Message', '').split())
                f.write(f"{location}\t{r.get('Kind', 'body')}: {message}\t{r['hits']}\t{r['root']}\t{r['Class']}.{r['Method']}\t{r['evidence']}\n")
    return f'step4-traps: {len(reached)} reached ({unknown_a}/{unknown_g} unknown) {unreached} unreached'


OPERATION = re.compile(r'runtime operation ([A-Za-z0-9_.]+)')


def trap_breakdown(sites, limit=30):
    """Count trap sites by operation when the message names one, else by message."""
    counts = {}
    for site in sites:
        message = ' '.join(site.get('Message', '').split())
        m = OPERATION.search(message)
        key = m.group(1) if m else (message.split(';')[0][:120] or '<empty>')
        counts[key] = counts.get(key, 0) + 1
    return sorted(counts.items(), key=lambda kv: (-kv[1], kv[0]))[:limit], len(counts)


def write_step4b_summary(out, sites):
    """Record keep-going evidence: unattributable refusals and a trap breakdown."""
    lines = []
    path = out / 'abap/evidence/unattributable-refusals.json'
    if path.exists():
        rows = json.loads(path.read_text()) or []
        by_op = {}
        for row in rows:
            by_op[row.get('operation') or '<none>'] = by_op.get(row.get('operation') or '<none>', 0) + 1
        lines.append(f'unattributable refusals: {len(rows)}')
        for op, n in sorted(by_op.items(), key=lambda kv: (-kv[1], kv[0])):
            lines.append(f'  {op}: {n}')
        for row in rows[:10]:
            lines.append(f"  node {row.get('node')} ({row.get('location')}): {row.get('message')}")
    else:
        lines.append('unattributable refusals: none recorded (no --keep-going evidence)')
    top, distinct = trap_breakdown(sites)
    lines.append(f'trap sites: {len(sites)} in {distinct} operations/messages (top {len(top)})')
    for op, n in top:
        lines.append(f'  {op}: {n}')
    (out / 'step4b-summary.txt').write_text('\n'.join(lines) + '\n')
    return f'unattributable: {len(rows) if path.exists() else "n/a"}'


def main(args):
    inputs = None
    if '--inputs' in args:
        i = args.index('--inputs')
        if i + 1 >= len(args):
            print(__doc__, file=sys.stderr)
            return 2
        inputs = Path(args[i + 1]).resolve().read_bytes()
        json.loads(inputs)  # Reject malformed JSON before changing the checkout.
        args = args[:i] + args[i + 2:]
    expect = None
    if '--expect-ref' in args:
        i = args.index('--expect-ref')
        if i + 1 >= len(args):
            print(__doc__, file=sys.stderr)
            return 2
        expect = args[i + 1].lower()
        args = args[:i] + args[i + 2:]
    abap = '--abap' in args
    list_traps = abap or '--list-traps' in args
    args = [a for a in args if a not in ('--list-traps', '--abap')]
    if len(args) != 5:
        print(__doc__, file=sys.stderr)
        return 2
    binary, osg, lint, transpiler, out = map(lambda x: Path(x).resolve(), args)
    if out.exists() and any(out.iterdir()):
        raise ValueError(f'output directory must be empty: {out}')
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, GOTOOLCHAIN='go1.26.0', GOFLAGS='-buildvcs=false', GOWORK='off')
    env.pop('NODE_V8_COVERAGE', None)
    if not env.get('GOCACHE'):
        env['GOCACHE'] = str(out / 'go-build')
    env.setdefault('TMPDIR', str(out))
    build, run, comparison = 'fail', 'not-run', 'not-run'
    traps = None
    ref, actual = out / 'ref.go', out / 'gogen.go'
    driver = osg / '.local/step4/step4-driver.mjs'
    original = driver.read_bytes()
    inputs_path = driver.with_name('step4-inputs.json')
    original_inputs = inputs_path.read_bytes() if inputs is not None and inputs_path.exists() else None
    host_ts = driver.with_name('step4-host.ts')
    config_path = osg / 'tsconfig.selfhost-step4.json'
    temporary = [host_ts, config_path]
    if any(p.exists() for p in temporary):
        raise ValueError('temporary host/config files already exist in checkout')

    def heavy(command, name, cwd=osg, stdout=None, extra=None, memory='16G'):
        available = next(int(line.split()[1]) for line in Path('/proc/meminfo').read_text().splitlines()
                         if line.startswith('MemAvailable:'))
        if available <= 30 * 1024**2:
            raise RuntimeError('memory available at or below 30G; stopping')
        usage = shutil.disk_usage(out)
        if usage.free < 40 * 1024**3:
            raise RuntimeError('disk free below 40G; stopping')
        with (out / f'{name}.log').open('wb') as log:
            # Containers without systemd rely on their own memory cap.
            wrap = ['nice'] if os.environ.get('STEP4_NO_SYSTEMD') == '1' or not shutil.which('systemd-run') else \
                ['systemd-run', '--user', '--scope', '-q', '-p', f'MemoryMax={memory}', 'nice']
            result = subprocess.run(
                [*wrap, *map(str, command)],
                cwd=cwd, env=dict(env, **(extra or {})), stdout=stdout or log, stderr=log)
        if result.returncode:
            raise RuntimeError(f'{name} failed (exit {result.returncode}); see {out / (name + ".log")}')

    try:
        if inputs is not None:
            (out / 'inputs.json').write_bytes(inputs)
            inputs_path.write_bytes(inputs)
        # The reference executes the original driver, with coverage from that
        # exact execution. Validate before adapting any source.
        with ref.open('wb') as f:
            heavy(['node', driver], 'reference', stdout=f, extra={'NODE_V8_COVERAGE': str(out / 'v8')})
        if digest(ref) == EMPTY_SHA256:
            raise RuntimeError('Node reference is empty; see reference.log')
        if inputs is not None:
            print(f'step4-reference: sha256={digest(ref)}')
        if expect and not digest(ref).startswith(expect):
            raise RuntimeError(f'reference SHA256 mismatch: {digest(ref)}; expected {expect}')
        (out / 'closure.mjs').write_text(CLOSURE)
        lists = []
        for name, roots in [('pilot', [driver]), ('core', [lint / 'packages/core/src/index.ts', transpiler / 'packages/transpiler/src/rearranger.ts'])]:
            path = out / f'{name}-files.rel'
            with path.open('wb') as f:
                heavy(['node', out / 'closure.mjs', osg, *roots], f'{name}-closure', stdout=f,
                      extra={'STEP4_INIT_OUT': str(out / f'{name}-initialization.json')})
            lists.extend(path.read_text().splitlines())
        # Package roots loaded through gogen's reviewed createRequire adapter
        # precede the relative-import driver graph. Each graph uses JS imports
        # after TypeScript erases type-only dependencies; cycles run once.
        order = json.loads((out / 'core-initialization.json').read_text()) + json.loads((out / 'pilot-initialization.json').read_text())
        (out / 'initialization.json').write_text(json.dumps(list(dict.fromkeys(order)), indent=1) + '\n')
        heavy(['node', REPO / 'tools/coverage-manifest.mjs', osg, out / 'v8', out / 'pilot-files.rel', 'step4', out / 'reach.json'], 'coverage')
        # Extern declarations are removed from the translated closure before
        # coverage validation. Their Node bodies are outside the Go program.
        reach_path = out / 'reach.json'
        reach = json.loads(reach_path.read_text())
        reach['spans'] = [s for s in reach['spans'] if s['file'] != 'tools/gogen/host-io.ts']
        reach_path.write_text(json.dumps(reach, indent=1) + '\n')
        driver.write_text(adapt_driver(original.decode()))
        (out / 'adapted-driver.mjs').write_bytes(driver.read_bytes())
        host_ts.write_text('export declare function readText(file: string): string;\nexport declare function writeStdout(text: string): void;\n')
        lists.append(str(host_ts.relative_to(osg)))
        lists = sorted(set(lists))
        config = json.loads((osg / 'tsconfig.gogen.json').read_text())
        rearranger = str(transpiler / 'packages/transpiler/src/rearranger.ts')
        config['compilerOptions'].update({
            'types': ['node'], 'allowJs': True, 'checkJs': False, 'baseUrl': '.',
            'paths': {'@abaplint/core': [str(lint / 'packages/core/src/index.ts')],
                      '@abaplint/transpiler/build/src/rearranger': [rearranger],
                      '@abaplint/transpiler/build/src/rearranger.js': [rearranger]}})
        config['include'] = lists
        config_path.write_text(json.dumps(config, indent=2) + '\n')
        (out / 'tsconfig.json').write_bytes(config_path.read_bytes())
        (out / 'files.rel').write_text('\n'.join(lists) + '\n')
        host = out / 'host'
        host.mkdir()
        host_import = 'step4.local/host'
        hostio_import = 'github.com/oisee/abapiti/extern/hostio'
        (host / 'go.mod').write_text(f'module {host_import}\n\ngo 1.26.0\n\nrequire {hostio_import} v0.0.0\n')
        (host / 'host.go').write_text('package host\nimport ("os"; "io"; hostio "' + hostio_import + '")\nfunc ReadText(file string) (string,error) { return hostio.ReadText(file) }\nfunc WriteStdout(text string) error { _,err:=io.WriteString(os.Stdout,text);return err }\n')
        externs = {'modules': [
            {'file': 'tools/gogen/host-io.ts', 'goImport': hostio_import, 'goDir': str(REPO / 'extern/hostio')},
            {'file': str(host_ts.relative_to(osg)), 'goImport': host_import, 'goDir': str(host)}]}
        extern_path = out / 'externs.json'
        extern_path.write_text(json.dumps(externs, indent=2) + '\n')
        # The driver's only edits are host IO. Keep all other unsupported sites
        # in the emitted program as located traps, using the existing policy.
        if abap:
            heavy([binary, 'emit-abap', '--keep-going', '--tsconfig', config_path, '--externs', extern_path,
                   '--overrides', 'registry', '--reachability', out / 'reach.json',
                   '-o', out / 'abap', *lists], 'emit')
        else:
            heavy([binary, 'emit-go', '--tsconfig', config_path, '--externs', extern_path,
                   '--overrides', 'registry', '--trap-on-failure', '--reachability', out / 'reach.json',
                   '--initialize', out / 'initialization.json',
                   '-o', out / 'go', *lists], 'emit')
        if list_traps:
            # The replay runs the original driver, like the reference.
            driver.write_bytes(original)
            for path in temporary:
                path.unlink(missing_ok=True)
            sites_path = out / ('abap' if abap else 'go') / 'evidence/trap-sites.json'
            if sites_path.exists():
                sites = json.loads(sites_path.read_text()) or []
            else:  # older abapiti: whole-body traps only
                sites = [dict(s, Kind='body', Source='', Message=s['Location'])
                         for s in json.loads((out / 'go/evidence/registry-traps.json').read_text()) or []]
            rows, unknown_a, unknown_g = classify_traps(sites, osg, lint, transpiler, load_coverage(out / 'v8'))
            for n, row in enumerate(rows):
                row['index'] = n
            breakpoints = [{'id': r['index'], 'url': r['url'], 'line': r['bp'][0], 'column': r['bp'][1]}
                           for r in rows if r['hits'] and r.get('bp') and r.get('Kind') != 'guard']
            (out / 'order-breakpoints.json').write_text(json.dumps(breakpoints) + '\n')
            hits = out / 'order-hits.txt'
            hits.write_text('')
            heavy(['node', REPO / 'tools/selfhost-step4-order.mjs', out / 'order-breakpoints.json', hits,
                   out / 'order-ref.go', driver], 'order')
            if digest(out / 'order-ref.go') != digest(ref):
                raise RuntimeError('breakpoint replay output differs from the reference')
            order = [int(x) for x in hits.read_text().split()]
            traps = write_traps(out, order_rows(rows, order), unknown_a, unknown_g)
            if abap:
                traps = traps.replace('step4-traps:', 'step4b-traps:', 1) + '; ' + write_step4b_summary(out, sites)
            return 0
        # Keep the large generated package's compiler heap below the scope cap.
        heavy(['go', 'build', '-o', out / 'gogen', '.'], 'build', cwd=out / 'go', memory='24G',
              extra={'GOMEMLIMIT': env.get('GOMEMLIMIT', '12GiB'), 'GOGC': env.get('GOGC', '50')})
        build = 'ok'
        with actual.open('wb') as f:
            try:
                heavy([out / 'gogen'], 'run', stdout=f)
                run = 'ok'
            except RuntimeError:
                log = (out / 'run.log').read_text(errors='replace')
                location = re.search(r'([^\s;()]+\.(?:ts|mjs|js):\d+:\d+[^\n]*)', log)
                reason = location.group(1).strip() if location else log.splitlines()[0] if log else 'unknown (see run.log)'
                run = 'trap at ' + reason.split(';', 1)[0]
        # A clean exit without output must not read as success when Node
        # printed something (dropped statements once looked exactly so).
        if run == 'ok' and actual.stat().st_size == 0 and ref.stat().st_size > 0:
            run = 'ok-empty'
        cmp_result = subprocess.run(['cmp', '-s', str(ref), str(actual)])
        if cmp_result.returncode > 1:
            raise RuntimeError(f'cmp failed (exit {cmp_result.returncode})')
        comparison = 'equal' if cmp_result.returncode == 0 else first_difference(ref, actual)
        return 0 if run == 'ok' and comparison == 'equal' else 1
    except (RuntimeError, OSError, ValueError) as error:
        print(str(error), file=sys.stderr)
        return 1
    finally:
        if inputs is not None:
            if original_inputs is None:
                inputs_path.unlink(missing_ok=True)
            else:
                inputs_path.write_bytes(original_inputs)
        driver.write_bytes(original)
        for path in temporary:
            path.unlink(missing_ok=True)
        # Preserve logs and coverage-derived evidence, discard bulky raw V8 data.
        shutil.rmtree(out / 'v8', ignore_errors=True)
        summary = f'step4: build={build} run={run} cmp={comparison} ref={digest(ref)[:16] if ref.exists() else "none"} out={digest(actual)[:16] if actual.exists() else "none"}'
        if list_traps:
            summary = traps or 'step4-traps: failed before listing'
        (out / 'summary.txt').write_text(summary + '\n')
        print(summary)


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
