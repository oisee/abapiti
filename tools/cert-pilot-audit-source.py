#!/usr/bin/env python3
"""Read-only MCP view of one immutable pilot audit snapshot; no shell or writes."""
import hashlib
import json
import pathlib
import re
import sys

ROOT = pathlib.Path(sys.argv[1]).resolve()


def safe_path(name):
    path = (ROOT / name).resolve()
    if not path.is_relative_to(ROOT) or not path.is_file():
        raise ValueError("path outside audit snapshot or not a file")
    return path


def sha(data):
    return hashlib.sha256(data).hexdigest()


def call(name, args):
    if name == "inventory":
        prefix = args.get("prefix", "")
        return "\n".join(str(p.relative_to(ROOT)) for p in sorted(ROOT.rglob("*"))
                         if p.is_file() and str(p.relative_to(ROOT)).startswith(prefix))
    if name == "read":
        lines = safe_path(args["path"]).read_text().splitlines()
        start = max(0, args.get("start_line", 1) - 1)
        count = min(600, args.get("line_count", 180))
        return "\n".join(f"{i+1}: {line}" for i, line in enumerate(lines[start:start+count], start))
    if name == "search":
        pattern = re.compile(args["pattern"])
        prefix = args.get("prefix", "")
        found = []
        for path in sorted(ROOT.rglob("*")):
            rel = str(path.relative_to(ROOT))
            if not path.is_file() or not rel.startswith(prefix) or path.suffix not in {".ts", ".go", ".txt", ".json"}:
                continue
            for i, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
                if pattern.search(line):
                    found.append(f"{rel}:{i}:{line[:1000]}")
                    if len(found) >= 100:
                        return "\n".join(found) + "\n[100 matches; narrow prefix/pattern]"
        return "\n".join(found)
    if name == "check_bindings":
        c = json.loads(safe_path("certificates/" + args["id"] + ".json").read_text())
        index = json.loads(safe_path("declaration-spans.json").read_text())
        lookup = {(d["key"]["File"], d["key"]["Symbol"], d["key"]["Kind"]): d for d in index}
        errors = []
        for b in [c["target"]] + c["dependencies"]:
            key = b["key"]
            d = lookup[(key["File"], key["Symbol"], key["Kind"])]
            raw = safe_path("source/packages/core/" + key["File"]).read_bytes()
            span = raw[d["start"]:d["end"]]
            if sha(span) != b["sha256"]:
                errors.append(key)
        receivers = sorted(d["key"]["File"] + "." + d["key"]["Symbol"] for d in index
                           if d["key"]["File"].startswith("src/abap/3_structures/")
                           and d["key"]["Kind"] == "KindClassDeclaration")
        receiver_sha = sha("\n".join(receivers).encode())
        archive_sha = sha(safe_path("abaplint-core-577f875e.tar.gz").read_bytes())
        binding = {"Target": c["target"], "Dependencies": c["dependencies"],
                   "Receivers": c["receiver_sha256"], "Upstream": c["upstream_sha256"]}
        return {"checked_declarations": len(c["dependencies"])+1, "span_errors": errors,
                "target_span": lookup[(c["target"]["key"]["File"], c["target"]["key"]["Symbol"], c["target"]["key"]["Kind"])],
                "receiver_sha256": receiver_sha, "receiver_set_matches": receivers == sorted(c["receiver_set"]),
                "archive_sha256": archive_sha, "receiver_digest_matches": receiver_sha == c["receiver_sha256"],
                "archive_digest_matches": archive_sha == c["upstream_sha256"],
                "binding_sha256": sha(json.dumps(binding, separators=(",", ":")).encode())}
    raise ValueError("unknown read-only tool")


TOOLS = [
    ("inventory", "List files inside the immutable source snapshot.", {"prefix": {"type": "string"}}, []),
    ("read", "Read lines from a snapshot file; use search to locate generated methods.",
     {"path": {"type": "string"}, "start_line": {"type": "integer"}, "line_count": {"type": "integer"}}, ["path"]),
    ("search", "Regex search within snapshot text. No shell; narrow prefix for source or generated host.",
     {"pattern": {"type": "string"}, "prefix": {"type": "string"}}, ["pattern"]),
    ("check_bindings", "Recompute exact declaration-span SHAs from original bytes, closed receiver digest, full upstream archive SHA and canonical binding digest. Offsets use the shared override tsgo token span index; inspect that implementation independently.",
     {"id": {"type": "string"}}, ["id"]),
]

for line in sys.stdin:
    try:
        request = json.loads(line)
        rid = request.get("id")
        method = request.get("method")
        if rid is None:
            continue
        if method == "initialize":
            result = {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}},
                      "serverInfo": {"name": "cert-pilot-read-only-source", "version": "1"}}
        elif method == "tools/list":
            result = {"tools": [{"name": n, "description": d,
                                 "inputSchema": {"type": "object", "properties": p, "required": r,
                                                 "additionalProperties": False},
                                 "annotations": {"readOnlyHint": True, "destructiveHint": False}}
                                for n, d, p, r in TOOLS]}
        elif method == "tools/call":
            params = request["params"]
            value = call(params["name"], params.get("arguments", {}))
            result = {"content": [{"type": "text", "text": value if isinstance(value, str) else json.dumps(value)}]}
        elif method == "ping":
            result = {}
        else:
            raise ValueError("unsupported method")
        response = {"jsonrpc": "2.0", "id": rid, "result": result}
    except Exception as error:
        response = {"jsonrpc": "2.0", "id": request.get("id"),
                    "error": {"code": -32603, "message": str(error)}}
    print(json.dumps(response), flush=True)
