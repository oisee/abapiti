#!/usr/bin/env bash
# Run the generated OSD classes as ABAP Unit on osgo (open-steamgate's Go
# runtime) straight from the files: no ADT, no activation. open-steamgate is
# checked out at the commit pinned in .github/ci/osgo.ref.
#
#   .github/ci/osgo-unit.sh <workdir>
#
# Exit codes are osgo:unit's: 0 every test passed, 1 a test failed,
# 2 NOT_COMPILED / ERROR / SKIPPED or a line over 255 characters, 3 no tests.
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
node -e '
  const d = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
  const t = d.totals || {};
  console.log(`osgo-unit: ${t.success} passed, ${t.failure} failed, ${t.not_compiled} not compiled, ${t.error} errors, ${t.tests} tests`);
  for (const r of d.rows || []) if (r.status !== "SUCCESS") console.log(`  ${r.class} ${r.method} ${r.status} ${String(r.message || "").split("\n")[0].slice(0, 160)}`);
' "$work/osgo.json" >&2 || { tail -20 "$work/osgo.err" >&2; [ "$status" -ne 0 ] || status=4; }
exit "$status"
