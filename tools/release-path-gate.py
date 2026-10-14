#!/usr/bin/env python3
"""Offline release gate: inspect every file and every zip/tar member.

Binary bytes are scanned as well as `strings -a` output, including binaries
inside archives. No allowlist: a leak prevents checksums/publication.
"""
import argparse
import getpass
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import zipfile


def scan(stream, needles):
    tail = b""
    keep = max(map(len, needles)) - 1
    found = set()
    while chunk := stream.read(1024 * 1024):
        data = tail + chunk
        found.update(label for needle, label in needles.items() if needle in data)
        tail = data[-keep:] if keep else b""
    return found


def check_stream(stream, needles):
    # Spool binary members without unbounded memory use; strings must inspect
    # their actual decompressed bytes, not the enclosing compressed archive.
    with tempfile.TemporaryFile() as staged:
        prefix = stream.read(8192)
        staged.write(prefix)
        while chunk := stream.read(1024 * 1024):
            staged.write(chunk)
        staged.seek(0)
        found = scan(staged, needles)
        if b"\0" in prefix:
            staged.seek(0)
            with subprocess.Popen(["strings", "-a"], stdin=staged, stdout=subprocess.PIPE) as proc:
                found.update(scan(proc.stdout, needles))
                if proc.wait() != 0:
                    raise RuntimeError("strings failed")
        return found


def check_asset(path, needles):
    violations = []

    def check(name, stream):
        found = scan(io.BytesIO(name.encode()), needles) | check_stream(stream, needles)
        if found:
            # Avoid reproducing leaked paths in the release report.
            violations.append({"entry": len(violations) + 1, "tokens": sorted(found)})

    # Include the asset name and raw archive metadata (comments/headers),
    # then inspect decompressed members as well. Compressed bytes alone are
    # never evidence that embedded sources/binaries are clean.
    with path.open("rb") as stream:
        check(path.name, stream)
    if zipfile.is_zipfile(path):
        with zipfile.ZipFile(path) as archive:
            for entry in archive.infolist():
                with archive.open(entry) as stream:
                    check(entry.filename, stream)
    elif path.name.endswith((".tar.gz", ".tgz", ".tar")):
        with tarfile.open(path) as archive:
            for entry in archive:
                if entry.isfile():
                    with archive.extractfile(entry) as stream:
                        check(entry.name, stream)
                else:
                    check(entry.name + " " + entry.linkname, io.BytesIO(b""))
    return {"asset": path.name, "leakedEntries": len(violations), "violations": violations}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("assets", type=Path)
    parser.add_argument("--root", action="append", required=True)
    parser.add_argument("--user", default=getpass.getuser())
    args = parser.parse_args()
    needles = {b"/mnt/": "mnt", b"/home/": "home", b"/tmp/": "tmp"}
    for root in args.root:
        needles[os.fsencode(root)] = "build-root"
        needles[os.fsencode(root.replace("/", "\\"))] = "build-root"
    if not args.user:
        parser.error("user name must not be empty")
    needles[args.user.encode()] = "user"
    results = [check_asset(p, needles) for p in sorted(args.assets.iterdir()) if p.is_file()]
    if not results:
        parser.error("no assets to check")
    print(json.dumps(results, indent=2))
    return int(any(row["leakedEntries"] for row in results))


if __name__ == "__main__":
    raise SystemExit(main())
