#!/usr/bin/env python3
"""Prove generated Go trees differ only by consistent layout-derived names.

Usage: python3 -I tools/layout-rename-check.py BASE TRAIN OUT
OUT retains the inferred bijection, renamed base sources, and any real diffs.
No whitespace, ordering, literals, or source locations are normalized away.
"""
import argparse
import difflib
import json
from pathlib import Path
import re
import subprocess
import sys

# HIR-generated Go names and the structural identities embedded in runtime
# metadata. Every replacement is simultaneous, whole-token, and bijective.
NAME = re.compile(rb'\bz_[a-z0-9_]{1,12}_[0-9a-f]{14}\b|\b(?:family\.)?(?:shape|subset|payload)\.[0-9a-f]{32,64}\b|\b[0-9a-f]{64}\b')


def masked(data):
    return NAME.sub(b'@LAYOUT_NAME@', data)


def check(base, train, out):
    out.mkdir(parents=True, exist_ok=True)
    left = {p.name: p.read_bytes() for p in base.glob('*.go')}
    right = {p.name: p.read_bytes() for p in train.glob('*.go')}
    if not left or not right:
        raise ValueError('both inputs must contain generated Go sources')
    pairs, pending = [], dict(right)
    # Prefer equal filenames; layout filenames can themselves have changed.
    for name, data in left.items():
        if name in pending:
            pairs.append((name, name))
            del pending[name]
    unmatched = set(left) - {a for a, _ in pairs}
    for name in sorted(unmatched):
        candidates = [n for n, data in pending.items() if masked(left[name]) == masked(data)]
        if len(candidates) != 1:
            raise ValueError(f'cannot pair source {name}: {len(candidates)} candidates')
        other = candidates[0]
        pairs.append((name, other))
        del pending[other]
    if pending:
        raise ValueError(f'new sources: {sorted(pending)}')
    mapping, inverse = {}, {}
    for a, b in pairs:
        chunks = [(left[a], right[b])]
        if masked(left[a]) != masked(right[b]):
            # Infer names from unchanged code around a real difference too,
            # so the retained diff isolates behavior rather than hash churn.
            la, lb = left[a].splitlines(True), right[b].splitlines(True)
            matcher = difflib.SequenceMatcher(None, [masked(x) for x in la], [masked(x) for x in lb])
            chunks = [(b''.join(la[i:i+n]), b''.join(lb[j:j+n])) for i, j, n in matcher.get_matching_blocks() if n]
        for ca, cb in chunks:
            for x, y in zip(NAME.findall(ca), NAME.findall(cb), strict=True):
                if mapping.get(x, y) != y or inverse.get(y, x) != x:
                    raise ValueError(f'non-bijective name mapping in {a}: {x!r} -> {y!r}')
                mapping[x], inverse[y] = y, x
    # Union alternative IDs also appear without their family.payload prefix.
    # Rename a bare digest only when a qualified layout proves its mapping;
    # unrelated checksums must remain an exact part of the byte comparison.
    bare = {}
    for x, y in mapping.items():
        if b'.' in x and b'.' in y:
            a, b = x.rsplit(b'.', 1)[1], y.rsplit(b'.', 1)[1]
            if len(a) == len(b) == 64:
                bare[a] = b
    mapping = {x: y for x, y in mapping.items() if not re.fullmatch(rb'[0-9a-f]{64}', x) or bare.get(x) == y}
    changed = {x: y for x, y in mapping.items() if x != y}
    (out / 'mapping.json').write_text(json.dumps({x.decode(): y.decode() for x, y in sorted(changed.items())}, indent=2) + '\n')
    renamed = out / 'renamed-base'
    renamed.mkdir(exist_ok=True)
    failures = []
    for a, b in pairs:
        actual = NAME.sub(lambda m: mapping.get(m[0], m[0]), left[a])
        actual_name = NAME.sub(lambda m: mapping.get(m[0], m[0]), a.encode()).decode()
        (renamed / actual_name).write_bytes(actual)
        comparison = subprocess.run(['cmp', '-s', str(renamed / actual_name), str(train / b)])
        if comparison.returncode > 1:
            raise OSError(f'cmp failed for {b}: exit {comparison.returncode}')
        if actual_name != b or comparison.returncode:
            diff = ''.join(difflib.unified_diff(actual.decode().splitlines(True), right[b].decode().splitlines(True), fromfile='renamed-base/' + a, tofile='train/' + b))
            (out / (b + '.diff')).write_text(diff)
            failures.append(b)
    print(f'layout-rename-check: {len(pairs)} files, {len(changed)} renames, {len(failures)} differing files')
    if failures:
        print('diffs: ' + ', '.join(failures), file=sys.stderr)
    return bool(failures)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('base', type=Path)
    parser.add_argument('train', type=Path)
    parser.add_argument('out', type=Path)
    args = parser.parse_args()
    try:
        return check(args.base, args.train, args.out)
    except (OSError, ValueError) as error:
        print(f'layout-rename-check: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
