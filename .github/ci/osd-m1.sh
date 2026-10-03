#!/usr/bin/env bash
# Compile the OSD modules listed in wasm/osd_unit_test.go to ABAP, deploy each
# class and its generated ABAP Unit test class to a running OSD with vsp, and
# run the tests. Every generated class is deployed; none is listed here.
# The expected values in the test classes come from wazero, not from abapiti.
#
#   SAP_URL=http://localhost:3030 .github/ci/osd-m1.sh <workdir>
#
# vsp runs from an empty directory with every inherited SAP_*/VSP_* variable
# removed, so it can only reach $SAP_URL as the OSD user DEVELOPER. The vsp
# binary is the pinned release (vsp.version), checked against the committed
# vsp.sha256 and the release's checksums.txt. Package $ZOSD_TEST_SRC (OSD
# up to 0.6.1511 had no $TMP).
set -euo pipefail

work=${1:?usage: osd-m1.sh <workdir>}
url=${SAP_URL:?SAP_URL must point at the OSD}
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
pkg='$ZOSD_TEST_SRC'

mkdir -p "$work"/{vsp,gen,empty,log}
work=$(cd "$work" && pwd)

# The pinned vsp.
tag=$(tr -d '[:space:]' < "$here/vsp.version")
asset=vsp-linux-amd64
pinned=$(awk -v k="$tag/$asset" '$2 == k { print $1 }' "$here/vsp.sha256")
[ -n "$pinned" ] || { echo "osd-m1: no committed sha256 for $tag/$asset" >&2; exit 4; }
if [ -n "${VSP_DL_DIR:-}" ]; then
  cp "$VSP_DL_DIR/$asset" "$VSP_DL_DIR/checksums.txt" "$work/vsp/"
else
  gh release download "$tag" -R oisee/vibing-steampunk -p "$asset" -p checksums.txt -D "$work/vsp" --clobber
fi
actual=$(sha256sum "$work/vsp/$asset" | cut -d' ' -f1)
release=$(awk -v a="$asset" '$2 == a { print $1 }' "$work/vsp/checksums.txt")
if [ "$actual" != "$pinned" ] || [ "$actual" != "$release" ]; then
  echo "osd-m1: $asset sha256 $actual does not match the pin ($pinned) and checksums.txt ($release); refusing" >&2
  exit 4
fi
chmod +x "$work/vsp/$asset"

# vsp gets an allow-listed environment: no inherited SAP_*/VSP_* (a
# developer's shell may point at a real system), no GH_TOKEN, and its own HOME,
# so no ~/.vsp.json or .env can redirect it.
mkdir -p "$work/vsp-home"
vsp() {
  (cd "$work/empty" &&
    env -i PATH="$PATH" HOME="$work/vsp-home" TMPDIR="${TMPDIR:-/tmp}" \
      SAP_URL="$url" SAP_USER=DEVELOPER SAP_PASSWORD=osd SAP_CLIENT=001 \
      "$work/vsp/$asset" "$@")
}
echo "osd-m1: vsp $tag as DEVELOPER on $url" >&2

# Generate the classes and their test classes.
(cd "$root" && ABAPITI_TEST_OUT="$work/gen" go test ./wasm -run '^TestOSD_EmitUnitClasses$' -count=1 -v) \
  > "$work/log/generate.log" 2>&1 || { cat "$work/log/generate.log" >&2; exit 1; }
gen="$work/gen/TestOSD_EmitUnitClasses"
# Flatten the same fixture sets consumed by osgo.
cp "$gen"/split/*.abap "$gen/"
classes=()
for f in "$gen"/*.clas.abap; do
  [ -e "$f" ] || continue
  b=$(basename "$f" .clas.abap)
  if [[ "$b" != *_st && ! "$b" =~ _c[0-9]+$ ]]; then
    [ -f "$gen/$b.clas.testclasses.abap" ] || { echo "osd-m1: $b has no test class" >&2; exit 1; }
  fi
  classes+=("$b")
done
[ "${#classes[@]}" -gt 0 ] || { echo "osd-m1: the generator wrote no classes" >&2; exit 1; }

# OSD_SHARD of OSD_SHARDS: a fixture (its facade, state class, chunks and
# interfaces) stays in one shard; fixtures are dealt round-robin by name.
shards=${OSD_SHARDS:-1}
shard=${OSD_SHARD:-1}
[[ "$shards" =~ ^[1-9][0-9]*$ && "$shard" =~ ^[1-9][0-9]*$ && "$shard" -le "$shards" ]] ||
  { echo "osd-m1: bad shard $shard of $shards" >&2; exit 1; }
fixture() { local n=${1/#zif_/zcl_}; n=${n%_st}; [[ "$n" =~ ^(.*)_c[0-9]+$ ]] && n=${BASH_REMATCH[1]}; echo "$n"; }
mapfile -t fixtures < <(for c in "${classes[@]}"; do fixture "$c"; done | sort -u)
declare -A mine=()
for i in "${!fixtures[@]}"; do
  if [ $((i % shards + 1)) -eq "$shard" ]; then mine[${fixtures[$i]}]=1; fi
done
picked=()
for c in "${classes[@]}"; do if [ -n "${mine[$(fixture "$c")]:-}" ]; then picked+=("$c"); fi; done
classes=("${picked[@]}")
[ "${#classes[@]}" -gt 0 ] || { echo "osd-m1: shard $shard of $shards has no classes" >&2; exit 1; }
echo "osd-m1: shard $shard of $shards: ${#classes[@]} classes: ${classes[*]}" >&2

# Interfaces activate first, then state, independent chunks and facades.
for f in "$gen"/*.intf.abap; do
  [ -e "$f" ] || continue
  [ -n "${mine[$(fixture "$(basename "$f" .intf.abap)")]:-}" ] || continue
  if ! vsp deploy "$f" "$pkg" --call-timeout 900 > "$work/log/deploy-$(basename "$f").log" 2>&1; then
    cat "$work/log/deploy-$(basename "$f").log" >&2
    exit 1
  fi
done
ordered=()
for c in "${classes[@]}"; do if [[ "$c" == *_st ]]; then ordered+=("$c"); fi; done
for c in "${classes[@]}"; do if [[ "$c" =~ _c[0-9]+$ ]]; then ordered+=("$c"); fi; done
for c in "${classes[@]}"; do
  if [[ "$c" != *_st && ! "$c" =~ _c[0-9]+$ ]]; then ordered+=("$c"); fi
done
classes=("${ordered[@]}")

# Deploy and test. Any failure stops the run: on OSD up to 0.6.1511 a failed
# activation poisons every later one.
fail=0
for c in "${classes[@]}"; do
  for f in "$c.clas.abap" "$c.clas.testclasses.abap"; do
    [ -f "$gen/$f" ] || continue
    if ! vsp deploy "$gen/$f" "$pkg" --call-timeout 900 > "$work/log/deploy-$f.log" 2>&1; then
      echo "osd-m1: deploy of $f failed" >&2
      cat "$work/log/deploy-$f.log" >&2
      exit 1
    fi
  done
  [ -f "$gen/$c.clas.testclasses.abap" ] || continue
  # The exit status alone is not enough: the run must report exactly as many
  # passed tests as the generated test class has methods.
  want=$(grep -cE '^    METHODS c[0-9]+ FOR TESTING\.$' "$gen/$c.clas.testclasses.abap" || true)
  if [ "$want" -gt 0 ] && vsp test CLAS "${c^^}" > "$work/log/test-$c.log" 2>&1 &&
    grep -qx "Total: $want passed, 0 failed" "$work/log/test-$c.log"; then
    echo "osd-m1: $c: $want passed" >&2
  else
    echo "osd-m1: $c FAILED" >&2
    cat "$work/log/test-$c.log" >&2
    fail=1
  fi
done
exit $fail
