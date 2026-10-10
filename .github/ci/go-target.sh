#!/usr/bin/env bash
# Pinned upstream has already been built; missing/skipped oracles fail closed.
set -euo pipefail
work=${1:?usage: go-target.sh WORK KIT}
kit=${2:?usage: go-target.sh WORK KIT}
mkdir -p "$work"
work=$(cd "$work" && pwd)
export GOFLAGS=-buildvcs=false
: "${TSFRONT_ABAPLINT:?set TSFRONT_ABAPLINT to the built pinned packages/core}"
go build -o "$work/abapiti" ./cmd/abapiti
"$work/abapiti" abaplint --target go -o "$work/out"
go test -count=1 -timeout 8m -v ./tsfront \
  -run '^(TestGoLexerDifferential|TestGoRegistryJSON|TestGoRegistryXML|TestEmitRegistry(Arrays|Sorts|Iterators|Features))$' \
  | tee "$work/oracles.log"
python3 - "$work/oracles.log" <<'PY'
import sys
text = open(sys.argv[1]).read()
if '44/44 equal; token type mutation rejected' not in text:
    raise SystemExit('lexer differential did not compare all 44 cases')
for name in ('TestGoRegistryJSON', 'TestGoRegistryXML', 'TestEmitRegistryArrays/Go',
             'TestEmitRegistrySorts/Go', 'TestEmitRegistryIterators/Go', 'TestEmitRegistryFeatures/Go'):
    if '--- PASS: ' + name + ' (' not in text:
        raise SystemExit('missing registry oracle: ' + name)
PY
python3 tools/go-target-check.py "$kit" "$work/out/go/zabaplint" --output "$work/check.json"
