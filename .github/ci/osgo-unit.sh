#!/usr/bin/env bash
# Run the generated OSD classes as ABAP Unit on osgo (open-steamgate's Go
# runtime) straight from the files: no ADT, no activation. open-steamgate is
# checked out at the commit pinned in .github/ci/osgo.ref.
#
#   .github/ci/osgo-unit.sh <workdir>
#
# Exit 0 when every test passed except NOT_COMPILED rows matching a known osgo
# gap (osgo-known-gaps.txt); 1 for any other non-passing row; 4 for setup,
# generation or JSON problems.
# Needs Node 22+; Go 1.26 is fetched through GOTOOLCHAIN.
set -euo pipefail

work=${1:?usage: osgo-unit.sh <workdir>}
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
ref=$(tr -d '[:space:]' < "$here/osgo.ref")
case $ref in
  [0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]*) ;;
  *) echo "osgo-unit: .github/ci/osgo.ref is not a commit id" >&2; exit 4 ;;
esac
[ "${#ref}" -eq 40 ] || { echo "osgo-unit: .github/ci/osgo.ref must be a full 40-character commit id" >&2; exit 4; }

mkdir -p "$work"
work=$(cd "$work" && pwd)
osg="$work/open-steamgate"
gen="$work/gen"

if [ ! -d "$osg/.git" ]; then
  git init -q "$osg"
  git -C "$osg" remote add origin https://github.com/oisee/open-steamgate.git
fi
git -C "$osg" fetch -q --depth 1 origin "$ref"
git -C "$osg" checkout -q --detach FETCH_HEAD
[ "$(git -C "$osg" rev-parse HEAD)" = "$ref" ] || { echo "osgo-unit: checkout is not $ref" >&2; exit 4; }
echo "osgo-unit: open-steamgate $ref" >&2

(cd "$osg" && npm ci --no-audit --no-fund && node tools/osd-libs.mjs --sync && node tools/osd-fetch.mjs) > "$work/setup.log" 2>&1 ||
  { cat "$work/setup.log" >&2; exit 4; }

rm -rf "$gen"
(cd "$root" && ABAPITI_TEST_OUT="$gen" go test ./wasm -run '^TestOSD_EmitUnitClasses$' -count=1) > "$work/generate.log" 2>&1 ||
  { cat "$work/generate.log" >&2; exit 4; }
classes="$gen/TestOSD_EmitUnitClasses"
n=$(find "$classes" -name '*.clas.abap' ! -name '*.testclasses.abap' | wc -l)
[ "$n" -gt 0 ] || { echo "osgo-unit: the generator wrote no classes" >&2; exit 4; }
echo "osgo-unit: $n classes" >&2

status=0
(cd "$osg" && GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false npm run -s osgo:unit -- "$classes" --json) > "$work/osgo.json" 2> "$work/osgo.err" || status=$?
# The runner's exit code is not enough on its own: it must be one the runner
# documents (0 green, 1 failure, 2 not compiled or error) and agree with the
# report; the JSON must carry a rows array that matches the totals, at least
# one row must pass, and every row that did not pass must be a NOT_COMPILED
# whose diagnostic is exactly a line of osgo-known-gaps.txt. Anything else (a
# failure, an error, a skip, an unknown NOT_COMPILED) is exit 1; a report that
# cannot be trusted is exit 4.
gaps="$here/osgo-known-gaps.txt"
verdict=0
node -e '
  const fs = require("fs");
  const runner = Number(process.argv[3]);
  const fail = (m) => { console.log("osgo-unit: " + m); process.exit(4); };
  let d;
  try { d = JSON.parse(fs.readFileSync(process.argv[1], "utf8")); } catch (e) { fail("the JSON does not parse: " + e.message); }
  const gaps = fs.readFileSync(process.argv[2], "utf8").split("\n").map((l) => l.trim()).filter((l) => l && !l.startsWith("#"));
  const t = d && d.totals;
  const keys = { success: "SUCCESS", failure: "FAILURE", not_compiled: "NOT_COMPILED", error: "ERROR", skipped: "SKIPPED" };
  const n = (k) => (t && Number.isInteger(t[k]) && t[k] >= 0 ? t[k] : NaN);
  const tests = n("tests"), skipped = t && t.skipped === undefined ? 0 : n("skipped");
  const [ok, failed, nc, err] = ["success", "failure", "not_compiled", "error"].map(n);
  if ([ok, failed, nc, err, skipped, tests].some(Number.isNaN) || tests < 1) fail("the JSON has no valid totals");
  if (!Array.isArray(d.rows) || d.rows.length !== tests) fail(`the JSON has ${Array.isArray(d.rows) ? d.rows.length : "no"} rows for ${tests} tests`);
  const count = {};
  for (const r of d.rows) count[r && r.status] = (count[r && r.status] || 0) + 1;
  for (const [k, s] of Object.entries(keys)) if ((count[s] || 0) !== (k === "skipped" ? skipped : n(k))) fail(`${s} rows do not match totals.${k}`);
  if (Object.keys(count).some((s) => !Object.values(keys).includes(s))) fail("a row has an unknown status");
  const want = failed > 0 ? 1 : nc + err > 0 ? 2 : 0;
  if (runner !== want && !(runner === 2 && want === 1)) fail(`the runner exited ${runner}, the report says ${want}`);
  console.log(`osgo-unit: ${ok} passed, ${failed} failed, ${nc} not compiled, ${err} errors, ${skipped} skipped, ${tests} tests`);
  const used = new Set();
  let bad = 0, known = 0;
  for (const r of d.rows) {
    if (r.status === "SUCCESS") continue;
    const msg = String(r.message || "").split("\n")[0];
    const m = /^NOT_COMPILED in \S+ \([^)]*\): (.*)$/.exec(msg);
    const g = r.status === "NOT_COMPILED" && m && gaps.includes(m[1]) ? m[1] : undefined;
    if (g) { known++; used.add(g); continue; }
    bad++;
    console.log(`  ${r.class} ${r.method} ${r.status} ${msg.slice(0, 160)}`);
  }
  if (known) console.log(`osgo-unit: ${known} not compiled because of known osgo gaps: ${[...used].join("; ")}`);
  for (const g of gaps) if (!used.has(g)) console.log(`osgo-unit: known gap "${g}" matched nothing; remove it from osgo-known-gaps.txt if open-steamgate compiles it now`);
  if (ok < 1) { console.log("osgo-unit: no test passed"); process.exit(5); }
  if (bad > 0) process.exit(5);
' "$work/osgo.json" "$gaps" "$status" >&2 || verdict=$?
case $verdict in
  0) exit 0 ;;
  5) exit 1 ;;
  *) tail -20 "$work/osgo.err" >&2; exit 4 ;;
esac
