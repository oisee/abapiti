# abaplint 2.120.73

The embedded core is pinned to upstream tag `v2.120.73`, commit
`9f7f098c21e93b8b534fff0f0148f0a900d29d16`. The deterministic archive contains
1,548 SHA-256-verified TypeScript sources plus LICENSE. Active CI, OCI, release
assets and usage documentation use this pin; dated measurements keep their
original pins.

## Verification

- `go build ./...`, `go vet ./...`, and `go test -short -timeout 30m ./...`: pass.
- Original Node, TS-HG and TS-HA@OSGO: **11,488 active tests passed**, 0 failed,
  0 trapped, 0 unregistered. All 635 files / 17 shards; 32 Node-pending cases
  remain outside the active denominator. Whole-suite title multiplicity and
  disjoint root coverage are checked.
- OSG: clean `.github/ci/osgo.ref` checkout at
  `0171e92b2974854153a5ab4d645f0e53b6f27092`.
- M1: 2,785 files, **82 issues** on all three hosts, byte-identical. Node .73
  adds two generic DataReference diagnostics to the previous 80-issue result.
- M2: the freshly emitted .73 core, **4,640 files + 716 dependencies**,
  **0 issues** on all three hosts, byte-identical.

Reference provenance and expected output are in
[`tsfront/testdata/selfhost-73`](../tsfront/testdata/selfhost-73).

## Implementation

The override audit retained 108 unchanged entries, removed three obsolete
entries and reviewed/re-fingerprinted three changed spans. Two singleton
constructor certificates guard NGenericType and its NumericType dependency.
The default full translation applies 110 fingerprinted entries.

General fixes cover collection inference, safe lazy regex quantifiers, dynamic
replacement callbacks, finite variable division, optional-string sorting, typed
class copying, regex lastIndex updates, erased reference assertions, absent
indexed reads, regex syntax errors and literal lookahead exec. All-unit PCRE
spelling is portable across the tested hosts. Dynamic zero is returned unchanged
by ABAP Math.ceil; negative fractions outside its supported signed-zero domain
still refuse. Native multifile loading preserves dependency paths.

The default translation has 2,079 HIR classes / 79 interfaces, 0 blocking
diagnostics and 0 HIR errors; emitted core is 2,317 ABAP objects. Five preexisting
source-located refusal bodies remain unexecuted by these gates.

All gates used 16 GiB scopes, with rules at 24 GiB. Frontend emission is isolated
from compilation and the memory-heavy optional per-object cache is off by
default; see [the current OSG build profile](osg-harness-speed.md#current-bounded-builds).
Step 4 and current-version SAP kernel certification were outside this task.
