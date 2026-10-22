#!/usr/bin/env python3
"""Regenerate the embedded, byte-verified core archive from a clean upstream."""
import gzip
import hashlib
import io
import json
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile

repository = Path(__file__).resolve().parent.parent
upstream = Path(sys.argv[1]).resolve()
corpus = repository / 'tsfront/testdata/registrycorpus'
with tempfile.TemporaryDirectory(prefix='registry-archive-') as temporary:
    closure = Path(temporary) / 'closure'
    subprocess.run(['node', str(repository / 'tools/registry-closure.mjs'),
                    str(closure), str(upstream)], check=True)
    manifest = json.loads((closure / 'closure.json').read_text())
    target = corpus / f"abaplint-core-{manifest['upstreamPin'][:8]}.tar.gz"
    with target.open('wb') as raw:
        with gzip.GzipFile(fileobj=raw, mode='wb', filename='', mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode='w') as archive:
                sources = {'packages/core/' + s['file']: s['sha256'] for s in manifest['sources']}
                for name in ['LICENSE', *sources]:
                    data = (upstream / name).read_bytes()
                    if name in sources and hashlib.sha256(data).hexdigest() != sources[name]:
                        raise ValueError(f'source changed during archive generation: {name}')
                    entry = tarfile.TarInfo(name)
                    entry.size = len(data)
                    entry.mode = 0o644
                    archive.addfile(entry, io.BytesIO(data))
    (corpus / 'closure.json').write_bytes((closure / 'closure.json').read_bytes())
    print(f'{target}: {len(sources)} SHA-256 verified sources + LICENSE')
