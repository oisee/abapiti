#!/usr/bin/env python3
"""Check one install set: core + registry + tests monolith OR test parts.

The monolith and parts are alternative installations of the same tests.
Local package.devc.xml is repository metadata, not a shared object name.
"""
import re
import sys
import zipfile


def check(archives):
    if len(archives) < 3:
        raise ValueError('expected CORE REGISTRY TESTS...')
    owners = {}
    forbidden = re.compile(
        rb'zcl_lnt_registry_a4h|zlnt_registry_(?:run|clean|neg)|zcl_abapiti_',
        re.IGNORECASE,
    )
    for index, archive in enumerate(archives):
        local = set()
        with zipfile.ZipFile(archive) as zipped:
            for entry in zipped.infolist():
                if entry.is_dir():
                    continue
                raw = zipped.read(entry)
                if index == 0 and forbidden.search(entry.filename.encode() + b'\n' + raw):
                    raise ValueError(f'core references registry/runtime: {entry.filename}')
                name = entry.filename.rsplit('/', 1)[-1].split('.')[0].upper()
                if not entry.filename.startswith('src/') or name == 'PACKAGE':
                    continue
                if name in owners and name not in local:
                    raise ValueError(f'duplicate object {name}: {owners[name]} and {archive}')
                local.add(name)
                owners[name] = archive
    print(f'release archives: {len(archives)} archives, {len(owners)} unique objects; core clean')


if __name__ == '__main__':
    check(sys.argv[1:])
