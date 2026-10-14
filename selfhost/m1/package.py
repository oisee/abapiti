"""Package the untracked synthetic root, config, oracle and runner contract."""
import hashlib
import json
import pathlib
import zipfile

root = pathlib.Path('.tmp/selfhost/osg-check-kit')
manifest = json.loads(pathlib.Path('selfhost/m1/workload.json').read_text())
for name in ('workload.json', 'node-reference.json', 'go-probe.json', 'README.md'):
    (root / name).write_bytes(pathlib.Path('selfhost/m1', name).read_bytes())
(root / 'README.txt').write_text('Frozen OSG M1 workload. See README.md.\n'
    'All files.txt entries are main files, deps.txt is empty.\n'
    'check.sh requires the inventory host loader from go-probe.mjs;\n'
    'the ordinary zabaplint-go CLI accepts only one main file.\n')
(root / 'check.sh').write_text('''#!/bin/sh
set -eu
checker=$(realpath "${1:?usage: check.sh MULTIFILE_CHECKER}")
cd "$(dirname "$0")"
"$checker" --file files.txt --config abaplint.json --deps deps.txt -allow-read . > actual-clean.txt
cmp actual-clean.txt expected-clean.txt
echo "equal to pinned Node"
''')
(root / 'check.sh').chmod(0o755)
# Include available root licenses; this kit remains local and untracked.
repositories = [('.osg', 'osg'), ('.tmp/abaplint', 'abaplint')] + [
    (f".osg/.local/lars/{lib['folder']}", lib['folder']) for lib in manifest['libs']]
licenses = []
for folder, label in repositories:
    for file in sorted(pathlib.Path(folder).iterdir()):
        if file.is_file() and (file.name.lower().startswith('license') or file.name.lower().startswith('copying')):
            target = root / 'licenses' / label / file.name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(file.read_bytes())
            licenses.append(str(target.relative_to(root)))
names = sorted([file['path'] for file in manifest['files']] + licenses + [
    'abaplint.json', 'files.txt', 'deps.txt', 'expected-clean.txt',
    'README.txt', 'README.md', 'check.sh', 'workload.json', 'node-reference.json', 'go-probe.json'])
archive = pathlib.Path('.tmp/selfhost/osg-check-kit.zip')
with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as z:
    for name in names:
        entry = zipfile.ZipInfo('osg-check-kit/' + name, date_time=(2026, 10, 14, 0, 0, 0))
        entry.compress_type = zipfile.ZIP_DEFLATED
        entry.external_attr = (0o100755 if name == 'check.sh' else 0o100644) << 16
        z.writestr(entry, (root / name).read_bytes())
with zipfile.ZipFile(archive) as z:
    assert z.testzip() is None
    for file in manifest['files']:
        assert hashlib.sha256(z.read('osg-check-kit/' + file['path'])).hexdigest() == file['sha256']
result = dict(path=str(archive), bytes=archive.stat().st_size,
              sha256=hashlib.sha256(archive.read_bytes()).hexdigest(), members=len(names),
              workloadFiles=len(manifest['files']), verified=True,
              tracked=False, variants=['clean'],
              note='Frozen inventory only; no artificial seeded variant. Full loader contract in README.md.')
pathlib.Path('selfhost/m1/kit.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result, indent=2))
