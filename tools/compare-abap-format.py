#!/usr/bin/env python3
"""Compare generated ABAP trees, ignoring indentation and statement wrapping.

Literal/template bytes and comment contents are retained. Outside literals,
physical line breaks and runs of spaces are canonicalized to one space.
Usage: compare-abap-format.py BEFORE AFTER
"""
import pathlib
import sys


def canonical(source):
    result = []
    quote = None
    for line in source.splitlines():
        line = line.lstrip(" \t")
        if line.startswith("*"):
            result.append("\n" + line + "\n")
            continue
        i = 0
        while i < len(line):
            c = line[i]
            if quote:
                result.append(c)
                if quote == "|" and c == "\\" and i + 1 < len(line):
                    i += 1
                    result.append(line[i])
                elif c == quote:
                    if quote != "|" and i + 1 < len(line) and line[i + 1] == quote:
                        i += 1
                        result.append(line[i])
                    else:
                        quote = None
            elif c == '"':
                result.append(line[i:] + "\n")
                break
            elif c in "'`|":
                quote = c
                result.append(c)
            elif c.isspace():
                if result and result[-1] != " ":
                    result.append(" ")
            else:
                result.append(c)
            i += 1
        if quote:
            result.append("\n")
        elif result and result[-1] != " ":
            result.append(" ")
    return "".join(result).strip()


def files(root):
    root = pathlib.Path(root)
    return {str(p.relative_to(root)): canonical(p.read_text())
            for p in root.rglob("*.abap")}


before, after = map(files, sys.argv[1:])
different = [p for p in sorted(before.keys() | after.keys())
             if before.get(p) != after.get(p)]
if different:
    print(f"FAIL: {len(different)} differing/missing files")
    print("\n".join(different))
    sys.exit(1)
print(f"PASS: {len(before)} ABAP files identical ignoring indentation/wrapping")
