#!/usr/bin/env bash
set -euo pipefail
work=${1:?usage: hir-unit.sh <osgo workdir>}
root=$(git rev-parse --show-toplevel)
osg="$work/open-steamgate"
gen="$work/hir"
trap 'rm -rf "$gen"' EXIT
rm -rf "$gen/TestRuntimeDebtFixtures" "$gen/TestNumericFixtures" "$gen/TestFixtures" "$gen/Test750Semantics" "$gen/TestEmitSyntaxFeatures" "$gen/TestEmitStringRepeatMatch" "$gen/TestEmitRegExpExec" "$gen/TestEmitPOSIXRegexJSClasses" "$gen/TestConfigJSONFixtures" "$gen/TestJSONSubsetFixtures" "$gen/TestJSONSubsetMaterializedFixtures" "$gen/TestJSONSubsetConfigFixture" "$gen/TestLiveClassViewJSONNodeOracle" "$gen/JSONStructural" "$gen/TestEmitSwitchGroupedLabels"
(cd "$root" && ABAPITI_TEST_OUT="$gen" go test -p 2 ./hir/abap -run '^(TestFixtures|TestNumericFixtures|Test750Semantics|TestRuntimeDebtFixtures|TestConfigJSONFixtures|TestJSONSubsetFixtures|TestJSONSubsetMaterializedFixtures|TestJSONSubsetConfigFixture)$' -count=1)
for suite in TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec TestEmitPOSIXRegexJSClasses TestEmitSwitchGroupedLabels; do
  (cd "$root" && ABAPITI_TEST_OUT="$gen/$suite" go test -p 2 ./tsfront -run "^$suite$" -count=1)
done
(cd "$root" && ABAPITI_TEST_OUT="$gen/TestLiveClassViewJSONNodeOracle" go test -p 2 ./tsfront -run '^TestLiveClassViewJSONNodeOracle$' -count=1)
(cd "$root" && ABAPITI_TEST_OUT="$gen/JSONStructural" go test -p 2 ./tsfront -run '^TestABAPJSONStructural.*NodeOracle$' -count=1)
node "$root/.github/ci/hir-lint-test.mjs" "$osg"
node "$root/.github/ci/hir-lint.mjs" "$gen/TestFixtures" "$osg"
node "$root/.github/ci/hir-lint.mjs" "$gen/TestRuntimeDebtFixtures" "$osg" --fixture
node "$root/.github/ci/hir-lint.mjs" "$gen/TestNumericFixtures" "$osg" --fixture
node "$root/.github/ci/hir-lint.mjs" "$gen/Test750Semantics" "$osg"
for suite in TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec TestEmitPOSIXRegexJSClasses TestEmitSwitchGroupedLabels; do
  node "$root/.github/ci/hir-lint.mjs" "$gen/$suite" "$osg" --fixture
done
for classes in "$gen"/TestConfigJSONFixtures/case*; do
  node "$root/.github/ci/hir-lint.mjs" "$classes" "$osg"
done
for classes in "$gen"/TestJSONSubsetFixtures/case* "$gen"/TestJSONSubsetMaterializedFixtures/case*; do
  node "$root/.github/ci/hir-lint.mjs" "$classes" "$osg" --fixture
done
node "$root/.github/ci/hir-lint.mjs" "$gen/TestJSONSubsetConfigFixture" "$osg" --fixture
node "$root/.github/ci/hir-lint.mjs" "$gen/TestLiveClassViewJSONNodeOracle/json-live" "$osg" --fixture
for classes in "$gen"/JSONStructural/json-*; do
  node "$root/.github/ci/hir-lint.mjs" "$classes" "$osg" --fixture
done
node "$root/.github/ci/osg-transpiler.mjs" "$osg"
result=0
# OSGO is the gate on every PR; OSG-JS runs nightly (HIR_RUNTIMES="osgo osgjs").
for runtime in ${HIR_RUNTIMES:-osgo}; do
  for suite in TestNumericFixtures TestFixtures TestRuntimeDebtFixtures Test750Semantics TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec TestEmitPOSIXRegexJSClasses TestEmitSwitchGroupedLabels TestConfigJSONFixtures/case{0,1,2,3} TestJSONSubsetFixtures/case{0,1,2,3,4,5} TestJSONSubsetMaterializedFixtures/case{0,1,2,3,4,5} TestJSONSubsetConfigFixture TestLiveClassViewJSONNodeOracle/json-live JSONStructural/json-structural JSONStructural/json-structural-live JSONStructural/json-structural-config JSONStructural/json-structural-wide; do
    expected=7
    if [[ "$suite" != TestFixtures ]]; then expected=1; fi
    if [[ "$suite" == TestRuntimeDebtFixtures ]]; then expected=475; fi
    if [[ "$suite" == TestEmitPOSIXRegexJSClasses ]]; then expected=2; fi
    if [[ "$suite" == TestNumericFixtures ]]; then expected=68; fi
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
