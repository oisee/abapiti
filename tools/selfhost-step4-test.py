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


if __name__ == '__main__':
    unittest.main()
