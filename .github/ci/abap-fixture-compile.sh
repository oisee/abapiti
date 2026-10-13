#!/usr/bin/env bash
# Read-only use of the already prepared, pinned OSG checkout. No clones/setup.
set -euo pipefail
osg=${1:?usage: abap-fixture-compile.sh <OSG checkout> <output directory>}
work=${2:?usage: abap-fixture-compile.sh <OSG checkout> <output directory>}
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
mkdir -p "$work"
work=$(cd -- "$work" && pwd)
osg=$(cd -- "$osg" && pwd)
# A fresh directory and -count=1 prevent stale exports or cached test success.
scratch=$(mktemp -d "$work/run.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
export ABAPITI_ABAP_FIXTURE_DIR="$scratch/fixtures"
(cd "$root" && go test -p 2 -count=1 -timeout 25m ./...)
# The legacy pipeline is dispatch-only, but its ABAP Unit generator is cheap
# and its emitted test classes must compile too (not run here).
(cd "$root" && go test -tags wasmlegacy -p 2 -count=1 -timeout 5m ./wasm -run '^TestOSD_EmitUnitClasses$')
status=0
node "$root/.github/ci/abap-fixture-compile.mjs" "$ABAPITI_ABAP_FIXTURE_DIR" "$scratch/compile" "$osg" || status=$?
# Keep only diagnostic artifacts, even on failure. Generated ABAP/Go never
# enter the checkout. The fresh scratch tree also prevents stale Go packages.
mkdir -p "$work/compile"
for file in fixtures.json build.log; do
  if [ -f "$scratch/compile/$file" ]; then cp "$scratch/compile/$file" "$work/compile/"; fi
done
exit "$status"
