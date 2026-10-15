#!/usr/bin/env python3
"""Exercise checks against archive bytes, including alternate tests layouts."""
import importlib.util
from pathlib import Path
import tempfile
import zipfile

spec = importlib.util.spec_from_file_location('gate', Path(__file__).with_name('release-archives-check.py'))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

with tempfile.TemporaryDirectory() as tmp:
    paths = [str(Path(tmp) / f'{name}.zip') for name in ['core', 'registry', 'tests', 'part']]

    def write(index, name, source):
        with zipfile.ZipFile(paths[index], 'w') as zipped:
            zipped.writestr('src/package.devc.xml', '<DEVC/>')
            zipped.writestr(f'src/{name}.clas.abap', source)
            zipped.writestr(f'src/{name}.clas.xml', '<CLASS/>')

    write(0, 'zcl_lnt_core', 'CLASS zcl_lnt_core.')
    write(1, 'zcl_lnt_registry_a4h', 'CLASS zcl_lnt_registry_a4h.')
    write(2, 'zcl_lnt_test', 'CLASS zcl_lnt_test.')
    write(3, 'zcl_lnt_part', 'CLASS zcl_lnt_part.')
    gate.check(paths[:3])
    gate.check(paths)
    for index, name, source in [
        (2, 'ZCL_LNT_REGISTRY_A4H', 'duplicate'),
        (0, 'zcl_lnt_core', 'zcl_lnt_registry_a4h=>run( ).'),
        (0, 'zcl_lnt_core', 'ZCL_ABAPITI_LOG=>create( ).'),
        (0, 'zlnt_registry_neg', 'REPORT zlnt_registry_neg.'),
    ]:
        write(index, name, source)
        try:
            gate.check(paths[:3])
        except ValueError:
            pass
        else:
            raise AssertionError(f'accepted invalid archive: {name} / {source}')
        write(0, 'zcl_lnt_core', 'CLASS zcl_lnt_core.')
        write(2, 'zcl_lnt_test', 'CLASS zcl_lnt_test.')
print('release archive regression checks passed')
