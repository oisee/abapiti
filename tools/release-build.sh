#!/usr/bin/env bash
# Offline release preparation only. Never tags, pushes, or contacts SAP.
# Caller supplies the pinned toolchain/dependencies and admits this whole script
# under the heavy-job lock + 48 GiB scope (do not nest that lock).
set -euo pipefail
if [ "$#" != 5 ]; then
  echo 'usage: release-build.sh ABAPLINT OSG KIT_ZIP ASSETS WORK' >&2
  exit 2
fi
upstream=$(realpath "$1"); osg=$(realpath "$2"); kit=$(realpath "$3")
mkdir -p "$4" "$5"
assets=$(realpath "$4"); work=$(realpath "$5")
root=$(git rev-parse --show-toplevel)
cd "$root"
version=0.5.0
export GOFLAGS=-buildvcs=false GOTOOLCHAIN=go1.26.0 CGO_ENABLED=0 GOMAXPROCS=8
export NODE_OPTIONS=--max-old-space-size=24576
# The native compiler's generated report is large. This bounded mode is also
# used by a4h-zabapgit-check.mjs, not optimized against an unoptimized baseline.
platforms=(darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64)
# Resume only after successful emission, with the same sources/pins/WORK.
if [ "${RELEASE_RESUME:-}" != packages ]; then
for platform in "${platforms[@]}"; do
  ext=; [[ "$platform" == windows-* ]] && ext=.exe
  GOOS=${platform%-*} GOARCH=${platform#*-} go build -p 2 -trimpath -ldflags="-s -w -X main.version=$version" -o "$assets/abapiti-$platform$ext" ./cmd/abapiti
done
"$assets/abapiti-linux-amd64" abaplint "$upstream" -o "$work/default" --target abap,go --evidence
# The release A4H pair must be emitted together, not made by mixing separate
# core and test name/shape tables. This is build only, with no kernel access.
node tools/a4h-tests-build.mjs "$upstream" "$work/a4h"
fi
node tools/a4h-tests-lint.mjs "$work/a4h" "$osg"
cp "$work/a4h/zlnt.zip" "$assets/abaplint-577f875e-a4h.zip"
cp "$work/a4h/zlnt_tests.zip" "$assets/abaplint-577f875e-a4h-tests.zip"
cp "$work/a4h/zlnt.manifest.json" "$assets/abaplint-577f875e-a4h.manifest.json"
cp "$work/a4h/zlnt_tests.manifest.json" "$assets/abaplint-577f875e-a4h-tests.manifest.json"
cp "$work/a4h/names.json" "$assets/abaplint-577f875e-names.json"
cp "$kit" "$assets/zabapgit-check-kit.zip"
mkdir -p "$work/kit"
unzip -q -o "$kit" -d "$work/kit"
# External OSG build root: no generated outputs in the shared OSG checkout.
node tools/a4h-zabapgit-check.mjs "$work/a4h" "$osg" "$work/kit/zabapgit-check-kit" "$work/osgo"
for platform in "${platforms[@]}"; do
  ext=; [[ "$platform" == windows-* ]] && ext=.exe
  (cd "$work/default/go" && GOOS=${platform%-*} GOARCH=${platform#*-} go build -trimpath -ldflags='-s -w' -o "$assets/zabaplint-go-$platform$ext" .)
  (cd "$work/osgo/native/go" && GOOS=${platform%-*} GOARCH=${platform#*-} go build -p 1 -tags nodatabase -trimpath -ldflags='-s -w' '-gcflags=osg/gogen/cmd/osabap=-N -l -dwarf=false' -o "$assets/zabaplint-$platform$ext" ./cmd/osabap)
done
# Copy, do not run checks in the source kit directory.
sh "$work/kit/zabapgit-check-kit/check.sh" "$assets/zabaplint-go-linux-amd64"
node tools/hir-go-check.mjs "$work/kit/zabapgit-check-kit" "$assets/zabaplint-go-linux-amd64" "$work/fresh-kit" '' "$upstream"
node tools/release-rules-check.mjs "$upstream" "$work/rules" "$assets/zabaplint-go-linux-amd64" "$assets/zabaplint-linux-amd64"
# Corpus is the unchanged checked-in abapGit repository (sources + metadata).
(cd tools/allbackends/corpus-abap && zip -q -r "$assets/abapiti-corpus-a4h.zip" .)
for platform in "${platforms[@]}"; do
  ext=; [[ "$platform" == windows-* ]] && ext=.exe
  bundle="$work/bundles/abaplint-abap-$platform"
  mkdir -p "$bundle"
  cp "$assets/abapiti-$platform$ext" "$bundle/abapiti$ext"
  cp "$assets/zabaplint-$platform$ext" "$bundle/zabaplint$ext"
  cp "$assets/zabaplint-go-$platform$ext" "$bundle/zabaplint-go$ext"
  cp -a "$work/kit/zabapgit-check-kit" "$bundle/"
  rm -f "$bundle"/zabapgit-check-kit/actual-*.txt
  printf '%s\n' "abapiti $version; abaplint 577f875e (2.120.56), FULL core / all rules." \
    'Run zabapgit-check-kit/check.sh (POSIX) or check.ps1 (Windows).' \
    'zabaplint = TS-HA@OSGO; zabaplint-go = TS-HG@Go.' \
    'A4H kernel certification is in progress; no kernel parity claim.' > "$bundle/README.txt"
  if [[ "$platform" == windows-* ]]; then
    (cd "$work/bundles" && zip -q -r "$assets/abaplint-abap-$platform.zip" "abaplint-abap-$platform")
  else
    tar -czf "$assets/abaplint-abap-$platform.tar.gz" -C "$work/bundles" "abaplint-abap-$platform"
  fi
done
(cd "$assets" && find . -maxdepth 1 -type f ! -name SHA256SUMS -printf '%f\n' | LC_ALL=C sort | xargs sha256sum > SHA256SUMS && sha256sum -c SHA256SUMS)
