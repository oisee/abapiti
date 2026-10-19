"""Package the untracked synthetic root, config, oracle and runner contract."""
import hashlib
import json
import pathlib
import zipfile

root = pathlib.Path('.tmp/selfhost-m2/zlnt-check-kit')
manifest = json.loads(pathlib.Path('selfhost/m2/workload.json').read_text())
for name in ('workload.json', 'node-reference.json', 'go-probe.json', 'emission.json', 'release-comparison.json', 'excluded-tests.json', 'README.md'):
    (root / name).write_bytes(pathlib.Path('selfhost/m2', name).read_bytes())
(root / 'README.txt').write_text('Frozen emitted ZLNT M2 workload. See README.md.\n'
    'All registry files.txt entries are main files; deps.txt is empty.\n'
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
repositories = [('.', 'abapiti'), ('.tmp/selfhost-m2/abaplint', 'abaplint')]
licenses = []
for folder, label in repositories:
    for file in sorted(pathlib.Path(folder).iterdir()):
        if file.is_file() and (file.name.lower().startswith('license') or file.name.lower().startswith('copying') or file.name == 'NOTICE'):
            target = root / 'licenses' / label / file.name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(file.read_bytes())
            licenses.append(str(target.relative_to(root)))
names = sorted([file['path'] for file in manifest['files']] + licenses + [
    'abaplint.json', 'files.txt', 'deps.txt', 'expected-clean.txt', 'issues-sorted.txt',
    'README.txt', 'README.md', 'check.sh', 'workload.json', 'node-reference.json', 'go-probe.json', 'emission.json', 'release-comparison.json', 'excluded-tests.json'])
reference = json.loads(pathlib.Path('selfhost/m2/node-reference.json').read_text())
assert hashlib.sha256(pathlib.Path('selfhost/m2/workload.json').read_bytes()).hexdigest() == reference['workloadSha256']
assert hashlib.sha256((root / 'abaplint.json').read_bytes()).hexdigest() == reference['configSha256']
assert hashlib.sha256((root / 'expected-clean.txt').read_bytes()).hexdigest() == reference['expectedFile']['sha256']
assert hashlib.sha256((root / 'issues-sorted.txt').read_bytes()).hexdigest() == reference['sortedIssues']['sha256']
if 'fullResult' in reference:
    (root / 'node-reference-full.json').write_bytes(pathlib.Path(reference['fullResult']['path']).read_bytes())
    assert hashlib.sha256((root / 'node-reference-full.json').read_bytes()).hexdigest() == reference['fullResult']['sha256']
    names.append('node-reference-full.json')
probe = json.loads(pathlib.Path('selfhost/m2/go-probe.json').read_text())
if 'fullResult' in probe:
    data = pathlib.Path(probe['fullResult']['path']).read_bytes()
    assert hashlib.sha256(data).hexdigest() == probe['fullResult']['sha256']
    (root / 'go-probe-full.json').write_bytes(data)
    names.append('go-probe-full.json')
names.sort()
archive = pathlib.Path('.tmp/selfhost-m2/zlnt-check-kit.zip')
with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as z:
    for name in names:
        entry = zipfile.ZipInfo('zlnt-check-kit/' + name, date_time=(2026, 10, 14, 0, 0, 0))
        entry.compress_type = zipfile.ZIP_DEFLATED
        entry.external_attr = (0o100755 if name == 'check.sh' else 0o100644) << 16
        z.writestr(entry, (root / name).read_bytes())
with zipfile.ZipFile(archive) as z:
    assert z.testzip() is None
    for file in manifest['files']:
        assert hashlib.sha256(z.read('zlnt-check-kit/' + file['path'])).hexdigest() == file['sha256']
result = dict(path=str(archive), bytes=archive.stat().st_size,
              sha256=hashlib.sha256(archive.read_bytes()).hexdigest(), members=len(names),
              workloadFiles=len(manifest['files']), verified=True,
              verificationScope='ZIP CRCs and every workload member SHA-256',
              nativeStatus=probe.get('fullWorkload', {}).get('status'),
              byteEqualToNode=probe.get('fullWorkload', {}).get('byteEqualToNode'),
              tracked=False, variants=['clean'],
              note='Single frozen emitted-core workload; clean names the original variant, not zero issues. Host-only multi-file adapter; see README.md.')
pathlib.Path('selfhost/m2/kit.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result, indent=2))
