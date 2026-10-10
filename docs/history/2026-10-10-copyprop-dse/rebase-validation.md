# Rebase validation

Base: `f56ae28d4929d0d16ddd1cc6bc6568a74f885f9e` (`origin/main`).
Kit: `~/.cache/kits/abaplint-577f875e`, all 1,538 pinned closure files.

Conflicts in `hir/abap/emit.go`, `hir/golang/emit.go`,
`hir/rewrite/demand.go`, `hir/rewrite/rewrite.go`, and later
`hir/rewrite/store.go` retained main's readable names, profiling emitter,
stable site assignment and public Grace API. Both emitters call the shared
store pass, which runs only when `ABAPITI_COPYPROP=1`. Go stamps identities
before this pass. Grace accepts the two store actions and the adapter uses
its public demand, fact and matcher APIs.

The following commands were run from the repository root. `--target all`
compares every ABAP target, including the reproducible A4H zip; `--target go`
compares Go code, the executable, profiler variant and `sites.json`.
Both builds emit at the same absolute path, then their complete directories are
moved aside before comparison. This keeps path-bearing README files identical
without normalizing or excluding any files. The initial distinct-output-path
ABAP comparison differed only in `osg/README.txt` and `native/README.txt`.

```bash
mkdir -p /tmp/copyprop-main
git archive origin/main | tar -x -C /tmp/copyprop-main
(cd /tmp/copyprop-main && env GOMAXPROCS=4 go build -buildvcs=false -o /tmp/copyprop-main/abapiti ./cmd/abapiti)
env GOMAXPROCS=4 go build -buildvcs=false -o .local/rebase-validation/abapiti ./cmd/abapiti
export GOMAXPROCS=4 GOFLAGS=-buildvcs=false
validation="$PWD/.local/rebase-validation"
kit="$HOME/.cache/kits/abaplint-577f875e"
for target in all go; do
 env ABAPITI_COPYPROP=0 /tmp/copyprop-main/abapiti abaplint "$kit" --target "$target" -o "$validation/emitted-$target" > "$validation/main-$target.log" 2>&1
 mv "$validation/emitted-$target" "$validation/main-matched-$target"
 env ABAPITI_COPYPROP=0 "$validation/abapiti" abaplint "$kit" --target "$target" -o "$validation/emitted-$target" > "$validation/off-$target.log" 2>&1
 mv "$validation/emitted-$target" "$validation/off-matched-$target"
 diff -r "$validation/main-matched-$target" "$validation/off-matched-$target" > "$validation/diff-$target.log"
 printf 'diff -r %s: byte-identical\n' "$target"
done
env ABAPITI_COPYPROP=1 "$validation/abapiti" abaplint "$kit" --target all -o "$validation/on-all" > "$validation/on-all.log" 2>&1
env ABAPITI_COPYPROP=1 "$validation/abapiti" abaplint "$kit" --target go -o "$validation/on-go" > "$validation/on-go.log" 2>&1
python3 tools/go-target-check.py "$HOME/dev/dell-work/kits/zabapgit-check-kit" "$validation/off-matched-go/go/zabaplint-go" --runs 3 --output "$validation/kit-off.json"
python3 tools/go-target-check.py "$HOME/dev/dell-work/kits/zabapgit-check-kit" "$validation/on-go/go/zabaplint-go" --runs 3 --output "$validation/kit-on.json"
```

Additional validation commands:

```bash
env ABAPITI_COPYPROP=0 GOMAXPROCS=4 go vet ./...
env ABAPITI_COPYPROP=1 GOMAXPROCS=4 go vet ./...
env ABAPITI_COPYPROP=0 GOMAXPROCS=4 go test -json -short ./...
env ABAPITI_COPYPROP=1 GOMAXPROCS=4 go test -json -short ./...
env ABAPITI_COPYPROP=1 GOMAXPROCS=4 go test -v ./tsfront -run '^TestGoLexerDifferential$' -count=1
env GOMAXPROCS=4 GRACE_COUNTS_MEASURE_STORES=1 go run ./cmd/grace-counts -output .local/rebase-validation/counts
```

The explicit lexer differential uses the compiled Node oracle at
`/home/alice/dev/abaplint/packages/core/build/src`. It passed 44/44 and rejected
its token-type mutation. Both vet modes passed. Store counts remain Go
116 copies / 655 DSE and ABAP 118 copies / 961 DSE, three rounds each.
The refreshed ABAP totals are 493,853 → 491,697; the 58 hot methods total
6,702 → 6,682. Historical performance samples remain labelled as pre-rebase.

Results are in [rebase-validation.json](rebase-validation.json).
Both `diff -r` commands returned 0 with empty output: the complete ABAP and
Go trees are byte-identical. Short tests passed in both modes: 1,129 passing
cases and 21 skips each (391 passing top-level tests, 19 skips; 18 passing
packages and 50 packages without tests), zero failures.

All twelve kit runs passed: three clean and three seeded runs per flag mode,
with raw bytes matching the release-kit expectations (zero and five issues).
Fresh timing samples are retained as correctness-run evidence on a shared
host; they do not establish a performance change.

The full Go `sites.json` files contain 38,198 occurrences and 38,191 unique
IDs in both modes. Flag-off JSON matches main exactly. Flag-on preserves every
ID, kind, source, method and inline path; generated locations may move.
No removed kit stores contain operations represented in this site map.
`TestCopyPropSiteIDs` proves the removal case separately: two operations with
the same source position become one, and the survivor retains its original ID.
The full-kit identity comparison was:

```python
import json
from collections import Counter
from pathlib import Path
root = Path(".local/rebase-validation")
main = json.loads((root / "main-matched-go/sites.json").read_text())
off = json.loads((root / "off-matched-go/sites.json").read_text())
on = json.loads((root / "on-go/sites.json").read_text())
assert main == off
key = lambda s: (s["site_id"], s["kind"], s["source"], s["method"], tuple(s["inline_path"]))
before = Counter(map(key, off["sites"]))
after = Counter(map(key, on["sites"]))
assert not after - before
```

During push verification, main advanced from `f9d5f8c` to `f56ae28` with
#85 (root README and glossary only). The branch was rebased again without
conflicts. Rebuilding main and the branch with `-buildvcs=false` produced the
same binary SHA-256 values as before this docs-only update, so the completed
emission and test results remain applicable to the final base:

- Main: `eca11a5a853821e9e51f494b8cb4578cd1ce454caf01ebf931bc1fa09a3fdd64`.
- Branch: `cfa9670848b5496de7a37e575966f56da7333edccb7e499eecec741f34e27e85`.

```bash
git fetch origin
git rebase origin/main
git archive origin/main | tar -x -C /tmp/copyprop-main
env GOMAXPROCS=4 go -C /tmp/copyprop-main build -buildvcs=false -o /tmp/copyprop-main/abapiti ./cmd/abapiti
env GOMAXPROCS=4 go build -buildvcs=false -o .local/rebase-validation/abapiti ./cmd/abapiti
sha256sum /tmp/copyprop-main/abapiti .local/rebase-validation/abapiti
```
