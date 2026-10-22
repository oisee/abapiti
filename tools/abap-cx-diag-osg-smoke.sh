#!/usr/bin/env bash
# ABAP-origin CX -> JS Error -> TS catch/rethrow -> OSGo uncaught diagnostics.
# usage: abap-cx-diag-osg-smoke.sh ABAPITI OSG OUT
set -euo pipefail
abapiti=$(realpath "${1:?usage: abap-cx-diag-osg-smoke.sh ABAPITI OSG OUT}")
osg=$(realpath "${2:?}")
out=${3:?}
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
mkdir -p "$out"
out=$(realpath "$out")
project=$out/project
mkdir -p "$project"
cp "$root"/cmd/abapiti/testdata/abap-cx-diag/* "$project/"
sed -i 's|"goDir":"[^"]*"|"goDir":"'"$root"'/extern/hostio"|g' "$project/externs.json"
cd "$project"
env -u ABAPITI_CX_MODE node main.ts > "$out/node-stdout.txt"
"$abapiti" emit-abap --tsconfig tsconfig.json --externs externs.json --initialize initialization.json \
  -o "$out/abap" main.ts host.ts cx.ts
cp ./*.clas.abap "$root/extern/hostio/abap/zcl_abapiti_hostio.clas.abap" "$out/abap/src/"
printf '{"version":1,"dirs":{},"env":{}}\n' > "$out/fs-snapshot.json"
node "$root/tools/selfhost-step4b-osg.mjs" --build --main \
  --snapshot "$out/fs-snapshot.json" --stdout "$out/stdout.txt" "$out/abap/src" "$out/osg" "$osg"
for mode in rethrow dynamic uncaught chain raw js; do
  node --input-type=module - "$out/fs-snapshot.json" "$mode" <<'JS'
import {writeFileSync} from 'node:fs';
writeFileSync(process.argv[2], JSON.stringify({version:1,dirs:{},env:{ABAPITI_CX_MODE:process.argv[3]}}));
JS
  status=0
  OSD_DATASET_HOME="$project" OSD_DATASET_READ="$out" OSD_DATASET_WRITE="$out" \
    "$out/osg/step4b-osg" 2> "$out/$mode.log" || status=$?
  test "$status" -eq 3
  head -n 1 "$out/stdout.txt" > "$out/js-message.txt"
  cmp "$out/node-stdout.txt" "$out/js-message.txt"
  if [ "$mode" = js ]; then
    grep -q 'step4b: exception: .*uncaught JS message unchanged' "$out/js.log"
    if grep -q 'ABAP origin:' "$out/js.log"; then
      echo "abap-cx-diag: JS Error acquired an ABAP origin" >&2
      exit 1
    fi
    echo "abap-cx-diag: js message matches Node (exit 3)"
    continue
  fi
  grep -q 'class=CX_SY_ZERODIVIDE text=.*CX_SY_ZERODIVIDE' "$out/$mode.log"
  grep -q 'position=unavailable: .*GET_SOURCE_POSITION' "$out/$mode.log"
  case "$mode" in
    rethrow)
      grep -q '^caught: CX_SY_ZERODIVIDE: ' "$out/stdout.txt"
      grep -q 'edited JS message' "$out/rethrow.log"
      grep -q 'ABAP origin: class=CX_SY_ZERODIVIDE' "$out/rethrow.log" ;;
    dynamic|uncaught)
      grep -q 'step4b: exception: .*CX_SY_ZERODIVIDE: ' "$out/$mode.log"
      grep -q 'ABAP origin: class=CX_SY_ZERODIVIDE' "$out/$mode.log" ;;
    chain|raw)
      if [ "$mode" = chain ]; then
        grep -q 'abap_program=ZDIAG_PROGRAM; abap_include=ZDIAG_INCLUDE' "$out/chain.log"
        grep -q 'outer host text @ ZDIAG_PROGRAM/ZDIAG_INCLUDE:42' "$out/chain.log"
      fi
      grep -q 'class=ZCX_ABAPITI_DIAG text=outer host text position=ZDIAG_PROGRAM/ZDIAG_INCLUDE:42' "$out/$mode.log"
      grep -q 'previous: class=ZCX_ABAPITI_DIAG text=inner host text' "$out/$mode.log"
      grep -q 'previous: class=CX_SY_ZERODIVIDE' "$out/$mode.log" ;;
  esac
  echo "abap-cx-diag: $mode diagnostics passed (exit 3)"
done
