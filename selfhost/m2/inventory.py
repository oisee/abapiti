"""Inventory every fresh core ZIP member, and compare it with v0.5.1."""
import hashlib
import json
import pathlib
import re
import zipfile

work = pathlib.Path('.tmp/selfhost-m2')
out = pathlib.Path('selfhost/m2')
kit = work / 'zlnt-check-kit'
def sha(data):
    return hashlib.sha256(data).hexdigest()
def normal(data):
    return data.decode('utf-8').removeprefix('\ufeff').replace('\r\n', '\n').replace('\r', '\n').encode('utf-8')
def load(path):
    with zipfile.ZipFile(path) as z:
        assert z.testzip() is None
        assert len(z.namelist()) == len(set(z.namelist()))
        return {i.filename: z.read(i) for i in z.infolist() if not i.is_dir()}
def write(name, obj):
    (out / name).write_text(json.dumps(obj, indent=2) + '\n')

fresh = load(work / 'a4h/zlnt.zip')
release_path = work / 'downloads/v0.5.1-core.zip'
release = load(release_path)
assert sha(release_path.read_bytes()) == 'bba0fbc463ac8aa77860ec3dfe95e6274bc816e14804ca723ce62f7ee0f1f910'
comparison = dict(
    release='v0.5.1', releaseCompilerCommit='6ed0e12230c7e491c32f04775d23f6bdff6a374e',
    releaseURL='https://github.com/oisee/abapiti/releases/download/v0.5.1/abaplint-577f875e-a4h.zip',
    releaseZipSha256=sha(release_path.read_bytes()), freshZipSha256=sha((work / 'a4h/zlnt.zip').read_bytes()),
    zipByteIdentical=(work / 'a4h/zlnt.zip').read_bytes() == release_path.read_bytes(),
    memberBytesIdentical=fresh == release,
    added=sorted(fresh.keys() - release.keys()), removed=sorted(release.keys() - fresh.keys()),
    changed=[dict(path=p, releaseSha256=sha(release[p]), freshSha256=sha(fresh[p]),
                  normalizedEqual=normal(fresh[p]) == normal(release[p]))
             for p in sorted(fresh.keys() & release.keys()) if fresh[p] != release[p]])
write('release-comparison.json', comparison)
files = []
for p, raw in sorted(fresh.items()):
    assert not pathlib.PurePosixPath(p).is_absolute() and '..' not in pathlib.PurePosixPath(p).parts
    identity = re.fullmatch(r'([^./]+)\.([a-z0-9]{4})\.(?:abap|xml)', pathlib.PurePosixPath(p).name)
    data = normal(raw)
    target = kit / p
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(data)
    files.append(dict(path=p, layer='core', objectType=identity[2].upper() if identity else None,
                      objectName=identity[1].upper() if identity else None, checkable=bool(identity),
                      sha256=sha(data), bytes=len(data)))
objects = {(f['objectType'], f['objectName']) for f in files if f['checkable']}
def counts(key):
    return {k:dict(files=len(fs), objects=len({(f['objectType'],f['objectName']) for f in fs if f['checkable']}))
            for k in sorted({f[key] for f in files if f[key]})
            for fs in [[f for f in files if f[key] == k]]}
tests = load(work / 'a4h/zlnt_tests.zip')
excluded = dict(package='$ZLNT_TESTS', zipSha256=sha((work / 'a4h/zlnt_tests.zip').read_bytes()),
                zipBytes=(work / 'a4h/zlnt_tests.zip').stat().st_size, members=len(tests),
                uncompressedBytes=sum(map(len,tests.values())),
                classes=sum(p.endswith('.clas.abap') for p in tests),
                interfaces=sum(p.endswith('.intf.abap') for p in tests),
                programs=sum(p.endswith('.prog.abap') for p in tests),
                note='Paired release build required for stable shared names/layouts; no test-package members are lint inputs.')
manifest = dict(schema='selfhost-workload/1', compilerCommit='953eae439f7fe5d39fcf6a4f4d8f4fb8d6a262ab',
    abaplintPin=dict(sha='577f875ebec44cfaf64841cfe71c8ab8dc32622e', package='@abaplint/core',version='2.120.56'),
    m1Template=dict(commit='4649b6b90a04b95ed01d1f5b777f9dab8d9ed042', branch='feat/selfhost-m1-workload'),
    normalization='UTF-8; strip one leading BOM; CRLF and CR to LF; preserve trailing newline',
    selection=dict(source='fresh tools/a4h-tests-build.mjs paired emission: zlnt.zip',package='$ZLNT',
        allArchiveMembers=True, dependencies=[],
        kitTemplate=dict(release='v0.5.1',sha256='2be51f82cfc1947416faef522c21b52c485e94f43bc6ea65d40fb8e9fb421de8'),
        registrySelection='all object .abap/.xml members including DEVC; .abapgit.xml retained as payload only',
        excludedTests=excluded),
    counts=dict(files=len(files),registryFiles=sum(f['checkable'] for f in files),objects=len(objects),
                normalizedBytes=sum(f['bytes'] for f in files),byObjectType=counts('objectType'),byLayer=counts('layer')),
    files=files)
write('workload.json',manifest)
(kit / 'files.txt').write_text(''.join(f['path']+'\n' for f in files if f['checkable']))
(kit / 'deps.txt').write_text('')
(kit / 'abaplint.json').write_bytes((out / 'abaplint.json').read_bytes())
emission=json.loads((work / 'emission.json').read_text())
emission['loweringGate']=json.loads((work / 'a4h/evidence/test-gate.json').read_text())
write('emission.json',emission)
print(json.dumps(dict(counts=manifest['counts'],excludedTests=excluded,comparison={k:v for k,v in comparison.items() if k!='changed'},changedMembers=len(comparison['changed'])),indent=2))
# The release splitter adds 21 routing helpers across seven excluded packages.
# Keep these sizes separate so the core workload and its oracle hash stay fixed.
parts_path = work / 'test-parts/parts.json'
if parts_path.exists():
    index = json.loads(parts_path.read_text())
    assert index['sourceZipSHA256'] == excluded['zipSha256']
    parts = {}
    for name, part in index['parts'].items():
        path = work / 'test-parts' / part['zip']
        members = load(path)
        assert sha(path.read_bytes()) == part['sha256']
        parts[name] = {k:part[k] for k in ('package','bytes','sha256','objectCount','dependsOn')}
        parts[name].update(members=len(members), uncompressedBytes=sum(map(len,members.values())),
                           classes=sum(p.endswith('.clas.abap') for p in members),
                           interfaces=sum(p.endswith('.intf.abap') for p in members))
    write('excluded-tests.json',dict(scope='size inventory only; excluded from M2 lint inputs',
        monolith=excluded,parts=parts,
        totalPartZipBytes=sum(p['bytes'] for p in parts.values()),
        totalPartUncompressedBytes=sum(p['uncompressedBytes'] for p in parts.values()),
        totalPartClasses=sum(p['classes'] for p in parts.values()),
        splitterHelperObjects=index['independence']['helperObjects'],
        independenceViolations=index['independence']['violations']))
