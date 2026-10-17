#!/usr/bin/env python3
"""Emit, build and compare TS-HG gogen against the pinned Node step-4 driver.

Usage: tools/selfhost-step4.py [--expect-ref <sha256-prefix>] <abapiti-bin> <osg> <abaplint> <transpiler> <out-dir>

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


def main(args):
    expect = None
    if '--expect-ref' in args:
        i = args.index('--expect-ref')
        if i + 1 >= len(args):
            print(__doc__, file=sys.stderr)
            return 2
        expect = args[i + 1].lower()
        args = args[:i] + args[i + 2:]
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
    ref, actual = out / 'ref.go', out / 'gogen.go'
    driver = osg / '.local/step4/step4-driver.mjs'
    original = driver.read_bytes()
    host_ts = driver.with_name('step4-host.ts')
    config_path = osg / 'tsconfig.selfhost-step4.json'
    temporary = [host_ts, config_path]
    if any(p.exists() for p in temporary):
        raise ValueError('temporary host/config files already exist in checkout')

    def heavy(command, name, cwd=osg, stdout=None, extra=None, memory='16G'):
        subprocess.run(['df', '-h', '/mnt/ssd-510'], check=True)
        subprocess.run(['grep', 'MemAvailable', '/proc/meminfo'], check=True)
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
        # The reference executes the original driver, with coverage from that
        # exact execution. Validate before adapting any source.
        with ref.open('wb') as f:
            heavy(['node', driver], 'reference', stdout=f, extra={'NODE_V8_COVERAGE': str(out / 'v8')})
        if digest(ref) == EMPTY_SHA256:
            raise RuntimeError('Node reference is empty; see reference.log')
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
        heavy([binary, 'emit-go', '--tsconfig', config_path, '--externs', extern_path,
               '--overrides', 'registry', '--trap-on-failure', '--reachability', out / 'reach.json',
               '--initialize', out / 'initialization.json',
               '-o', out / 'go', *lists], 'emit')
        # Keep the large generated package's compiler heap below the scope cap.
        heavy(['go', 'build', '-o', out / 'gogen', '.'], 'build', cwd=out / 'go',
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
        driver.write_bytes(original)
        for path in temporary:
            path.unlink(missing_ok=True)
        # Preserve logs and coverage-derived evidence, discard bulky raw V8 data.
        shutil.rmtree(out / 'v8', ignore_errors=True)
        summary = f'step4: build={build} run={run} cmp={comparison} ref={digest(ref)[:16] if ref.exists() else "none"} out={digest(actual)[:16] if actual.exists() else "none"}'
        (out / 'summary.txt').write_text(summary + '\n')
        print(summary)


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
