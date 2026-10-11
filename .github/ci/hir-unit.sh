#!/usr/bin/env bash
set -euo pipefail
work=${1:?usage: hir-unit.sh <osgo workdir>}
root=$(git rev-parse --show-toplevel)
osg="$work/open-steamgate"
gen="$work/hir"
trap 'rm -rf "$gen"' EXIT
rm -rf "$gen/TestRuntimeDebtFixtures" "$gen/TestFixtures" "$gen/Test750Semantics" "$gen/TestEmitSyntaxFeatures" "$gen/TestEmitStringRepeatMatch" "$gen/TestEmitRegExpExec" "$gen/TestConfigJSONFixtures" "$gen/TestEmitSwitchGroupedLabels"
(cd "$root" && ABAPITI_TEST_OUT="$gen" go test -p 2 ./hir/abap -run '^(TestFixtures|Test750Semantics|TestRuntimeDebtFixtures|TestConfigJSONFixtures)$' -count=1)
for suite in TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec TestEmitSwitchGroupedLabels; do
  (cd "$root" && ABAPITI_TEST_OUT="$gen/$suite" go test -p 2 ./tsfront -run "^$suite$" -count=1)
done
node "$root/.github/ci/hir-lint-test.mjs" "$osg"
node "$root/.github/ci/hir-lint.mjs" "$gen/TestFixtures" "$osg"
node "$root/.github/ci/hir-lint.mjs" "$gen/TestRuntimeDebtFixtures" "$osg" --fixture
node "$root/.github/ci/hir-lint.mjs" "$gen/Test750Semantics" "$osg"
for suite in TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec TestEmitSwitchGroupedLabels; do
  node "$root/.github/ci/hir-lint.mjs" "$gen/$suite" "$osg" --fixture
done
for classes in "$gen"/TestConfigJSONFixtures/case*; do
  node "$root/.github/ci/hir-lint.mjs" "$classes" "$osg"
done
node "$root/.github/ci/hir-lint.mjs" "$gen/TestConfigJSONFixtures/case0" "$osg" config-json-v702
node "$root/.github/ci/osg-transpiler.mjs" "$osg"
result=0
for runtime in osgo osgjs; do
  for suite in TestFixtures TestRuntimeDebtFixtures Test750Semantics TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec TestEmitSwitchGroupedLabels TestConfigJSONFixtures/case{0,1,2,3}; do
    expected=7
    if [[ "$suite" != TestFixtures ]]; then expected=1; fi
    if [[ "$suite" == TestRuntimeDebtFixtures ]]; then expected=475; fi
    classes="$gen/$suite"
    status=0
    runtime_args=()
    if [[ "$runtime" == osgo ]]; then runtime_args=(--jobs 2); fi
    (cd "$osg" && GOTOOLCHAIN=go1.26.0 GOFLAGS="${GOFLAGS:-} -p=2 -buildvcs=false" npm run -s "$runtime:unit" -- "$classes" --json "${runtime_args[@]}") > "$work/hir-$runtime-${suite//\//-}.json" 2> "$work/hir-$runtime-${suite//\//-}.err" || status=$?
    node -e '
      const fs = require("fs");
      const d = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
      const t = d.totals, expected = Number(process.argv[4]);
      // On osgo, a NOT_COMPILED row whose diagnostic is exactly a line of
      // osgo-known-gaps.txt is a recorded open-steamgate gap, not a failure.
      const gaps = process.argv[3].startsWith("osgo ") ? new Set(fs.readFileSync(process.argv[5], "utf8").split("\n").filter(l => l && !l.startsWith("#"))) : new Set();
      const regexSuite = ["TestEmitStringRepeatMatch", "TestEmitRegExpExec"].includes(process.argv[6]);
      const pcreGap = "NOT_COMPILED in FIND PCRE: a negative lookbehind (?<!...) is not in Go\u0027s RE2:";
      const pinnedPCRE = r => regexSuite && process.argv[3].startsWith("osgo ") && Number(process.argv[2]) === 2 && t.tests === 1 && t.not_compiled === 1 && t.success === 0 && t.failure === 0 && t.error === 0 && d.rows.length === 1 && String(r.message || "").startsWith(pcreGap);
      const known = r => r.status === "NOT_COMPILED" && (gaps.has(String(r.message || "").replace(/^NOT_COMPILED in .*?\): /, "")) || pinnedPCRE(r));
      const gapped = d.rows.filter(known).length;
      console.log(`${process.argv[3]} HIR: ${t.success}/${t.tests} passed` + (gapped ? `, ${gapped} known osgo gap(s)` : ""));
      if (t.tests !== expected || d.rows.length !== expected || t.success + gapped !== expected || d.rows.some(r => r.status !== "SUCCESS" && !known(r)) || (Number(process.argv[2]) !== 0 && gapped === 0)) process.exit(1);
    ' "$work/hir-$runtime-${suite//\//-}.json" "$status" "$runtime $suite" "$expected" "$root/.github/ci/osgo-known-gaps.txt" "$suite" || result=1
  done
done
node "$root/.github/ci/lexer-unit.mjs" "$work" || result=1
exit "$result"
