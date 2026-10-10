#!/usr/bin/env python3
"""Deterministic test partitions (wasm, tsfront), textual coverage merge, and CI aggregation."""
import collections
import json
from pathlib import Path
import re
import sys

WASM = "github.com/oisee/abapiti/wasm"
TSFRONT = "github.com/oisee/abapiti/tsfront"
# Packages whose top-level tests are split in halves over two shards, and the
# shards that run the halves.
SPLIT = {WASM: ("wasm", (1, 2)), TSFRONT: ("tsfront", (3, 4))}
SHARDS = (1, 2, 3, 4)


def split(listing, out, half, name="wasm"):
    # Benchmarks are listed by -list . too, but go test does not run them.
    names = sorted(line for line in listing.read_text().splitlines()
                   if re.fullmatch(r"(?:Test|Example|Fuzz)\w*", line))
    if not names or len(names) != len(set(names)):
        raise ValueError(f"empty or duplicate {name} test list")
    out.mkdir(parents=True, exist_ok=True)
    (out / f"{name}-listed.json").write_text(json.dumps(names))
    selected = names[half - 1::2]
    # An empty shard must never fall back to running all tests.
    pattern = "^(" + "|".join(selected) + ")$" if selected else "^$"
    (out / f"{name}-run.txt").write_text(pattern + "\n")
    print(f"{name} half {half}: {len(selected)} of {len(names)} top-level tests; regex {len(pattern)} bytes")


def merge(profiles, dest):
    mode = None
    blocks = {}
    for profile in profiles:
        lines = profile.read_text().splitlines()
        if not lines or lines[0] not in ("mode: set", "mode: count", "mode: atomic"):
            raise ValueError(f"invalid coverage header: {profile}")
        if mode is not None and mode != lines[0]:
            raise ValueError("coverage modes differ")
        mode = lines[0]
        for line in lines[1:]:
            location, statements, count = line.split()
            statements, count = int(statements), int(count)
            if location in blocks and blocks[location][0] != statements:
                raise ValueError(f"inconsistent block: {location}")
            blocks[location] = (statements, max(count, blocks.get(location, (0, 0))[1]))
    if mode is None:
        raise ValueError("no coverage profiles")
    dest.write_text(mode + "\n" + "".join(
        f"{loc} {n} {count}\n" for loc, (n, count) in sorted(blocks.items())))
    return blocks


def aggregate(root, out):
    out.mkdir(parents=True, exist_ok=True)
    shards = [root / f"test-shard-{i}" for i in SHARDS]
    logs = [s / "test.json" for s in shards if (s / "test.json").is_file()]
    combined = "".join(p.read_text().rstrip() + "\n" for p in logs)
    (out / "test.json").write_text(combined)
    if any((s / "tree-dirty").exists() for s in shards):
        (out / "tree-dirty").touch()
    profiles = [s / "cover.out" for s in shards if (s / "cover.out").is_file()]
    blocks = merge(profiles, out / "cover.out") if profiles else {}
    coverage = collections.defaultdict(lambda: [0, 0])
    for loc, (n, count) in blocks.items():
        pkg = loc.rsplit("/", 1)[0]
        coverage[pkg][0] += n
        coverage[pkg][1] += n if count > 0 else 0
    packages = {}
    for line in combined.splitlines():
        event = json.loads(line)
        pkg = event.get("Package")
        if not pkg:
            continue
        row = packages.setdefault(pkg, dict(pkg=pkg, pass_=0, fail=0, skip=0, seconds=None, cover=None))
        action = event.get("Action")
        if event.get("Test") and action in ("pass", "fail", "skip"):
            row["pass_" if action == "pass" else action] += 1
        if not event.get("Test") and action in ("pass", "fail") and "Elapsed" in event:
            # Sum package work across split halves, comparable to the old serial run.
            row["seconds"] = (row["seconds"] or 0) + event["Elapsed"]
    for pkg, row in packages.items():
        row["pass"] = row.pop("pass_")
        total, covered = coverage[pkg]
        if total:
            row["cover"] = round(100 * covered / total, 1)
    (out / "test-summary.json").write_text(json.dumps({"packages": [packages[p] for p in sorted(packages)]}) + "\n")


def check(root):
    logs = {}
    for i in SHARDS:
        shard = root / f"test-shard-{i}"
        # Require artifacts even if a shard was cancelled or failed before testing.
        events = [json.loads(line) for line in (shard / "test.json").read_text().splitlines()]
        if not events:
            raise ValueError(f"empty test log for shard {i}")
        logs[i] = events
    for pkg, (name, pair) in SPLIT.items():
        listed = [json.loads((root / f"test-shard-{i}" / f"{name}-listed.json").read_text()) for i in pair]
        if listed[0] != listed[1]:
            raise ValueError(f"{name} test lists differ between shards")
        executed = collections.Counter()
        for events in logs.values():
            for event in events:
                test = event.get("Test", "")
                if event.get("Package") == pkg and event.get("Action") == "run" and test and "/" not in test:
                    executed[test] += 1
        expected = collections.Counter(listed[0])
        if executed != expected:
            raise ValueError(f"{name} execution mismatch: missing={dict(expected - executed)}, extra/duplicate={dict(executed - expected)}")
        print(f"All {len(expected)} listed top-level {name} tests ran exactly once")


if __name__ == "__main__":
    command, *args = sys.argv[1:]
    if command == "split":
        split(Path(args[0]), Path(args[1]), int(args[2]), *args[3:])
    elif command == "merge":
        merge([Path(p) for p in args[1:]], Path(args[0]))
    elif command == "aggregate":
        aggregate(Path(args[0]), Path(args[1]))
    elif command == "check":
        check(Path(args[0]))
    else:
        raise ValueError(f"unknown command: {command}")
