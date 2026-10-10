# Dynamic fit-gap measurement

Run from the repository worktree, with the pinned, clean upstream checkout at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e` and its installed dependencies:

```sh
TMPDIR=/mnt/ram/abapiti/tmp \
npm_config_cache=/mnt/ssd-510/osg/abapiti/npm-cache \
nice -n 10 node tools/fit-gap.mjs OUT INPUT DEPS_SRC CONFIG NEGATIVES \
  /home/alice/dev/abaplint
```

Arguments default to the task scratchpad's `gapdyn`, `zabapgit/in`,
`zabapgit/deps/src`, `zabapgit/cfg/ci-abaplint.json`, and `neg/negative.json`.
Node is resolved from PATH. The tool uses `buildUpstream()` to compile a fresh
original core into a disposable TMPDIR directory; it never modifies upstream.
No network requests are made. Run the parent with `nice -n 10` so the build is
also nice; workload children are independently launched with nice. TMPDIR defaults
to the RAM path when unset; an explicit TMPDIR can select an allowed writable
scratch directory if that mount is read-only.

SIX unions the same four components as `registry-coverage.mjs`: clean deployment,
negative variants, registry oracle observations, and selected six-rule/syntax
tests. Its symmetric difference against the embedded schema-2 executed spans
is recorded. ALL checks only the clean input using `Config.getDefault()` and
the SIX syntax version. TESTS runs every built `test/**/*.js` with Mocha's dot
reporter and timeout 1000000. Test assertion failures are counted and retained;
build, workload, timeout, and incomplete-test-run errors are fatal.

Outputs are `fit-gap.json`, `fit-gap.md`, logs, observation files, and separate
`v8/{SIX,ALL,TESTS}` directories. A rerun replaces the V8 directories. Do not
commit generated outputs. JSON includes all declarations, byte offsets, workload
booleans, test failures, mapping diagnostics, and SIX diff identities. Identity
is relative TS path plus declaration start offset; source-map matching uses
`declarationCandidates()`. The denominator includes unloaded declarations with
bodies, excluding arrow functions/function expressions as in registry coverage.
Ambiguous matches are not counted executed. Gap percentage means gap / needed.

For diagnosing an environment missing the workload inputs, explicitly pass
`--tests-only`. Such reports use null for SIX/ALL, prominently identify themselves
as partial, and report TESTS-only lower bounds for needed/gap. They are **not** a
measurement of ALL ∪ TESTS. Restore the exact inputs and rerun without that flag
for the requested complete measurement. The live count always comes from the
current worktree's embedded JSON, not from a previously built CLI binary.
