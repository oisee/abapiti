#!/usr/bin/env bash
set -euo pipefail
work=${1:?usage: hir-unit.sh <osgo workdir>}
root=$(git rev-parse --show-toplevel)
osg="$work/open-steamgate"
gen="$work/hir"
rm -rf "$gen/TestFixtures" "$gen/Test750Semantics" "$gen/TestEmitSyntaxFeatures" "$gen/TestEmitStringRepeatMatch" "$gen/TestEmitRegExpExec"
(cd "$root" && ABAPITI_TEST_OUT="$gen" go test ./hir/abap -run '^(TestFixtures|Test750Semantics)$' -count=1)
for suite in TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec; do
  (cd "$root" && ABAPITI_TEST_OUT="$gen/$suite" go test ./tsfront -run "^$suite$" -count=1)
done
node "$root/.github/ci/hir-lint-test.mjs" "$osg"
node "$root/.github/ci/hir-lint.mjs" "$gen/TestFixtures" "$osg"
node "$root/.github/ci/hir-lint.mjs" "$gen/Test750Semantics" "$osg"
for suite in TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec; do
  node "$root/.github/ci/hir-lint.mjs" "$gen/$suite" "$osg" --fixture
done
node "$root/.github/ci/osg-transpiler.mjs" "$osg"
result=0
for runtime in osgo osgjs; do
  for suite in TestFixtures Test750Semantics TestEmitSyntaxFeatures TestEmitStringRepeatMatch TestEmitRegExpExec; do
    expected=7
    if [[ "$suite" != TestFixtures ]]; then expected=1; fi
    classes="$gen/$suite"
    status=0
    (cd "$osg" && GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false npm run -s "$runtime:unit" -- "$classes" --json) > "$work/hir-$runtime-$suite.json" 2> "$work/hir-$runtime-$suite.err" || status=$?
    node -e '
      const fs = require("fs");
      const d = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
      const t = d.totals, expected = Number(process.argv[4]);
      // The pinned osgo adapter explicitly refuses PCRE lookbehind. Do not
      // replace JS semantics with POSIX; OSG-JS remains mandatory for these
      // suites, and every other native failure still fails this gate.
      const regexSuite = ["TestEmitStringRepeatMatch", "TestEmitRegExpExec"].includes(process.argv[5]);
      const gap = "NOT_COMPILED in FIND PCRE: a negative lookbehind (?<!...) is not in Go\u0027s RE2:";
      if (regexSuite && process.argv[3].startsWith("osgo ") && Number(process.argv[2]) === 2 &&
          t.tests === 1 && t.not_compiled === 1 && t.success === 0 && t.failure === 0 && t.error === 0 &&
          d.rows.length === 1 && d.rows[0].status === "NOT_COMPILED" && d.rows[0].message.startsWith(gap)) {
        console.log(`${process.argv[3]} HIR: known pinned PCRE lookbehind gap (OSG-JS required)`);
        process.exit(0);
      }
      console.log(`${process.argv[3]} HIR: ${t.success}/${t.tests} passed`);
      if (Number(process.argv[2]) !== 0 || t.tests !== expected || t.success !== expected || d.rows.length !== expected || d.rows.some(r => r.status !== "SUCCESS")) process.exit(1);
    ' "$work/hir-$runtime-$suite.json" "$status" "$runtime $suite" "$expected" "$suite" || result=1
  done
done
node "$root/.github/ci/lexer-unit.mjs" "$work" || result=1
exit "$result"
