# 0003. The HIR emitter targets ABAP 7.50

Status: accepted, 2026-10-07 (Alice). Supersedes the 7.02 target for the HIR
track only.

## Context

Generated ABAP has targeted 7.02 syntax plus `int8` since 2026-10-02. `int8`
itself needs 7.50, so the output never ran on a real 7.02 system. The HIR
track's goal is readable code. abaplint's downport rule can lower newer syntax
when needed; the reverse is not possible. A4H is 7.58.

## Decision

The HIR emitter writes ABAP 7.50: inline declarations, `NEW`, `VALUE #`,
`CONV`, `CAST`, string templates, `IS INSTANCE OF` (replacing the
`instanceof` helper). The general corpus CI gate runs abaplint at `v750`
and checks that the corpus really contains 7.50 constructs. The WebAssembly
path keeps 7.02 style plus `int8`.

Exception (materialized RegExp.exec, observed lastIndex/sticky state, and
advanced regex syntax): these JavaScript regex semantics require PCRE,
introduced in 7.55. Ordinary match/test/replace and existing dynamic
configuration patterns retain the legacy translation; the default pruned
abaplint build remains 7.50/OSGO-compatible. PCRE lives in a separate runtime
subclass, emitted only when a lowered body needs it. Legacy FIND REGEX is longest-match,
multiline/dotall and rejects lazy quantifiers; it must not silently stand in for
JS exec. These feature suites are linted at `v755` when PCRE is present. Generated
A4H output states its minimum release; the pinned A4H system is 7.58. See
[the supported regex language](../../tsfront/regexp_exec.md).

## Consequences

- Shorter, readable output; `instanceof` becomes one predicate.
- osgo may not support every 7.50 construct (`NEW x( )->m( )` already failed,
  open-steamgate inbox case 019). Each gap becomes an inbox case; the emitter
  gets a narrow fallback only when a gap blocks.
