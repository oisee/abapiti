#!/usr/bin/env python3
"""Small boundary tests; no Node or Go subprocesses."""
from pathlib import Path
import runpy
import tempfile
import contextlib
import hashlib
import io
import json
from types import SimpleNamespace
from unittest.mock import patch
import unittest

TOOL = runpy.run_path(str(Path(__file__).with_name('selfhost-step4.py')))


class Step4Test(unittest.TestCase):
    def test_adapter_preserves_lines_and_fails_on_driver_drift(self):
        source = "import {readFileSync} from 'node:fs';\nconst inputs = JSON.parse(readFileSync(new URL('./step4-inputs.json', import.meta.url), 'utf8'));\nprocess.stdout.write(emitGo(program));\n"
        adapted = TOOL['adapt_driver'](source)
        self.assertEqual(source.count('\n'), adapted.count('\n'))
        self.assertIn("readText('.local/step4/step4-inputs.json')", adapted)
        self.assertIn('writeStdout(emitGo(program))', adapted)
        with self.assertRaises(ValueError):
            TOOL['adapt_driver'](source.replace('process.stdout.write', 'console.log'))

    def test_first_difference_counts_bytes_and_handles_eof(self):
        with tempfile.TemporaryDirectory() as directory:
            ref, actual = Path(directory) / 'ref', Path(directory) / 'actual'
            ref.write_bytes('é\nx\n'.encode())
            for data, expected in [(b'\xc3\xa9\ny\n', 'line 2 byte 4 (column 1; ref=78, out=79)'),
                                   (b'\xc3\xa9\n', 'line 2 byte 4 (column 1; ref=78, out=EOF)'),
                                   (b'\xc3\xa9\nx\nz', 'line 3 byte 6 (column 1; ref=EOF, out=7a)')]:
                with self.subTest(data=data):
                    actual.write_bytes(data)
                    self.assertEqual(TOOL['first_difference'](ref, actual), 'differs at ' + expected)

    def test_pipeline_summary_and_restore_on_failures(self):
        main = TOOL['main']
        source = "import {readFileSync} from 'node:fs';\nconst inputs=JSON.parse(readFileSync(new URL('./step4-inputs.json', import.meta.url), 'utf8'));\nprocess.stdout.write(emitGo(program));\n"
        cases = [(f, False, None) for f in [None, 'emit', 'build', 'run']] + [(None, True, None), (None, False, 'deadbeef')]
        for failure, nosystemd, expect in cases:
            with self.subTest(failure=failure, nosystemd=nosystemd, expect=expect), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                osg, out = root / 'osg', root / 'out'
                driver = osg / '.local/step4/step4-driver.mjs'
                driver.parent.mkdir(parents=True)
                driver.write_text(source)
                (osg / 'tsconfig.gogen.json').write_text(json.dumps({'compilerOptions': {}}))
                calls = []

                def fake_run(command, **kwargs):
                    calls.append(command)
                    if command[0] == 'cmp':
                        return SimpleNamespace(returncode=int(Path(command[-2]).read_bytes() != Path(command[-1]).read_bytes()))
                    name = Path(kwargs['stderr'].name).stem
                    if nosystemd:
                        self.assertEqual(command[0], 'nice')
                    else:
                        self.assertEqual(command[:5], ['systemd-run', '--user', '--scope', '-q', '-p'])
                        self.assertEqual(command[6], 'nice')
                        self.assertEqual(command[5], 'MemoryMax=' + ('24G' if name == 'build' else '16G'))
                    if name == failure:
                        kwargs['stderr'].write(b'panic: tools/gogen/example.mjs:12:3: unsupported site\n' if name == 'run' else b'failure\n')
                        return SimpleNamespace(returncode=1)
                    if name == 'reference' or name == 'run':
                        kwargs['stdout'].write(b'package main\n')
                    elif name.endswith('-closure'):
                        (out / f'{name.removesuffix("-closure")}-initialization.json').write_text(json.dumps(['.local/step4/step4-driver.mjs'] if name == 'pilot-closure' else []))
                        kwargs['stdout'].write(b'.local/step4/step4-driver.mjs\n' if name == 'pilot-closure' else b'')
                    elif name == 'coverage':
                        (out / 'reach.json').write_text(json.dumps({'spans': []}))
                    return SimpleNamespace(returncode=0)

                capture = io.StringIO()
                args = ['--expect-ref', expect] if expect else []
                with patch.dict(TOOL['os'].environ, {'STEP4_NO_SYSTEMD': '1' if nosystemd else '0'}), patch.object(TOOL['subprocess'], 'run', side_effect=fake_run), patch.object(TOOL['shutil'], 'disk_usage', return_value=SimpleNamespace(free=100 * 1024**3)), contextlib.redirect_stdout(capture), contextlib.redirect_stderr(io.StringIO()):
                    status = main([*args, str(root / 'abapiti'), str(osg), str(root / 'lint'), str(root / 'transpiler'), str(out)])
                self.assertEqual(status, int(failure is not None or expect is not None))
                self.assertEqual(driver.read_text(), source)
                self.assertFalse(driver.with_name('step4-host.ts').exists())
                self.assertFalse((osg / 'tsconfig.selfhost-step4.json').exists())
                summary = (out / 'summary.txt').read_text()
                self.assertEqual(capture.getvalue(), summary)
                if expect is not None:
                    self.assertIn('build=fail run=not-run cmp=not-run', summary)
                elif failure is None:
                    self.assertIn('build=ok run=ok cmp=equal', summary)
                elif failure == 'run':
                    self.assertIn('run=trap at tools/gogen/example.mjs:12:3: unsupported site', summary)
                    self.assertIn('cmp=differs at line 1 byte 1', summary)
                else:
                    self.assertIn('build=fail run=not-run cmp=not-run', summary)


    def test_coverage_span_filter_honours_nested_block_ranges(self):
        covered = TOOL['covered_hits']
        text = 'function f(a) {\n  if (a) { trapA(); }\n  trapB();\n}\nfunction g() { return 1; }\n'
        f_start, g_start = text.index('function f'), text.index('function g')
        branch = text.index('{ trapA')
        functions = [
            {'functionName': '', 'ranges': [{'startOffset': 0, 'endOffset': len(text), 'count': 1}]},
            {'functionName': 'f', 'ranges': [{'startOffset': f_start, 'endOffset': g_start - 1, 'count': 3},
                                             {'startOffset': branch, 'endOffset': text.index('}', branch) + 1, 'count': 0}]},
            {'functionName': 'g', 'ranges': [{'startOffset': g_start, 'endOffset': len(text) - 1, 'count': 0}]},
        ]
        # A statement in a count-0 branch of an executed function is not executed.
        self.assertEqual(covered(functions, text.index('trapA'))[:2], (0, 'block'))
        self.assertEqual(covered(functions, text.index('trapB'))[:2], (3, 'block'))
        # A body trap takes the count of the function starting at its node and
        # gets an entry breakpoint inside that body.
        hits, evidence, probe = covered(functions, f_start, body=True, text=text)
        self.assertEqual((hits, evidence, probe), (3, 'function', text.index('{') + 1))
        self.assertEqual(covered(functions, g_start, body=True, text=text)[:2], (0, 'function'))
        # Modifiers before the V8 start still select the method; a later
        # function across a statement boundary does not.
        method = 'x();\nclass C {\n  static async m() { x(); }\n}\nfunction h() {}\n'
        m_start = method.index('m()')
        fns = [{'functionName': '', 'ranges': [{'startOffset': 0, 'endOffset': len(method), 'count': 1}]},
               {'functionName': 'm', 'ranges': [{'startOffset': m_start, 'endOffset': method.index('}') + 1, 'count': 2}]},
               {'functionName': 'h', 'ranges': [{'startOffset': method.index('function h'), 'endOffset': len(method) - 1, 'count': 0}]}]
        self.assertEqual(covered(fns, method.index('static'), body=True, text=method)[:2], (2, 'function'))
        self.assertEqual(covered(fns, method.index('class'), body=True, text=method)[:2], (1, 'block'))

    def test_utf16_positions_and_arrow_bodies(self):
        text = 'const s = "\U0001F600é";\nconst f = (a) => a + 1;\n'
        offset = TOOL['utf16_offset'](text, 2, 11)
        self.assertEqual(offset, 17 + 11 - 1)  # the emoji is two UTF-16 units
        self.assertEqual(TOOL['utf16_line_column'](text, offset), (1, 10))
        self.assertEqual(TOOL['utf16_offset'](text, 2, None), 17)
        self.assertEqual(TOOL['body_probe'](text, offset), offset + len('(a) => '))

    def test_list_traps_orders_by_replay_and_excludes_guards(self):
        main = TOOL['main']
        source = "import {readFileSync} from 'node:fs';\nconst inputs=JSON.parse(readFileSync(new URL('./step4-inputs.json', import.meta.url), 'utf8'));\nprocess.stdout.write(emitGo(program));\n"
        lib = 'function a() { if (x) { y(); } }\nfunction b() { z(); }\nfunction c() {}\n'
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            osg, out = root / 'osg', root / 'out'
            driver = osg / '.local/step4/step4-driver.mjs'
            driver.parent.mkdir(parents=True)
            driver.write_text(source)
            (osg / 'tools').mkdir()
            (osg / 'tools/lib.mjs').write_text(lib)
            (osg / 'tsconfig.gogen.json').write_text(json.dumps({'compilerOptions': {}}))
            branch = lib.index('{ y')
            sites = [
                {'Class': 'tools/lib.mjs.module', 'Method': 'c', 'Source': 'tools/lib.mjs:3:1', 'Kind': 'body', 'Message': 'lowering trap: c'},
                {'Class': 'tools/lib.mjs.module', 'Method': 'a', 'Source': 'tools/lib.mjs:1:26', 'Kind': 'stmt', 'Message': 'tools/lib.mjs:1:26'},
                {'Class': 'tools/lib.mjs.module', 'Method': 'b', 'Source': 'tools/lib.mjs:2:1', 'Kind': 'body', 'Message': 'lowering trap: unresolved method a'},
                {'Class': 'tools/lib.mjs.module', 'Method': 'a', 'Source': 'tools/lib.mjs:1:16', 'Kind': 'guard', 'Message': 'guard'},
                {'Class': 'src/x.ts.X', 'Method': 'm', 'Source': '', 'Kind': 'body', 'Message': 'opaque'},
            ]
            sites[1]['Source'] = f'tools/lib.mjs:1:{lib.index("y()") + 1}'

            def fake_run(command, **kwargs):
                name = Path(kwargs['stderr'].name).stem
                if name == 'reference':
                    kwargs['stdout'].write(b'package main\n')
                    v8 = Path(kwargs['env']['NODE_V8_COVERAGE'])
                    v8.mkdir()
                    script = {'url': (osg / 'tools/lib.mjs').as_uri(), 'functions': [
                        {'functionName': '', 'ranges': [{'startOffset': 0, 'endOffset': len(lib), 'count': 1}]},
                        {'functionName': 'a', 'ranges': [{'startOffset': 0, 'endOffset': lib.index('\n'), 'count': 2},
                                                         {'startOffset': branch, 'endOffset': lib.index('}', branch) + 1, 'count': 0}]},
                        {'functionName': 'b', 'ranges': [{'startOffset': lib.index('function b'), 'endOffset': lib.index('function c') - 1, 'count': 1}]},
                        {'functionName': 'c', 'ranges': [{'startOffset': lib.index('function c'), 'endOffset': len(lib) - 1, 'count': 4}]}]}
                    (v8 / 'coverage-1.json').write_text(json.dumps({'result': [script]}))
                elif name.endswith('-closure'):
                    (out / f'{name.removesuffix("-closure")}-initialization.json').write_text('[]')
                elif name == 'coverage':
                    (out / 'reach.json').write_text(json.dumps({'spans': []}))
                elif name == 'emit':
                    self.assertEqual(driver.read_text(), TOOL['adapt_driver'](source))
                    (out / 'go/evidence').mkdir(parents=True)
                    (out / 'go/evidence/trap-sites.json').write_text(json.dumps(sites))
                elif name == 'order':
                    self.assertEqual(driver.read_text(), source)
                    breakpoints = json.loads(Path(command[-4]).read_text())
                    self.assertEqual(sorted(b['id'] for b in breakpoints), [0, 2])
                    Path(command[-3]).write_text('2\n0\n')
                    Path(command[-2]).write_bytes(b'package main\n')
                else:
                    self.fail(f'unexpected step {name}')
                return SimpleNamespace(returncode=0)

            capture = io.StringIO()
            with patch.dict(TOOL['os'].environ, {'STEP4_NO_SYSTEMD': '1'}), patch.object(TOOL['subprocess'], 'run', side_effect=fake_run), patch.object(TOOL['shutil'], 'disk_usage', return_value=SimpleNamespace(free=100 * 1024**3)), contextlib.redirect_stdout(capture), contextlib.redirect_stderr(io.StringIO()):
                status = main(['--list-traps', str(root / 'abapiti'), str(osg), str(root / 'lint'), str(root / 'transpiler'), str(out)])
            self.assertEqual(status, 0)
            self.assertEqual(driver.read_text(), source)
            self.assertEqual(capture.getvalue(), 'step4-traps: 2 reached (1/0 unknown) 1 unreached\n')
            rows = [line.split('\t') for line in (out / 'reached-traps.tsv').read_text().splitlines()[1:]]
            self.assertEqual([(r[0], r[2], r[3]) for r in rows], [('tools/lib.mjs:2:1', '1', 'root'), ('tools/lib.mjs:3:1', '4', 'root')])
            self.assertEqual(len((out / 'reached-guards.tsv').read_text().splitlines()), 2)
            self.assertFalse((out / 'v8').exists())

if __name__ == '__main__':
    unittest.main()
