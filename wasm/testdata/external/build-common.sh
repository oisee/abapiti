#!/usr/bin/env bash
# Sourced by the build scripts after they create a temporary directory.
external_dir=$(cd .. && pwd)
cache="$external_dir/.cache"
output=${ABAPITI_EXTERNAL_DIR:-"$external_dir/wasm"}
mkdir -p "$cache" "$output"
output=$(cd "$output" && pwd)

download() {
  local name=$1 url=$2 checksum=$3
  if [[ ! -f "$cache/$name" ]]; then
    curl --fail --location --retry 3 "$url" -o "$tmp/$name"
    printf '%s  %s\n' "$checksum" "$tmp/$name" | sha256sum --check
    mv "$tmp/$name" "$cache/$name"
  fi
  # Verify cached archives too, before extracting anything.
  printf '%s  %s\n' "$checksum" "$cache/$name" | sha256sum --check
}

module_checksum() {
  local module=$1 expected=$2 actual
  actual=$(sha256sum "$module" | cut -d ' ' -f 1)
  printf 'SHA256 %s: %s\n' "$(basename "$module")" "$actual"
  if [[ "$actual" == "$expected" ]]; then
    printf 'Matches scratch SHA256 %s\n' "$expected"
  else
    printf 'WARNING: differs from scratch SHA256 %s (toolchains/build metadata may differ)\n' "$expected" >&2
  fi
}
