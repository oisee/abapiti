#!/usr/bin/env bash
set -euo pipefail
work=${1:?usage: hir-unit.sh <osgo workdir>}
root=$(git rev-parse --show-toplevel)
osg="$work/open-steamgate"
gen="$work/hir"
rm -rf "$gen/TestFixtures" "$gen/Test750Semantics"
(cd "$root" && ABAPITI_TEST_OUT="$gen" go test ./hir/abap -run '^(TestFixtures|Test750Semantics)$' -count=1)
node "$root/.github/ci/hir-lint-test.mjs" "$osg"
node "$root/.github/ci/hir-lint.mjs" "$gen/TestFixtures" "$osg"
node "$root/.github/ci/hir-lint.mjs" "$gen/Test750Semantics" "$osg"
result=0
for runtime in osgo osgjs; do
  for suite in TestFixtures Test750Semantics; do
    expected=6
    if [[ "$suite" == Test750Semantics ]]; then expected=1; fi
    classes="$gen/$suite"
    status=0
    (cd "$osg" && GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false npm run -s "$runtime:unit" -- "$classes" --json) > "$work/hir-$runtime-$suite.json" 2> "$work/hir-$runtime-$suite.err" || status=$?
    node -e '
      const fs = require("fs");
      const d = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
      const t = d.totals, expected = Number(process.argv[4]);
      console.log(`${process.argv[3]} HIR: ${t.success}/${t.tests} passed`);
      if (Number(process.argv[2]) !== 0 || t.tests !== expected || t.success !== expected || d.rows.length !== expected || d.rows.some(r => r.status !== "SUCCESS")) process.exit(1);
    ' "$work/hir-$runtime-$suite.json" "$status" "$runtime $suite" "$expected" || result=1
  done
done
node "$root/.github/ci/lexer-unit.mjs" "$work" || result=1
exit "$result"
