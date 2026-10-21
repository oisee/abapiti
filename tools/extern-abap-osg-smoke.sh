#!/usr/bin/env bash
# OSGo smoke test of the ABAP extern host: cmd/abapiti/testdata/extern-abap
# (reads a file with invalid UTF-8, lists snapshot directories, path, env,
# git HEAD, writes a file and stdout) runs under Node with the fs-snapshot
# recorder, then as ABAP (emit-abap --initialize + ZCL_ABAPITI_HOSTIO) built
# and run on OSGo. Node's stdout and the ABAP stdout file must be equal.
# usage: extern-abap-osg-smoke.sh ABAPITI-BINARY OSG OUT
# OSG must be clean at .github/ci/osgo.ref; OUT must be outside it.
set -euo pipefail
abapiti=$(realpath "${1:?usage: extern-abap-osg-smoke.sh ABAPITI OSG OUT}")
osg=$(realpath "${2:?usage: extern-abap-osg-smoke.sh ABAPITI OSG OUT}")
out=${3:?usage: extern-abap-osg-smoke.sh ABAPITI OSG OUT}
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
# Shared builder hosts: start only below a load of 12.
while [ "$(cut -d' ' -f1 /proc/loadavg | cut -d. -f1)" -ge 12 ]; do sleep 30; done
rm -rf "$out"
mkdir -p "$out"
out=$(realpath "$out")
project=$out/project
cp -r "$root/cmd/abapiti/testdata/extern-abap" "$project"
sed -i 's|"goDir":"[^"]*"|"goDir":"'"$root"'/extern/hostio"|' "$project/externs.json"
mkdir -p "$project/data/sub/inner" "$project/data/empty" "$project/out"
printf 'h\xc3\xa9llo \xf0\x9f\x98\x80 bad:\xff\xc3( \xe0\x80\xaf \xed\xa0\x80 \xf4\x90\x80\x80 \xe2\x82 end\n' > "$project/data/in.txt"
printf 'b' > "$project/data/sub/b.txt"
printf 'a' > "$project/data/sub/a.txt"
git -C "$project" init -q
git -C "$project" add -A
git -C "$project" -c user.name=smoke -c user.email=smoke@example.invalid -c commit.gpgsign=false commit -qm smoke
cd "$project"
ABAPITI_SMOKE=on FS_SNAPSHOT_OUT="$project/fs-snapshot.json" FS_SNAPSHOT_STDOUT="$out/abap-stdout.txt" \
  node --import "$root/tools/fs-snapshot-recorder.mjs" main.ts > "$out/node-stdout.txt"
rm -f out/copy.txt
"$abapiti" emit-abap --tsconfig tsconfig.json --externs externs.json --initialize initialization.json \
  -o "$out/abap" main.ts state.ts host.ts
cp "$root/extern/hostio/abap/zcl_abapiti_hostio.clas.abap" "$out/abap/src/"
node "$root/tools/selfhost-step4b-osg.mjs" --build --main "$out/abap/src" "$out/osg" "$osg"
status=0
OSD_DATASET_HOME="$project" OSD_DATASET_READ="$project:$out" OSD_DATASET_WRITE="$project/out:$out" "$out/osg/step4b-osg" 2> "$out/run.log" || status=$?
if [ "$status" -ne 0 ]; then
  echo "extern-abap-smoke: run failed (exit $status): $(head -c 400 "$out/run.log")"
  exit 1
fi
if cmp -s "$out/node-stdout.txt" "$out/abap-stdout.txt"; then
  echo "extern-abap-smoke: equal ($(wc -l < "$out/node-stdout.txt") lines)"
else
  echo "extern-abap-smoke: different"
  diff "$out/node-stdout.txt" "$out/abap-stdout.txt" | head -20 || true
  exit 1
fi
