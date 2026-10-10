#!/usr/bin/env python3
"""Join a site-profile/1 certificate to sites/1 and print workload tables."""
import argparse
import json
from pathlib import Path


def percent(numerator, denominator):
    return f"{100 * numerator / denominator:.2f}%" if denominator else "0.00%"


def report(site_map, profile):
    if site_map.get("schema") != "sites/1" or profile.get("schema") != "site-profile/1":
        raise ValueError("expected sites/1 and site-profile/1")
    identities = {site["site_id"]: site for site in site_map["sites"]}
    counters = profile["sites"]
    missing = counters.keys() - identities.keys()
    if missing:
        raise ValueError(f"profile has {len(missing)} sites absent from map")

    def label(site_id):
        site = identities[site_id]
        return f"`{site['method']}` ({site['source'] or 'synthetic'})"

    def ranked(kind, metric, limit):
        return sorted(
            ((site_id, c) for site_id, c in counters.items() if c["kind"] == kind),
            key=lambda pair: (-pair[1].get(metric, 0), pair[0]),
        )[:limit]

    lines = [
        "Zabapgit clean kit site profile",
        "",
        f"Input SHA-256: `{profile['input_sha256']}`",
        "",
        f"Binary SHA-256: `{profile['binary_sha256']}`",
        "",
        "Top 30 virtual call sites; receiver histograms contain all observed classes.",
        "",
        "| TS method and source | Calls | Dominant receiver share | Receiver histogram |",
        "|---|---:|---:|---|",
    ]
    for site_id, c in ranked("virtual_call", "calls", 30):
        receivers = sorted(c.get("receivers", {}).items(), key=lambda pair: (-pair[1], pair[0]))
        dominant = receivers[0][1] if receivers else 0
        histogram = "; ".join(f"`{name}`: {count:,}" for name, count in receivers)
        lines.append(f"| {label(site_id)} | {c.get('calls', 0):,} | {percent(dominant, c.get('calls', 0))} | {histogram} |")

    loops = [c for c in counters.values() if c["kind"] == "loop"]
    row_loops = [c for site_id, c in counters.items() if c["kind"] == "loop" and site_id.rsplit("|", 2)[-2] == "foreach"]
    row_invocations = sum(c.get("invocations", 0) for c in row_loops)
    row_zero = sum(c["histogram"]["0"] for c in row_loops)
    row_one = sum(c["histogram"]["1"] for c in row_loops)
    invocations = sum(c.get("invocations", 0) for c in loops)
    trips = sum(c.get("trips", 0) for c in loops)
    buckets = {b: sum(c.get("histogram", {}).get(b, 0) for c in loops) for b in ("0", "1", "2-3", "4-7", "8+")}
    lines += [
        "", f"All loops: {invocations:,} invocations, {trips:,} total trips; "
        f"0/1-trip share: {percent(buckets['0'] + buckets['1'], invocations)}.", "",
        f"ForEach row loops: {row_invocations:,} invocations; zero rows {percent(row_zero, row_invocations)}, "
        f"one row {percent(row_one, row_invocations)}, combined {percent(row_zero + row_one, row_invocations)}.", "",
        "| Trip bucket | Invocations | Share |", "|---|---:|---:|",
    ]
    lines.extend(f"| {b} | {n:,} | {percent(n, invocations)} |" for b, n in buckets.items())
    lines += [
        "", "Top 20 loops by total actual body entries.", "",
        "| TS method and source | Trips | Invocations | 0 | 1 | 2–3 | 4–7 | 8+ | 0/1 share |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for site_id, c in ranked("loop", "trips", 20):
        hist = c["histogram"]
        values = " | ".join(f"{hist[b]:,}" for b in buckets)
        lines.append(f"| {label(site_id)} | {c.get('trips', 0):,} | {c['invocations']:,} | {values} | {percent(hist['0'] + hist['1'], c['invocations'])} |")
    allocations = sum(c.get("allocations", 0) for c in counters.values())
    lines += [
        "", f"Top 20 allocation sites; {allocations:,} total counted HIR `new` executions.", "",
        "| TS method and source | Allocations | Share |", "|---|---:|---:|",
    ]
    for site_id, c in ranked("new", "allocations", 20):
        lines.append(f"| {label(site_id)} | {c.get('allocations', 0):,} | {percent(c.get('allocations', 0), allocations)} |")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("sites", type=Path)
    parser.add_argument("profile", type=Path)
    args = parser.parse_args()
    print(report(json.loads(args.sites.read_text()), json.loads(args.profile.read_text())), end="")


if __name__ == "__main__":
    main()
