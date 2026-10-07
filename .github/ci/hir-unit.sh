#!/usr/bin/env bash
set -euo pipefail
work=${1:?usage: hir-unit.sh <osgo workdir>}
root=$(git rev-parse --show-toplevel)
osg="$work/open-steamgate"
gen="$work/hir"
(cd "$root" && ABAPITI_TEST_OUT="$gen" go test ./hir/abap -run '^TestFixtures$' -count=1)
node "$root/.github/ci/hir-lint.mjs" "$gen/TestFixtures" "$osg"
for runtime in osgo osgjs; do
  status=0
  (cd "$osg" && GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false npm run -s "$runtime:unit" -- "$gen/TestFixtures" --json) > "$work/hir-$runtime.json" 2> "$work/hir-$runtime.err" || status=$?
  node -e '
    const fs = require("fs");
    const d = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
    const t = d.totals;
    console.log(`${process.argv[3]} HIR: ${t.success}/${t.tests} passed`);
    if (Number(process.argv[2]) !== 0 || t.tests !== 6 || t.success !== 6 || d.rows.length !== 6 || d.rows.some(r => r.status !== "SUCCESS")) process.exit(1);
  ' "$work/hir-$runtime.json" "$status" "$runtime"
done
