#!/usr/bin/env python3
"""Compare TS-HG@Go with the release kit without whitespace normalization."""
import argparse
import json
from pathlib import Path
import statistics
import subprocess
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("kit", type=Path)
parser.add_argument("binary", type=Path)
parser.add_argument("--runs", type=int, default=1)
parser.add_argument("--output", type=Path)
args = parser.parse_args()
if args.runs < 1:
    parser.error("--runs must be positive")
kit, binary = args.kit.resolve(), args.binary.resolve()
rows = {}
for variant in ("clean", "seeded"):
    source = "zabapgit_standalone.prog.abap"
    if variant == "seeded":
        source = "seeded/" + source
    expected = (kit / f"expected-{variant}.txt").read_bytes()
    samples = []
    for run in range(args.runs):
        start = time.perf_counter()
        result = subprocess.run(
            [str(binary), "--file", str(kit / source), "--config", str(kit / "abaplint.json"), "--deps", str(kit / "deps.txt")],
            cwd=binary.parent, capture_output=True, check=True,
        )
        samples.append(time.perf_counter() - start)
        if result.stdout != expected:
            raise SystemExit(f"{variant} run {run + 1}: bytes differ from expected-{variant}.txt")
    rows[variant] = {"seconds": samples, "median_seconds": statistics.median(samples),
                     "byte_identical": True, "issues": int(expected.splitlines()[0])}
    print(f"TS-HG@Go {variant}: byte-identical, median {statistics.median(samples):.3f}s ({args.runs} runs)")
if args.output:
    args.output.write_text(json.dumps(rows, indent=2) + "\n")
