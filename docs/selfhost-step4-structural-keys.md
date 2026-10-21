# Step 4: resolved structural keys

Validated 2026-10-17, compiler base `b992f16a`, compiler commits `c95bc7ae`
and `466b6bc2`. The supplied OSG checkout is `1f6b7ef4`; #803 was tested in
separate private copies at `3b378469` and its parent `e718aa49`.

## Trap lists

| Checkout | Reached before → after | Unreached before → after | Unknown before → after |
| --- | --- | --- | --- |
| Supplied OSG | 134 → 130 | 291 → 290 | 1/39 → 1/39 |
| #803 parent e718aa49 | 139 → 135 | 307 → 306 | 3/39 → 3/39 |
| #803 3b378469 | 144 → 135 | 306 → 305 | 3/39 → 3/39 |

The #803 change added five reached locations before this train
(`frontend.mjs:227:39`, `227:147`, `228:66`, `230:47`, `231:51`). After the
train, its raw reached-location set has **zero new locations** versus its
parent. No location normalization was used. All six lists have the expected
Node reference SHA-256
`b90648781967b2cf3eaa4787f6dc9d41618682c26d18092e7608db18cfff5261`.

Run each checkout with both compiler binaries:

```sh
python3 -I tools/selfhost-step4.py --list-traps --expect-ref b9064878 \
  "$BIN" "$OSG" /home/alice/dev/abaplint /home/alice/dev/transpiler "$OUT"
```

Complete measurements, raw-list checksums and location-set differences are in
[selfhost-step4-structural-keys.json](selfhost-step4-structural-keys.json).
Raw logs, generated trees and binaries are retained under
`/mnt/ssd-510/osg/abapiti/tmp/step4-keys` (`before`, `803-base-before`,
`803-before`, `complete-after`, `complete-803-base-after`, `complete-803-after`).

## Red oracles and fixes

`TestStructuralKeysValueExprNodeOracle` first failed with constructor argument
`array<classref<family.shape…>> <- array<classref<shape…>>`.
`TestStructuralKeysFrontendParamNodeOracle` first failed with required-property
`classref<shape…> <- FrontendParam`. `red-final.log` retains both failures.
`TestStructuralKeysFrontendProgramRecoveryNodeOracle` first failed because a
closure resolved an unsupported initializer's declared local to an unrelated
module global (`red-program-recovery2.log`). Their Go/Node comparisons now pass.
The separate Program/Omit/Mutable oracle and resolved-field invariants also pass.

Intersections and generic data records now key resolved fields and member
types. Mapped readonly removal and nested record aliases participate in those
keys. Intersections register as structural views; required-property adaptation
requires an effect-safe dominating presence guard. The recovery fix keeps the
declared local binding and its explicit initializer trap, preventing a later
same-name module fallback. The five #803 Program callback mismatches and the
Param/default-value mismatches are absent from the final emitted sources.

## Gates

- `go vet ./...`: pass.
- `go test ./tsfront/ ./hir/... ./cmd/abapiti/ ./internal/abapfixture/ -short -p 2 -timeout 40m`: pass.
- Generated abaplint Go assignment traps: **2 before, 2 after**.
- M1 Go multifile output: byte-equal to `expected-clean.txt`; SHA-256
  `95d37bc666403557ceecbb6f0e12e31867c18d3c2870e105e3ceafb5b9267428`.
- `tools/layout-rename-check.py`: abaplint **8 files, 0 renames, 0 differences**;
  zabapgit default **7 files, 0 renames, 0 differences**. The tool infers a
  consistent bijection, replaces names simultaneously, and invokes actual `cmp`.
  Positive rename controls pass; semantic and unrelated-checksum mutations fail.
  Proof ran before substituting the M1 driver into generated `main.go`.
- ABAP fixture compile: **3 supported oracle fixtures**, OSG
  `f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6`, pass. The Expr oracle retains
  its expected located `union.admit` ABAP refusal.
- `ABAPITI_GRACECHECK=1 TestLexerFactsReport`: pass; no golden changes.

Go/Node/compiler processes ran in user systemd scopes at MemoryMax=16G
(24G for the required broad test), with the requested Go toolchain/cache/tmp
settings. This executor serialized its heavy steps and checked available memory above 30G
and free disk above 40G. The full 11,060×2 run is reserved for the later i10 train.
